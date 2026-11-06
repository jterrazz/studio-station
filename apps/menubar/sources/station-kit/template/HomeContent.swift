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
            if let needs = page.section(TodoBadge.section), stopped {
                engineStopped(needs)
            } else if let needs = page.section(TodoBadge.section), needs.items?.contains(where: { $0.row != nil }) == true {
                needsYou(needs)
            }
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

    // value: the engine does not answer: what is wrong in a sentence and
    // the one action that helps, Start, on its tile.
    var stopped: Bool { page.section(TodoBadge.section)?.rows.contains { $0.id == "engine" } == true && page.section("machines") == nil }
    static let start = "start"

    func engineStopped(_ s: Section) -> some View {
        let start = page.action(Self.start)
        let row = s.rows.first
        let title = s.caption?.first.map { Format.show($0) } ?? "The engine is stopped"
        let words = row?.hint?.command.map { "Run \($0) in a terminal to set it up." } ?? "Start it to see your machines and what needs you."
        return HStack(alignment: .center, spacing: Token.Tile.gap) {
            IconTile(Token.Symbol.warn, tint: palette.warn)
            VStack(alignment: .leading, spacing: 1) {
                Text(title).font(.system(size: Token.Size.text, weight: .semibold)).foregroundStyle(.primary)
                    .padding(.vertical, Token.Size.leading).fixedSize(horizontal: false, vertical: true)
                SubText(words)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            if let start {
                if state.running == start.id {
                    PendingDot()
                } else {
                    StationButton("Start", role: .primary) { perform(start, handlers) }.disabled(state.running != nil)
                }
            }
        }
        .fixedSize(horizontal: false, vertical: true)
        .stationTile(tint: palette.warn)
    }

    // value: what needs you as the vision's tile: its count in words, what
    // it is under them, a chevron that unfolds each row and its action.
    func needsYou(_ s: Section) -> some View {
        let rows = (s.items ?? []).compactMap(\.row)
        let title = s.caption?.first.map { Format.show($0) } ?? s.title ?? ""
        let worst = TodoBadge.todo(page)
        let tint = (worst?.n ?? 0) == 0 ? nil : worst?.mark == .broken && rows.allSatisfy { $0.mark == .broken } ? palette.bad : palette.warn
        return VStack(alignment: .leading, spacing: 6) {
            Button { unfolded.toggle() } label: {
                HStack(alignment: .center, spacing: Token.Tile.gap) {
                    IconTile(Token.Symbol.warn, tint: tint)
                    VStack(alignment: .leading, spacing: 1) {
                        Text(title).font(.system(size: Token.Size.text, weight: .semibold)).foregroundStyle(.primary).lineLimit(1)
                            .padding(.vertical, Token.Size.leading)
                        if !unfolded {
                            SubText(rows.compactMap(\.label).joined(separator: Token.Glyph.sep)).lineLimit(2)
                        }
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                    Chevron(down: unfolded)
                }
                .fixedSize(horizontal: false, vertical: true)
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityValue(unfolded ? "unfolded" : "folded")
            if unfolded {
                ForEach(Array(rows.enumerated()), id: \.offset) { _, row in
                    HStack(spacing: Token.Tile.gap) {
                        StateDot(mark: row.mark).frame(width: Token.Tile.icon)
                        Text(row.label ?? "").font(.system(size: Token.Size.text)).lineLimit(1).truncationMode(.middle)
                        Spacer(minLength: 4)
                        ForEach(Array(page.rowActions(row.id ?? "").enumerated()), id: \.offset) { _, a in
                            if state.running == a.id || state.settling == a.id {
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
                    let open = page.rowActions(accounts.rows.first?.id ?? "").first
                    place(symbol: Token.Symbol.accounts, active: false, title: accounts.title ?? "",
                          words: accounts.rows.first?.cells?.first.map { Format.show($0) }, tap: open.map { a in { perform(a, handlers) } }) { EmptyView() }
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
                if state.running == toggle.id || state.settling == toggle.id {
                    PendingDot()
                } else {
                    StationSwitch(on: on) { handlers.run(toggle, state.answers[toggle.id ?? ""] ?? [:]) }.disabled(state.running != nil)
                }
            }
        }
    }

    // invariant: a tile that opens something is one button across it, its
    // words included; what ends its head (a switch) stays its own control,
    // and its words take the tile's whole width under the head.
    func place<T: View>(symbol: String, active: Bool, title: String, words: String?, tap: (() -> Void)?, @ViewBuilder trailing: () -> T) -> some View {
        let button = { (label: AnyView) in
            Button { tap?() } label: { label.contentShape(Rectangle()) }.buttonStyle(.plain).allowsHitTesting(tap != nil)
        }
        return VStack(alignment: .leading, spacing: 4) {
            HStack(spacing: Token.Tile.gap) {
                button(AnyView(HStack(spacing: Token.Tile.gap) {
                    IconTile(symbol, active: active)
                    Text(title).font(.system(size: Token.Size.text, weight: .semibold)).foregroundStyle(.primary).lineLimit(1).fixedSize()
                    if tap != nil, T.self == EmptyView.self { Chevron() }
                    Spacer(minLength: 0)
                }))
                trailing().fixedSize()
            }
            .frame(minHeight: 26)
            if let words, !words.isEmpty {
                button(AnyView(SubText(words.replacingOccurrences(of: Token.Glyph.dash, with: Token.Glyph.unbroken)).lineLimit(3)
                    .frame(maxWidth: .infinity, alignment: .leading)))
            }
            Spacer(minLength: 0)
        }
        .frame(maxHeight: .infinity, alignment: .top)
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
