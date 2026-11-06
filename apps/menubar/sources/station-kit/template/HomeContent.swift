import SwiftUI

// invariant: SPEC §10.2's Home, vision panel a, in its order: the three
// measures' tiles, what needs you, the machines' list, the Awake and
// Accounts tiles side by side, then the menu.
struct HomeContent: View {
    let page: Page
    let state: PopoverState
    let handlers: PopoverHandlers
    @State private var unfolded = false
    @Environment(\.stationPalette) private var palette

    var body: some View {
        VStack(alignment: .leading, spacing: Token.Tile.section) {
            if let strip = page.strip {
                Button { handlers.push(Navigation.activity) } label: {
                    Metrics(measures: [("CPU", strip.cpu, strip.spark), ("Memory", strip.memory, strip.memory_spark), ("Network", strip.net?.first, strip.net_spark)])
                        .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
            }
            if let needs = page.section(TodoBadge.section), needs.items?.contains(where: { $0.row != nil }) == true { needsYou(needs) }
            if let machines = page.section("machines") {
                SectionHeader(machines.title ?? "")
                MachineTree(section: machines, kinds: false, only: nil, open: handlers.openMachine)
            }
            places
            SheetFor(page: page, state: state, handlers: handlers)
            if state.confirming == nil || page.action(state.confirming ?? "")?.on == "row" {
                menu
            }
        }
    }

    // value: what needs you as the vision's tile: its count in words, what
    // it is under them, a chevron that unfolds each row and its action.
    func needsYou(_ s: Section) -> some View {
        let rows = (s.items ?? []).compactMap(\.row)
        let title = s.caption?.first.map { Format.show($0) } ?? s.title ?? ""
        return VStack(alignment: .leading, spacing: 6) {
            MenuRow(title, subtitle: rows.compactMap(\.label).joined(separator: Token.Glyph.sep), lines: 3, bare: true, tap: { unfolded.toggle() }) {
                let worst = TodoBadge.todo(page)
                IconTile(Token.Symbol.warn, tint: (worst?.n ?? 0) == 0 ? nil : worst?.mark == .broken && rows.allSatisfy { $0.mark == .broken } ? palette.bad : palette.warn)
            } trailing: {
                Chevron(down: unfolded)
            }
            if unfolded {
                ForEach(Array(rows.enumerated()), id: \.offset) { _, row in
                    HStack(spacing: Token.Tile.gap) {
                        StateDot(mark: row.mark).frame(width: Token.Tile.icon)
                        Text(row.label ?? "").font(.system(size: Token.Size.text)).lineLimit(1)
                        Spacer(minLength: 4)
                        ForEach(Array(page.rowActions(row.id ?? "").enumerated()), id: \.offset) { _, a in
                            if state.running == a.id {
                                PendingDot()
                            } else {
                                StationButton(a.label ?? "") { perform(a, handlers) }.disabled(state.running != nil)
                            }
                        }
                    }
                    .frame(minHeight: 24)
                }
            }
        }
        .stationTile()
    }

    // value: the vision's two tiles under the machines, side by side and
    // as tall: Awake with its switch, Accounts with its chevron, each its
    // words under it.
    @ViewBuilder
    var places: some View {
        let awake = page.section("awake")
        let accounts = page.section("accounts")
        if awake != nil || accounts != nil {
            HStack(alignment: .top, spacing: Token.Tile.pair) {
                if let awake { awakePlace(awake) }
                if let accounts {
                    place(symbol: Token.Symbol.accounts, active: false, title: accounts.title ?? "",
                          words: accounts.rows.first?.cells?.first.map { Format.show($0) }, tap: {}) { Chevron() }
                }
            }
            .fixedSize(horizontal: false, vertical: true)
        }
    }

    func awakePlace(_ s: Section) -> some View {
        let toggle = (page.actions ?? []).first { $0.rows?.contains("awake") == true && $0.run != nil }
        let on = toggle?.id == "power-sleep"
        let words = (s.rows.first?.cells?.first).map { Format.show($0) }
        return place(symbol: Token.Symbol.awake, active: on, title: s.title ?? "", words: words, tap: { handlers.push(Navigation.awake) }) {
            if let toggle {
                if state.running == toggle.id {
                    PendingDot()
                } else {
                    StationSwitch(on: on) { handlers.run(toggle, state.answers[toggle.id ?? ""] ?? [:]) }.disabled(state.running != nil)
                }
            }
        }
    }

    func place<T: View>(symbol: String, active: Bool, title: String, words: String?, tap: @escaping () -> Void, @ViewBuilder trailing: () -> T) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack(spacing: Token.Tile.gap) {
                Button(action: tap) {
                    HStack(spacing: Token.Tile.gap) {
                        IconTile(symbol, active: active)
                        Text(title).font(.system(size: Token.Size.text, weight: .semibold)).foregroundStyle(.primary).lineLimit(1)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                trailing()
            }
            .frame(minHeight: 26)
            if let words, !words.isEmpty {
                SubText(words.replacingOccurrences(of: Token.Glyph.dash, with: Token.Glyph.unbroken)).lineLimit(2)
            }
        }
        .frame(maxHeight: .infinity, alignment: .center)
        .stationTile()
    }

    var menu: some View {
        VStack(alignment: .leading, spacing: 0) {
            SectionDivider().padding(.bottom, 2)
            ForEach(Array(page.pageActions.enumerated()), id: \.offset) { _, a in
                if state.running == a.id {
                    PendingDot()
                } else {
                    let line = MenuLine(action: a, symbol: Symbols.of(a), danger: a.id == "stop-all") { perform(a, handlers) }.disabled(state.running != nil)
                    switch a.key {
                    case "cmd+n": line.keyboardShortcut("n", modifiers: .command)
                    case "cmd+t": line.keyboardShortcut("t", modifiers: .command)
                    default: line
                    }
                }
            }
        }
    }
}
