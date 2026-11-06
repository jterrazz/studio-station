import SwiftUI

// invariant: SPEC §10.2 Activity: the machine's measures as graphs over
// the window picked, the picker of 6 min, 1 h and 24 h on top with the page's
// window picked, then the top processes. Picking hands the window's word to
// the app; nothing runs from here.
public struct ActivityPage: View {
    let state: PopoverState
    let handlers: PopoverHandlers
    @Environment(\.stationPalette) private var palette

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
            PageHeader(back: banner?.label ?? page.context?.machine ?? "", title: "Activity", subtitle: Format.show(banner?.cells?.first ?? Value()), action: handlers.back)
            Segments(options: Self.picker(page, handlers: handlers).map { ($0.words, $0.picked, $0.tap) })
            if let graphs = page.section("graphs") {
                let rows = graphs.rows
                if let lead = rows.first, (lead.cells?.dropFirst().first?.points.count ?? 0) > 1 {
                    wide(lead)
                    VStack(alignment: .leading, spacing: 12) {
                        ForEach(Array(rows.dropFirst().enumerated()), id: \.offset) { _, r in measure(r) }
                    }
                } else {
                    VStack(alignment: .leading, spacing: 12) {
                        ForEach(Array(rows.enumerated()), id: \.offset) { _, r in measure(r) }
                    }
                }
            }
            if let top = page.section("processes") {
                VStack(alignment: .leading, spacing: 2) {
                    SectionHeader(top.title ?? "")
                    ForEach(Array(top.rows.prefix(5).enumerated()), id: \.offset) { _, r in
                        process(top, r)
                    }
                }
            }
        }
    }

    // value: the first measure as a wide chart drawn on the panel: its
    // name and words, its window in bars with the latest accented, the
    // window's start and Now under it.
    func wide(_ r: Row) -> some View {
        let points = r.cells?.dropFirst().first?.points ?? []
        let start = Self.picked(page).map { "\($0) ago" } ?? ""
        return VStack(alignment: .leading, spacing: 6) {
            HStack(alignment: .firstTextBaseline) {
                SectionHeader(r.label ?? "")
                Spacer(minLength: Token.Space.gap)
                Text(r.cells?.first.map { Format.show($0) } ?? "").font(.subheadline.weight(.semibold)).monospacedDigit().contentTransition(.numericText())
            }
            Bars(Self.buckets(points, 24), height: 64, accented: 2, chart: true)
            HStack {
                Text(start)
                Spacer()
                Text("Now")
            }
            .font(.caption).foregroundStyle(.secondary)
        }
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

    func measure(_ r: Row) -> some View {
        let words = r.cells?.first.map { Format.show($0) } ?? ""
        let points = r.cells?.dropFirst().first?.points ?? []
        let ratio = r.cells?.dropFirst().first.flatMap { $0.kind == "ratio" ? $0 : nil }
        return VStack(alignment: .leading, spacing: 3) {
            HStack(alignment: .firstTextBaseline) {
                Text(r.label ?? "").font(.subheadline).foregroundStyle(.secondary)
                Spacer(minLength: Token.Space.gap)
                Text(words).font(.subheadline.weight(.medium)).foregroundStyle(.primary).lineLimit(1).monospacedDigit()
                    .contentTransition(.numericText())
            }
            if points.count > 1 {
                Bars.share(Self.buckets(points, 24), height: Token.Tile.measure)
            } else if let ratio, let of = ratio.of, of > 0 {
                GeometryReader { geo in
                    ZStack(alignment: .leading) {
                        Capsule().fill(palette.faint)
                        Capsule().fill(palette.accent).frame(width: geo.size.width * CGFloat(ratio.n ?? 0) / CGFloat(of))
                    }
                }
                .frame(height: Token.Space.meterHeight)
            }
        }
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
                    Text(Token.Glyph.missing).font(.system(size: Token.Space.small)).foregroundStyle(palette.subtle).frame(width: 52, alignment: .trailing)
                        .help(Self.missing(key))
                }
            }
        }
        .frame(minHeight: Token.Tile.row)
    }
}
