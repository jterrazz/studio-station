import SwiftUI

// invariant: SPEC §10.2's switcher, vision panel b: a search, the
// machines opened last with This Mac first and checked, every machine
// as Home lists them, each VM with its kind, then Show all machines and
// Add a Mac.
struct SwitcherContent: View {
    let page: Page
    let state: PopoverState
    let handlers: PopoverHandlers

    var tree: Section? { page.section("machines") }

    func matching(_ s: Section) -> Set<String>? {
        guard !state.search.isEmpty else { return nil }
        var out = Set<String>()
        func walk(_ items: [Item]) {
            for it in items {
                if let r = it.row ?? it.group?.head, (r.label ?? "").localizedCaseInsensitiveContains(state.search) { out.insert(r.id ?? "") }
                walk(it.group?.items ?? [])
            }
        }
        walk(s.items ?? [])
        return out
    }

    // value: This Mac's line of the tree, where the owner is.
    func here(_ s: Section) -> Row? {
        (s.items ?? []).compactMap { $0.group?.head }.first { $0.label == Page.ThisMacWord }
    }

    // value: a VM's line under Recent, as the vision's: its kind, the Mac
    // it runs on, then its health or state; a Mac keeps its own words.
    func recentWords(_ s: Section, _ r: Row) -> String? {
        guard let kind = r.cells?.first?.text, kind != "Mac",
              let mac = (s.items ?? []).compactMap(\.group).first(where: { g in
                  (g.items ?? []).contains { ($0.group?.items ?? []).contains { $0.row?.id == r.id } || $0.row?.id == r.id }
              })?.head?.label else { return nil }
        let state = (r.cells?.last?.items ?? []).dropFirst().first.map { Format.show($0) }
        return (["\(kind) on \(mac)"] + [state].compactMap { $0 }).joined(separator: Token.Glyph.sep)
    }

    func rows(_ s: Section, _ ids: [String]) -> [Row] {
        var out: [Row] = []
        func walk(_ items: [Item]) {
            for it in items {
                if let r = it.row ?? it.group?.head, ids.contains(r.id ?? "") { out.append(r) }
                walk(it.group?.items ?? [])
            }
        }
        walk(s.items ?? [])
        return ids.compactMap { id in out.first { $0.id == id } }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Token.Tile.section) {
            TypedField("Search machines", text: Binding(get: { state.search }, set: handlers.search), symbol: Token.Symbol.search)
            if let tree {
                if state.search.isEmpty {
                    VStack(alignment: .leading, spacing: 4) {
                        SectionHeader("Recent")
                        if let mac = here(tree) {
                            MachineRow(row: mac, symbol: Token.Symbol.mac, kinds: true, sub: "\(mac.id ?? "")\(Token.Glyph.sep)you are here", picked: true, open: { _ in handlers.back() })
                        }
                        ForEach(Array(rows(tree, state.recent.filter { $0 != here(tree)?.id }).enumerated()), id: \.offset) { _, r in
                            MachineRow(row: r, symbol: Token.Symbol.machine(r.cells?.first?.text), kinds: true, sub: recentWords(tree, r), open: handlers.openMachine)
                        }
                    }
                }
                VStack(alignment: .leading, spacing: 4) {
                    SectionHeader(tree.title ?? "")
                    MachineTree(section: tree, kinds: true, only: matching(tree), open: handlers.openMachine)
                }
            }
            SheetFor(page: page, state: state, handlers: handlers)
            VStack(alignment: .leading, spacing: 2) {
                SectionDivider().padding(.bottom, 4)
                Button(action: handlers.back) {
                    MenuText(symbol: Token.Symbol.mac, title: "Show all machines", key: Token.Key.shown("cmd+0"), danger: false)
                }
                .buttonStyle(.plain)
                ForEach(Array(page.pageActions.enumerated()), id: \.offset) { _, a in
                    MenuLine(action: a, symbol: Token.Symbol.plus, danger: false) { perform(a, handlers) }
                }
            }
        }
    }
}
