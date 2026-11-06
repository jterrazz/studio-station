import SwiftUI

// invariant: SPEC §10.2 Activity: the machine's measures as graphs over
// the window picked, the picker of 6 min, 1 h and 24 h on top with the page's
// window picked, then the top processes. Picking hands the window's word to
// the app; nothing runs from here.
public struct ActivityPage: View {
    let state: PopoverState
    let handlers: PopoverHandlers
    @Environment(\.stationPalette) private var palette
    @Environment(\.colorScheme) private var scheme

    // value: SPEC §6.4's picker: the window's word on the page, and the
    // word the call's window takes.
    public static let windows: [(words: String, flag: String)] = [("6 min", "6m"), ("1 h", "1h"), ("24 h", "24h")]

    public init(_ state: PopoverState, handlers: PopoverHandlers = PopoverHandlers()) {
        self.state = state
        self.handlers = handlers
    }

    var page: Page { state.page ?? Page() }

    public static func picked(_ page: Page) -> String? { page.section("graphs")?.caption?.first?.text }

    // value: the picker as it is drawn: each window's words, whether it is
    // the page's, and its tap, which asks the app for that window.
    public static func picker(_ page: Page, handlers: PopoverHandlers) -> [(words: String, picked: Bool, tap: () -> Void)] {
        windows.map { w in (w.words, picked(page) == w.words, { handlers.window(w.flag) }) }
    }

    public var body: some View {
        let banner = page.section("machine")?.rows.first
        VStack(alignment: .leading, spacing: Token.Tile.section) {
            PageHeader(back: state.back ?? Page.ThisMacWord, title: "Activity", subtitle: Format.show(banner?.cells?.first ?? Value()), action: handlers.back)
            Segments(options: Self.picker(page, handlers: handlers).map { ($0.words, $0.picked, $0.tap) })
            if let graphs = page.section("graphs") {
                VStack(alignment: .leading, spacing: 14) {
                    ForEach(Array(graphs.rows.enumerated()), id: \.offset) { i, r in measure(r, lead: i == 0) }
                }
                .stationTile()
                if !graphs.rows.contains(where: { if case .graph = Self.chart($0) { true } else { false } }) {
                    FootNote("The graphs fill in as \(banner?.label ?? Page.ThisMacWord) is sampled, every 2 s while this page is open.")
                }
            }
            if let top = page.section("processes") {
                VStack(alignment: .leading, spacing: 2) {
                    HStack(spacing: Token.Space.gap) {
                        SectionHeader(top.title ?? "")
                        Spacer(minLength: 0)
                        ForEach(["CPU", "Memory"], id: \.self) { key in
                            Text(key.uppercased()).font(.system(size: Token.Size.caption, weight: .semibold)).tracking(Token.Size.caption * Token.Size.tracking)
                                .foregroundStyle(.secondary).frame(width: 52, alignment: .trailing)
                        }
                    }
                    ForEach(Array(top.rows.prefix(5).enumerated()), id: \.offset) { _, r in
                        process(top, r)
                    }
                }
            }
        }
    }

    // value: how a measure is drawn: its series as a graph, a share of a
    // whole (the disk's free space) as a meter, or, before it is sampled,
    // its graph's room waiting.
    public enum Chart: Equatable {
        case graph([Int])
        case meter(Value)
        case waiting
    }

    public static func chart(_ r: Row) -> Chart {
        let drawn = r.cells?.dropFirst().first
        if let points = drawn?.points, points.count > 1 { return .graph(points) }
        if let drawn, drawn.kind == "ratio", (drawn.of ?? 0) > 0 { return .meter(drawn) }
        return .waiting
    }

    // value: a series cut into `n` buckets, each its mean, oldest first.
    static func buckets(_ points: [Int], _ n: Int) -> [Int] {
        guard points.count > n else { return points }
        return (0..<n).map { i in
            let lo = i * points.count / n, hi = max((i + 1) * points.count / n, lo + 1)
            let slice = points[lo..<hi]
            return slice.reduce(0, +) / slice.count
        }
    }

    // invariant: a measure as the vision draws one: its name and its
    // words now, then its graph over the window, a line over an area
    // (Swift Charts), the first one taller, with the window's start and
    // Now under it.
    func measure(_ r: Row, lead: Bool) -> some View {
        let words = r.cells?.first.map { Format.show($0) } ?? ""
        let height: CGFloat = lead ? 56 : 30
        return VStack(alignment: .leading, spacing: 4) {
            HStack(alignment: .firstTextBaseline) {
                Text(r.label ?? "").font(.subheadline.weight(lead ? .semibold : .regular)).foregroundStyle(lead ? .primary : .secondary)
                Spacer(minLength: Token.Space.gap)
                Text(words).font(.subheadline.weight(.medium)).foregroundStyle(.primary).lineLimit(1).monospacedDigit()
                    .contentTransition(.numericText())
            }
            switch Self.chart(r) {
            case .graph(let points):
                Graph(Self.buckets(points, 90), height: height, tint: tint(r.id))
            case .meter(let ratio):
                GeometryReader { geo in
                    ZStack(alignment: .leading) {
                        Capsule().fill(.fill.tertiary)
                        Capsule().fill(palette.accent).frame(width: geo.size.width * CGFloat(ratio.n ?? 0) / CGFloat(max(ratio.of ?? 1, 1)))
                    }
                }
                .frame(height: Token.Space.meterHeight)
            case .waiting:
                ZStack(alignment: .bottom) {
                    Color.clear
                    Rectangle().fill(.fill.tertiary).frame(height: 1)
                }
                .frame(height: height)
                .accessibilityLabel("Not sampled yet")
            }
            if lead {
                HStack {
                    Text(Self.picked(page).map { "\($0) ago" } ?? "")
                    Spacer()
                    Text("Now")
                }
                .font(.caption).foregroundStyle(.secondary)
            }
        }
    }

    // value: each measure in its own ink, as Home's tiles: CPU blue,
    // memory indigo, the rest teal.
    func tint(_ id: String?) -> Color {
        Token.Hue.measure(id == "cpu" ? "CPU" : id == "memory" || id == "swap" ? "Memory" : "", scheme)
    }

    // value: why a process has no figure in a column: the engine ranks
    // the busiest by CPU and by memory apart, and names a figure only
    // where its list holds the process.
    static func missing(_ key: String) -> String {
        "Not among the busiest by \(key == "cpu" ? "CPU" : key)"
    }

    func process(_ s: Section, _ r: Row) -> some View {
        HStack(spacing: Token.Space.gap) {
            Text(r.label ?? "").font(.body).foregroundStyle(.primary).lineLimit(1)
            if let what = s.cell(r, "what") { Text(Format.show(what)).font(.subheadline).foregroundStyle(.secondary) }
            Spacer(minLength: 0)
            ForEach(["cpu", "memory"], id: \.self) { key in
                if let v = s.cell(r, key) {
                    Text(Format.show(v)).font(.subheadline).foregroundStyle(.secondary).monospacedDigit().frame(width: 52, alignment: .trailing)
                } else {
                    Text(Token.Glyph.missing).font(.system(size: Token.Space.small)).foregroundStyle(.tertiary).frame(width: 52, alignment: .trailing)
                        .help(Self.missing(key))
                }
            }
        }
        .padding(.horizontal, 4)
        .frame(minHeight: 30)
    }
}
