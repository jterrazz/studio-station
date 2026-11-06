import SwiftUI

// invariant: SPEC §10.2's page of one machine, vision panels c and f: its
// name, kind and state, its sections in the vision's order and words,
// its buttons; each action names its call and asks at its call's level.
// A server's one primary action is Restart.
struct MachineContent: View {
    let page: Page
    let state: PopoverState
    let handlers: PopoverHandlers
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationNow) private var now
    @Environment(\.stationStill) private var still

    var head: Section? { page.section("machine") }
    var server: Bool { page.context?.kind == "server" }

    var body: some View {
        VStack(alignment: .leading, spacing: Token.Tile.section) {
            if let error = page.error { ErrorLine(error) }
            if let head { header(head) }
            ForEach(Array((page.sections ?? []).enumerated()), id: \.offset) { _, s in
                if s.id != "machine" { section(s) }
            }
            SheetFor(page: page, state: state, handlers: handlers)
            if state.confirming == nil || page.action(state.confirming ?? "")?.on == "row" {
                buttons
            }
        }
    }

    func header(_ s: Section) -> some View {
        let row = s.items?.first?.row
        let mark = row?.mark ?? .ok
        var caption = (s.caption ?? []).map { Format.show($0) }
        var badge: String?
        if caption.first == "Server VM" {
            badge = "Server"
            caption.removeFirst()
        }
        let back = server ? page.context?.on ?? Self.all : Self.all
        return PageHeader(back: back, title: s.title ?? "", subtitle: caption.joined(separator: Token.Glyph.sep), symbol: Token.Symbol.machine(page.context?.kind),
                          badge: badge, action: handlers.back) {
            if let word = row?.cells?.first.map({ Format.show($0) }) {
                StateWord(word, mark: mark)
            }
        }
    }

    // value: where a page goes back to: All machines, or for a server the
    // Mac it lives on (vision panels c and f).
    static let all = "All machines"

    func words(_ v: Value?) -> String { v.map { Format.show($0, now: now) } ?? "" }

    @ViewBuilder
    func section(_ s: Section) -> some View {
        switch s.id {
        case "activity":
            if server { SectionHeader("Resources") }
            Metrics(measures: s.rows.map { r in (r.label ?? "", r.cells?.first, r.cells?.dropFirst().first?.points) })
        case "up":
            HStack(alignment: .top, spacing: Token.Tile.pair) {
                ForEach(Array(s.rows.enumerated()), id: \.offset) { _, r in figure(r) }
            }
            .fixedSize(horizontal: false, vertical: true)
        case "health":
            SectionHeader(s.header)
            Rows {
                ForEach(Array(s.rows.enumerated()), id: \.offset) { _, r in health(r) }
            }
        case "inside":
            Rows {
                ForEach(Array(s.rows.enumerated()), id: \.offset) { _, r in
                    MenuRow(r.label ?? "", tap: {}) {
                        IconTile(Token.Symbol.line(r.id ?? "") ?? Token.Symbol.forward)
                    } trailing: {
                        TrailingValue(words(r.cells?.first))
                        Chevron()
                    }
                }
            }
        case "ports":
            SectionHeader("\(s.header)\(Token.Glyph.sep)\((s.items ?? []).compactMap(\.link).count)")
            Rows {
                ForEach(Array((s.items ?? []).compactMap(\.link).enumerated()), id: \.offset) { _, l in link(l) }
                if let share = page.action("share-port") {
                    MenuRow("\(share.label ?? "")\(Token.Glyph.ellipsis)", tap: { perform(share, handlers) }) {
                        IconTile(Token.Symbol.share)
                    }
                }
            }
        case "shared":
            SectionHeader(s.header)
            Rows {
                ForEach(Array(s.rows.enumerated()), id: \.offset) { _, r in
                    MenuRow(r.label ?? "", subtitle: words(r.cells?.first)) {
                        EmptyView()
                    } trailing: {
                        ForEach(Array(page.rowActions(r.id ?? "").enumerated()), id: \.offset) { _, a in
                            StationButton(a.label ?? "") { perform(a, handlers) }.disabled(state.running != nil)
                        }
                    }
                }
            }
        case "server":
            Rows {
                ForEach(Array(s.rows.enumerated()), id: \.offset) { _, r in serverRow(r) }
            }
            ForEach(Array(s.notes.enumerated()), id: \.offset) { _, n in FootNote(n) }
        case "vms":
            SectionHeader(s.header)
            MachineTree(section: s, kinds: true, only: nil, open: handlers.openMachine)
        default:
            SectionHeader(s.header)
            Rows {
                ForEach(Array((s.items ?? []).enumerated()), id: \.offset) { _, item in
                    if let row = item.row {
                        markRow(s, row)
                    } else if let note = item.note?.text {
                        FootNote(note).padding(Token.Tile.rowPadding)
                    } else {
                        ItemsView([item], columns: s.columns ?? [], width: Token.Space.width - Token.Space.margin * 2).padding(Token.Tile.rowPadding)
                    }
                }
            }
        }
    }

    // value: an Overview figure as the vision's two tiles: its name, its
    // figure in 13 pt bold, its words under it.
    func figure(_ r: Row) -> some View {
        let value = r.cells?.first
        let items = value?.kind == "list" ? value?.items ?? [] : value.map { [$0] } ?? []
        let shown: String
        if r.id == "restarts" {
            shown = "\(Token.Glyph.restart) \(items.first?.n ?? 0)"
        } else {
            shown = items.first.flatMap { Self.span($0) }.map { Format.show($0) }.flatMap { $0.isEmpty ? nil : $0 } ?? Token.Glyph.missing
        }
        let rest = items.dropFirst().map { Format.show($0) }.joined(separator: Token.Glyph.sep)
        return VStack(alignment: .leading, spacing: 4) {
            SubText(r.label ?? "")
            Text(shown).font(.system(size: Token.Size.text, weight: .bold)).monospacedDigit()
            if !rest.isEmpty { SubText(rest) }
        }
        .frame(maxHeight: .infinity, alignment: .topLeading)
        .stationTile()
    }

    // value: the Up figure without the word its label already says: `up
    // 12 d` is shown 12 d.
    static func span(_ v: Value?) -> Value? {
        guard var v, let text = v.text, text.hasPrefix("up ") else { return v }
        v.text = String(text.dropFirst(3))
        return v
    }

    // value: a health check as the vision's: a check in its state's
    // colour, its name, an address under it when it is one, what it
    // answered (or its value) on the right.
    func health(_ r: Row) -> some View {
        let value = r.cells?.first.flatMap { $0.isNone ? nil : $0 }
        let parts = value.map { $0.kind == "list" ? $0.items ?? [] : [$0] } ?? []
        let address = parts.first.flatMap { Format.show($0).hasPrefix("http") ? Format.show($0) : nil }
        let rest = parts.dropFirst(address == nil ? 0 : 1).map { Format.show($0) }.joined(separator: Token.Glyph.sep)
        return MenuRow(r.label ?? "", subtitle: address, link: true) {
            Image(systemName: r.mark == .ok ? Token.Symbol.health : Token.Symbol.of(r.mark ?? .notKnown))
                .font(.system(size: Token.Size.check, weight: .medium)).foregroundStyle(palette.of(r.mark ?? .notKnown)).frame(width: Token.Tile.chevron)
        } trailing: {
            if !rest.isEmpty { TrailingValue(rest) }
        }
    }

    func markRow(_ s: Section, _ r: Row) -> some View {
        let fix = s.columns?.first { $0.kind == "command" }?.key
        let command = fix.flatMap { s.cell(r, $0)?.text }
        let value = (r.cells ?? []).filter { !$0.isNone && $0.text != command }.map { Format.show($0, now: now) }.filter { !$0.isEmpty }.joined(separator: Token.Glyph.sep)
        let title = (r.short ?? r.label ?? "").capitalizedFirst
        let long = value.count > 14 || title.count + value.count > 38
        return MenuRow(title, subtitle: long ? value.capitalizedFirst : nil) {
            StateDot(mark: r.mark).frame(width: Token.Tile.icon)
        } trailing: {
            if !value.isEmpty, !long { TrailingValue(value) }
            ForEach(Array(page.rowActions(r.id ?? "").enumerated()), id: \.offset) { _, a in
                TextAction(a.label ?? "") { perform(a, handlers) }.disabled(state.running != nil)
            }
        }
    }

    func serverRow(_ r: Row) -> some View {
        let value = words(r.cells?.first)
        if r.id == "ssh" {
            return AnyView(MenuRow(r.label ?? "", subtitle: value, link: true) {
                IconTile(Token.Symbol.terminal)
            } trailing: {
                PlainIcon(Token.Symbol.copy) { handlers.copy(value) }
            })
        }
        return AnyView(MenuRow(r.label ?? "", tap: { handlers.push(Navigation.ports) }) {
            IconTile(Token.Symbol.ports)
        } trailing: {
            TrailingValue(Self.counted(r.cells?.first))
            Chevron()
        })
    }

    // value: a shared port as the vision's line: its name and port, its
    // address under it in the accent, Copy, QR and Open at its end; the QR
    // unfolds under it.
    func link(_ l: Link) -> some View {
        let url = l.value.flatMap(Format.url)
        let address = l.value.map { Format.show($0) } ?? ""
        let lead = l.lead.map { Format.show($0) } ?? l.label ?? ""
        return VStack(alignment: .center, spacing: 8) {
            MenuRow(lead, subtitle: address, link: true) {
                EmptyView()
            } trailing: {
                HStack(spacing: 10) {
                    PlainIcon(Token.Symbol.copy) { handlers.copy(address) }
                    if let url {
                        PlainIcon(Token.Symbol.qr, picked: state.qr == url.absoluteString) { handlers.toggleQR(url.absoluteString) }
                        PlainIcon(Token.Symbol.open) { handlers.open(url) }
                    }
                }
            }
            if let url, state.qr == url.absoluteString {
                QRView(url.absoluteString).padding(12).background(RoundedRectangle(cornerRadius: Token.Tile.qrCorner, style: .continuous).fill(palette.qrLight)).padding(.bottom, 12)
            }
        }
    }

    // invariant: the page's own actions as the vision's buttons, side by
    // side and as wide: Restart the one primary, Stop, and what destroys
    // in red; Open in Terminal answers its key.
    var buttons: some View {
        let shown = page.pageActions.filter { ["restart", "stop", "start", "delete", "unlock"].contains($0.id ?? "") }
        let terminal = page.action("open-station")
        return HStack(spacing: Token.Tile.pair) {
            ForEach(Array(shown.enumerated()), id: \.offset) { _, a in
                if state.running == a.id {
                    PendingDot().frame(maxWidth: .infinity)
                } else {
                    StationButton(Self.button(a), role: Self.role(a), wide: true) { perform(a, handlers) }
                        .disabled(state.running != nil)
                }
            }
        }
        .fixedSize(horizontal: false, vertical: true)
        .background {
            if let terminal, !still {
                Button("") { handlers.run(terminal, [:]) }.keyboardShortcut("t", modifiers: .command).opacity(0).allowsHitTesting(false)
            }
        }
    }

    // value: a count said as the vision says it, its number first: `Public
    // 1` is 1 public.
    static func counted(_ v: Value?) -> String {
        let parts = (v?.items ?? v.map { [$0] } ?? []).map { Format.show($0) }
        return parts.map { p in
            let words = p.split(separator: " ")
            guard words.count == 2, Int(words[1]) != nil else { return p }
            return "\(words[1]) \(words[0].lowercased())"
        }.joined(separator: Token.Glyph.sep)
    }

    // value: a page button's role: Restart the primary, Delete in red, a
    // button whose sheet asks for the name greyed and saying so (vision
    // panel f: Stop, then "(type the name)").
    static func role(_ a: Action) -> StationRole {
        switch a.id {
        case "restart": .primary
        case "delete": .danger
        default: a.confirm?.type != nil ? .quiet : .plain
        }
    }

    static func button(_ a: Action) -> String {
        role(a) == .quiet ? "\(label(a)) (type the name)" : label(a)
    }

    static func label(_ a: Action) -> String {
        (a.form != nil || a.confirm?.type != nil) ? "\(a.label ?? "")\(Token.Glyph.ellipsis)" : a.label ?? ""
    }
}
