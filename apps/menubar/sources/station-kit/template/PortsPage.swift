import SwiftUI

// invariant: SPEC §10.2 Ports and A port, vision panels d and g: what is
// reachable outside Tailscale undeclared first, in red with Review; then
// Public; then Private; each row says its class in words. A workspace's
// page (vision d) says instead whether its shares get an https:// link,
// then lists them under no header. A shared port unfolds in its row: its
// link, its QR, Copy link, Open, Stop sharing.
public struct PortsPage: View {
    let state: PopoverState
    let handlers: PopoverHandlers
    @Environment(\.stationPalette) private var palette

    public init(_ state: PopoverState, handlers: PopoverHandlers = PopoverHandlers()) {
        self.state = state
        self.handlers = handlers
    }

    var page: Page { state.page ?? Page() }
    var act: Act { Act(state: state, handlers: handlers) }

    public var body: some View {
        let summary = page.summary()
        VStack(alignment: .leading, spacing: Token.Tile.section) {
            PageHeader(back: summary?.label ?? page.context?.machine ?? "", title: "Ports", subtitle: Format.show(summary?.cells?.first ?? Value()), action: handlers.back)
            if let red = page.section("needs-you") { needsYou(red) }
            if let https = page.section("https"), let row = https.rows.first {
                MenuRow(row.label ?? "", subtitle: Format.show(row.cells?.first ?? Value())) {
                    EmptyView()
                } trailing: {
                    StationSwitch(on: row.mark == .ok).disabled(true)
                }
            }
            if let public_ = page.section("public") {
                VStack(alignment: .leading, spacing: 2) {
                    SectionHeader(public_.header)
                    ForEach(Array(public_.rows.enumerated()), id: \.offset) { _, r in plainRow(public_, r, symbol: Token.Symbol.globe) }
                }
            }
            if let private_ = page.section("private") {
                VStack(alignment: .leading, spacing: 2) {
                    if page.section("https") == nil { SectionHeader(private_.header) }
                    ForEach(Array(private_.rows.enumerated()), id: \.offset) { _, r in privateRow(private_, r) }
                }
            }
            if let ranges = page.section("ranges") {
                ForEach(Array(ranges.notes.enumerated()), id: \.offset) { _, n in FootNote(n) }
            }
            footer
        }
    }

    // value: what is reachable outside Tailscale undeclared, as the
    // vision's red finding: its count in words, the ports and what they
    // are under them, Review; the page's one grouping.
    func needsYou(_ s: Section) -> some View {
        let rows = s.rows
        let ports = rows.map { r in "\(r.short ?? r.label ?? "")\(Token.Glyph.sep)\(s.cell(r, "port").map { Format.show($0) } ?? "")" }
        let whats = rows.compactMap { s.cell($0, "what").map { Format.show($0) } }
        let review = rows.first.flatMap { page.actions(for: $0.id ?? "").first { $0.run?.terminal == true } }
        let reason = s.title.flatMap { t in t.range(of: ": ").map { String(t[$0.upperBound...]) } } ?? ""
        let title = "\(rows.count) \(rows.count == 1 ? "port" : "ports") \(reason.components(separatedBy: ",").first ?? "")"
        return HStack(alignment: .center, spacing: Token.Space.gap) {
            IconTile(Token.Symbol.warn, active: true, tint: palette.bad)
            VStack(alignment: .leading, spacing: 2) {
                Text(title).font(.body.weight(.semibold)).fixedSize(horizontal: false, vertical: true)
                Text(([ports.joined(separator: ", ")] + whats).joined(separator: Token.Glyph.sep)).font(.subheadline).foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
            .layoutPriority(1)
            Spacer(minLength: 4)
            if let review { StationButton(review.label ?? "") { act.tap(review) } }
        }
        .stationGroup()
    }

    func line(title: String, detail: String, mark: Mark?, symbol: String?, trailing: Bool = false, open: Bool = false, tap: (() -> Void)? = nil) -> some View {
        MenuRow(title, subtitle: detail, tap: tap) {
            if let symbol { IconTile(symbol) }
        } trailing: {
            if mark == .broken { StateDot(mark: mark) }
            if trailing { Chevron(down: open) }
        }
    }

    func plainRow(_ s: Section, _ r: Row, symbol: String) -> some View {
        let port = s.cell(r, "port").map { Format.show($0) } ?? ""
        let words = [s.cell(r, "what"), s.cell(r, "declared")].compactMap { $0.map { Format.show($0) } }
        return line(title: "\(r.short ?? r.label ?? "")\(Token.Glyph.sep)\(port)", detail: words.joined(separator: Token.Glyph.sep), mark: nil, symbol: symbol, trailing: true)
    }

    func shared(_ s: Section, _ r: Row) -> URL? { s.cell(r, "url").flatMap(Format.url) }

    // invariant: a private row names its class in words: a shared port its
    // reserved port and state, under no tile as linux-1's links (vision c,
    // d), any other how Tailscale carries it.
    func privateRow(_ s: Section, _ r: Row) -> some View {
        let port = s.cell(r, "port")
        let url = shared(s, r)
        let reserved = port?.kind == "portmap" ? "reserved port \(port?.of ?? 0)" : nil
        let state = s.cell(r, "state").map { Format.show($0) }
        let profile = Format.show(r.detail ?? Value())
        let what = !profile.isEmpty ? profile.capitalized : reserved != nil ? "TCP" : s.cell(r, "process").map { Format.show($0) } ?? r.label ?? ""
        let title = url != nil || reserved != nil ? "\(what)\(Token.Glyph.sep)\(port?.n ?? 0)" : "\(r.short ?? r.label ?? "")\(Token.Glyph.sep)\(port?.n ?? 0)"
        let detail = (["Private", reserved ?? state] + (url == nil && reserved == nil ? [s.cell(r, "process").map { Format.show($0) }] : [])).compactMap { $0 }.joined(separator: Token.Glyph.sep)
        let open = url.map { state_qr($0) } ?? false
        return VStack(alignment: .leading, spacing: Token.Space.gap) {
            line(title: title, detail: detail, mark: r.mark, symbol: url == nil && reserved == nil ? Token.Symbol.tailnet : nil,
                 trailing: true, open: open, tap: url.map { u in { handlers.toggleQR(u.absoluteString) } })
            if let url, open { unfolded(r, url) }
        }
    }

    func state_qr(_ url: URL) -> Bool { state.qr == url.absoluteString }

    // value: what a shared port's unfolded row does with its link: Copy
    // link puts it on the pasteboard, Open opens it.
    public static func link(_ url: URL, handlers: PopoverHandlers) -> [(label: String, tap: () -> Void)] {
        [("Copy link", { handlers.copy(url.absoluteString) }), ("Open", { handlers.open(url) })]
    }

    // value: the note over Declare public, the routes in the words its form
    // offers them.
    public static func routes(_ declare: Action) -> String {
        let labels = (declare.form?.fields?.first { $0.key == "Via" }?.options ?? []).compactMap(\.label)
        let words = labels.count > 1 ? labels.dropLast().joined(separator: ", ") + " or " + labels[labels.count - 1] : labels.joined()
        return "To open a port on purpose, declare it public by \(words)."
    }

    // value: a shared port unfolded in its row, as the vision's: its QR,
    // its address, where it works, Copy link the page's primary action and
    // Open beside it, Stop sharing in red under them.
    func unfolded(_ r: Row, _ url: URL) -> some View {
        let stop = page.actions(for: r.id ?? "").first { $0.id?.hasPrefix("stop-sharing:") == true }
        let links = Self.link(url, handlers: handlers)
        return VStack(alignment: .center, spacing: 10) {
            QRView(url.absoluteString)
                .frame(width: Token.Space.qr, height: Token.Space.qr)
                .padding(8)
                .background(RoundedRectangle(cornerRadius: 10, style: .continuous).fill(palette.qrLight))
            LinkText(url.absoluteString, center: true)
            Text("Scan with your phone. Works on your Tailscale devices.").font(.subheadline).foregroundStyle(.secondary).multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)
            HStack(spacing: Token.Space.gap) {
                ForEach(Array(links.enumerated()), id: \.offset) { i, l in
                    StationButton(l.label, role: i == 0 ? .primary : .plain, wide: true, action: l.tap)
                }
            }
            if let stop { TextAction(stop.label ?? "", danger: true) { act.tap(stop) } }
            if let stop { act.sheet(stop) }
        }
        .frame(maxWidth: .infinity)
        .padding(.horizontal, Token.Space.gap)
        .padding(.bottom, 6)
    }

    var footer: some View {
        let declare = page.action("declare-public")
        let share = page.action("share-port")
        return VStack(alignment: .leading, spacing: Token.Space.gap) {
            if let declare { FootNote(Self.routes(declare)) }
            if let declare, act.open(declare) { act.sheet(declare) }
            if let share, act.open(share) { act.sheet(share) }
            VStack(alignment: .leading, spacing: 0) {
                if let declare, !act.open(declare) {
                    Button { act.tap(declare) } label: { MenuText(symbol: Token.Symbol.globe, title: act.label(declare), key: nil, danger: false) }.buttonStyle(.plain)
                }
                if let share, !act.open(share) {
                    Button { act.tap(share) } label: { MenuText(symbol: Token.Symbol.plus, title: act.label(share), key: nil, danger: false) }.buttonStyle(.plain)
                }
            }
        }
    }
}
