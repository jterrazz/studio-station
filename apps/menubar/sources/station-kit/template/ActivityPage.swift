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
        VStack(alignment: .leading, spacing: Token.Space.gap + 2) {
            PushedHeader(back: banner?.label ?? page.context?.machine ?? "", title: "Activity", subtitle: Format.show(banner?.cells?.first ?? Value()), action: handlers.back)
            HStack(spacing: 4) {
                ForEach(Array(Self.picker(page, handlers: handlers).enumerated()), id: \.offset) { _, w in
                    Pill(w.words, picked: w.picked, action: w.tap)
                }
                Spacer(minLength: 0)
            }
            if let graphs = page.section("graphs") {
                GlassCard {
                    ForEach(Array(graphs.rows.enumerated()), id: \.offset) { _, r in measure(r) }
                }
            }
            if let top = page.section("processes") {
                SectionHead(top.title ?? "")
                GlassCard {
                    ForEach(Array(top.rows.prefix(5).enumerated()), id: \.offset) { _, r in process(top, r) }
                }
            }
        }
    }

    func measure(_ r: Row) -> some View {
        let words = r.cells?.first.map { Format.show($0) } ?? ""
        let points = r.cells?.dropFirst().first?.points ?? []
        let ratio = r.cells?.dropFirst().first.flatMap { $0.kind == "ratio" ? $0 : nil }
        return VStack(alignment: .leading, spacing: 3) {
            HStack(alignment: .firstTextBaseline) {
                Text(r.label ?? "").font(.system(size: Token.Space.text, weight: .medium)).foregroundStyle(palette.text)
                Spacer(minLength: Token.Space.gap)
                Text(words).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).lineLimit(1).monospacedDigit()
            }
            if points.count > 1 {
                Graph(points)
            } else if let ratio, let of = ratio.of, of > 0 {
                GeometryReader { geo in
                    ZStack(alignment: .leading) {
                        Capsule().fill(palette.text.opacity(0.1))
                        Capsule().fill(palette.accent).frame(width: geo.size.width * CGFloat(ratio.n ?? 0) / CGFloat(of))
                    }
                }
                .frame(height: Token.Space.meterHeight)
            }
        }
    }

    func process(_ s: Section, _ r: Row) -> some View {
        HStack(spacing: Token.Space.gap) {
            Text(r.label ?? "").font(.system(size: Token.Space.text)).foregroundStyle(palette.text).lineLimit(1)
            if let what = s.cell(r, "what") { Text(Format.show(what)).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted) }
            Spacer(minLength: 0)
            ForEach(["cpu", "memory"], id: \.self) { key in
                Text(s.cell(r, key).map { Format.show($0) } ?? Token.Glyph.dash).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).monospacedDigit().frame(width: 52, alignment: .trailing)
            }
        }
    }
}
