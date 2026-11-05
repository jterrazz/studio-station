import SwiftUI

// invariant: SPEC §10.2 Ports and A port, vision panels d and g: what is
// reachable outside Tailscale undeclared first, in red with Review; then
// Public; then Private; each row says its class in words. A shared port
// unfolds in its row: its link, its QR, Copy link, Open, Stop sharing.
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
        VStack(alignment: .leading, spacing: Token.Space.gap + 2) {
            PushedHeader(back: summary?.label ?? page.context?.machine ?? "", title: "Ports", subtitle: Format.show(summary?.cells?.first ?? Value()),
                         symbol: Token.Symbol.port, tint: palette.accent, id: Navigation.ports, action: handlers.back)
            if let red = page.section("needs-you") { needsYou(red) }
            if let public_ = page.section("public") {
                GroupCard(public_.title ?? "", count: public_.rows.count, spacing: 6) { ForEach(Array(public_.rows.enumerated()), id: \.offset) { _, r in plainRow(public_, r, symbol: Token.Symbol.globe) } }
            }
            if let private_ = page.section("private") {
                GroupCard(private_.title ?? "", count: private_.rows.count, spacing: 8) { ForEach(Array(private_.rows.enumerated()), id: \.offset) { i, r in privateRow(private_, r).overlay(alignment: .top) { if i > 0 { Rule().offset(y: -4) } } } }
            }
            if let ranges = page.section("ranges") {
                ForEach(Array(ranges.notes.enumerated()), id: \.offset) { _, n in note(n) }
            }
            footer
        }
    }

    func note(_ text: String) -> some View {
        Text(text).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).padding(.horizontal, 4).fixedSize(horizontal: false, vertical: true)
    }

    func needsYou(_ s: Section) -> some View {
        let rows = s.rows
        let ports = rows.map { s.cell($0, "port").map { Format.show($0) } ?? "" }
        return GlassCard(tint: palette.bad) {
            HStack(alignment: .top, spacing: Token.Space.gap + 2) {
                IconCircle(Token.Symbol.warn, tint: palette.bad)
                VStack(alignment: .leading, spacing: 2) {
                    Text("\(rows.count) \(rows.count == 1 ? "port" : "ports") reachable outside Tailscale").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
                    Text("Not declared public\(Token.Glyph.sep)\(ports.joined(separator: ", "))").font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
                }
                Spacer(minLength: 0)
                if let review = rows.first.flatMap({ page.actions(for: $0.id ?? "").first { $0.run?.terminal == true } }) {
                    Pill(review.label ?? "") { act.tap(review) }
                }
            }
            ForEach(Array(rows.enumerated()), id: \.offset) { _, r in
                line(title: "\(r.short ?? r.label ?? "")\(Token.Glyph.sep)\(ports[rows.firstIndex(of: r) ?? 0])",
                     detail: [s.cell(r, "class"), s.cell(r, "what")].compactMap { $0.map { Format.show($0) } }.joined(separator: Token.Glyph.sep), mark: .broken)
            }
        }
    }

    func line(title: String, detail: String, mark: Mark?, symbol: String? = nil, trailing: String? = nil) -> some View {
        let tint: Color? = {
            switch symbol {
            case Token.Symbol.globe: return palette.warn
            case Token.Symbol.tailnet: return palette.teal
            default:
                switch mark {
                case .broken: return palette.bad
                case .attention: return palette.warn
                case .ok: return palette.accent
                default: return nil
                }
            }
        }()
        return HStack(alignment: .center, spacing: Token.Space.gap + 2) {
            IconTile(symbol ?? Token.Symbol.port, tint: tint)
            VStack(alignment: .leading, spacing: 1) {
                Text(title).font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text).lineLimit(1)
                Text(detail).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).lineLimit(2)
            }
            Spacer(minLength: 0)
            if let trailing {
                Image(systemName: trailing).font(.system(size: Token.Space.small, weight: .semibold)).foregroundStyle(palette.subtle)
            }
        }
        .frame(minHeight: Token.Tile.rowHeight - 6)
        .contentShape(Rectangle())
    }

    func plainRow(_ s: Section, _ r: Row, symbol: String) -> some View {
        let port = s.cell(r, "port").map { Format.show($0) } ?? ""
        let words = [s.cell(r, "what"), s.cell(r, "declared")].compactMap { $0.map { Format.show($0) } }
        return line(title: "\(r.short ?? r.label ?? "")\(Token.Glyph.sep)\(port)", detail: words.joined(separator: Token.Glyph.sep), mark: nil, symbol: symbol)
    }

    func shared(_ s: Section, _ r: Row) -> URL? { s.cell(r, "url").flatMap(Format.url) }

    // invariant: a private row names its class in words: a shared port its
    // reserved port and state, any other how Tailscale carries it.
    func privateRow(_ s: Section, _ r: Row) -> some View {
        let port = s.cell(r, "port")
        let url = shared(s, r)
        let reserved = port?.kind == "portmap" ? "reserved port \(port?.of ?? 0)" : nil
        let state = s.cell(r, "state").map { Format.show($0) }
        let what = Format.show(r.detail ?? Value()).isEmpty ? (s.cell(r, "process").map { Format.show($0) } ?? r.label ?? "") : Format.show(r.detail ?? Value()).capitalized
        let title = url != nil || reserved != nil ? "\(what)\(Token.Glyph.sep)\(port?.n ?? 0)" : "\(r.short ?? r.label ?? "")\(Token.Glyph.sep)\(port?.n ?? 0)"
        let detail = (["Private", reserved ?? state] + (url == nil && reserved == nil ? [s.cell(r, "process").map { Format.show($0) }] : [])).compactMap { $0 }.joined(separator: Token.Glyph.sep)
        let open = url.map { state_qr($0) } ?? false
        return VStack(alignment: .leading, spacing: Token.Space.gap) {
            Button { if let url { handlers.toggleQR(url.absoluteString) } } label: {
                line(title: title, detail: detail, mark: r.mark, symbol: url == nil && reserved == nil ? Token.Symbol.tailnet : nil,
                     trailing: url == nil ? nil : (open ? Token.Symbol.down : Token.Symbol.forward))
            }
            .buttonStyle(.plain)
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

    func unfolded(_ r: Row, _ url: URL) -> some View {
        let stop = page.actions(for: r.id ?? "").first { $0.id?.hasPrefix("stop-sharing:") == true }
        return VStack(alignment: .center, spacing: Token.Space.gap) {
            QRView(url.absoluteString)
                .frame(width: Token.Space.qr, height: Token.Space.qr)
                .padding(12)
                .background(RoundedRectangle(cornerRadius: 14, style: .continuous).fill(palette.qrLight))
            Text(url.absoluteString).font(.system(size: Token.Space.small, design: .monospaced)).foregroundStyle(palette.text).lineLimit(2).multilineTextAlignment(.center)
                .frame(maxWidth: .infinity).padding(.horizontal, 9).padding(.vertical, 7)
                .background(RoundedRectangle(cornerRadius: Token.Tile.hover, style: .continuous).fill(palette.well))
            Text("Scan with your phone. Works on your Tailscale devices.").font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)
            HStack(spacing: Token.Glass.spacing) {
                ForEach(Array(Self.link(url, handlers: handlers).enumerated()), id: \.offset) { i, l in
                    Pill(l.label, role: i == 0 ? .primary : .plain, wide: true, action: l.tap)
                }
            }
            if let stop { Pill(stop.label ?? "", role: .danger, wide: true) { act.tap(stop) } }
            if let stop { act.sheet(stop) }
        }
        .frame(maxWidth: .infinity)
        .padding(.bottom, 4)
    }

    var footer: some View {
        let declare = page.action("declare-public")
        let share = page.action("share-port")
        return VStack(alignment: .leading, spacing: Token.Space.gap) {
            if let declare {
                note(Self.routes(declare))
                if act.open(declare) { act.sheet(declare) } else { HStack { Pill(act.label(declare)) { act.tap(declare) }; Spacer() } }
            }
            if let share {
                if act.open(share) { act.sheet(share) } else { HStack { Pill(act.label(share), role: .primary) { act.tap(share) }; Spacer() } }
            }
        }
    }
}
