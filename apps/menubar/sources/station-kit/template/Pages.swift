import SwiftUI

// value: a rounded tile a step lighter than the panel (vision a-f).
struct TileBox<Content: View>: View {
    let content: Content
    @Environment(\.stationPalette) private var palette

    init(@ViewBuilder _ content: () -> Content) { self.content = content() }

    var body: some View {
        content
            .padding(Token.Tile.padding)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(RoundedRectangle(cornerRadius: Token.Tile.radius).fill(palette.text.opacity(Token.Tile.fill)))
    }
}

// value: one of the three measures: its name, its value, its sparkline.
struct MetricTile: View {
    let label: String
    let value: Value?
    let spark: [Int]?
    @Environment(\.stationPalette) private var palette

    var body: some View {
        TileBox {
            VStack(alignment: .leading, spacing: 2) {
                Text(label).font(.system(size: Token.Space.small - 1)).foregroundStyle(palette.muted)
                ValueText(value, size: Token.Space.text).lineLimit(1).minimumScaleFactor(0.7)
                if let spark, spark.count > 1 {
                    Spark(Self.scaled(spark), width: 80, height: Token.Tile.sparkHeight)
                }
            }
        }
    }

    // value: a series drawn against its own peak, so a rate in KB/s draws
    // like a share in percent.
    static func scaled(_ points: [Int]) -> [Int] {
        let top = max(points.max() ?? 1, 1)
        return top <= 100 ? points : points.map { $0 * 100 / top }
    }
}

// value: a machine's line: its mark, its name, a Server capsule, the words
// under it, and the way into its page.
struct MachineLine: View {
    let row: Row
    let symbol: String
    let capsule: Bool
    let open: (String) -> Void
    @Environment(\.stationPalette) private var palette

    var kind: String { row.cells?.first?.text ?? "" }
    var sub: String {
        let rest = (row.cells ?? []).dropFirst().filter { !$0.isNone }.map { Format.show($0) }
        return ([kind == "Mac" ? kind : nil] + rest.map { Optional($0) }).compactMap { $0 }.joined(separator: Token.Glyph.sep)
    }

    var body: some View {
        Button { open(row.id ?? "") } label: {
            HStack(spacing: Token.Space.gap) {
                Image(systemName: symbol).font(.system(size: Token.Space.text)).foregroundStyle(row.mark == .absent ? palette.muted : palette.accent).frame(width: 18)
                VStack(alignment: .leading, spacing: 1) {
                    HStack(spacing: 5) {
                        Text(row.label ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
                        if capsule && kind != "Mac" {
                            Text(kind).font(.system(size: Token.Space.small - 2, weight: .semibold)).foregroundStyle(palette.accent)
                                .padding(.horizontal, 5).padding(.vertical, 1)
                                .background(Capsule().fill(palette.accent.opacity(0.18)))
                        }
                    }
                    if !sub.isEmpty {
                        Text(sub).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).lineLimit(1)
                    }
                }
                Spacer(minLength: 0)
                MarkView(row.mark)
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

// invariant: SPEC §10.2's Machines: each Mac, then its Servers, then its
// Workspaces, each group named; a line opens its machine's page.
struct MachinesTree: View {
    let section: Section
    let capsules: Bool
    let only: Set<String>?
    let open: (String) -> Void
    @Environment(\.stationPalette) private var palette

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            ForEach(Array((section.items ?? []).enumerated()), id: \.offset) { _, item in
                if let g = item.group, g.head?.cells?.isEmpty ?? true {
                    groups([item])
                } else if let mac = item.group {
                    if let head = mac.head, only == nil || only!.contains(head.id ?? "") {
                        MachineLine(row: head, symbol: Token.Symbol.mac, capsule: false, open: open)
                    }
                    groups(mac.items ?? [])
                } else if let row = item.row {
                    MachineLine(row: row, symbol: Token.Symbol.vm, capsule: capsules, open: open)
                } else if let empty = item.empty {
                    EmptyLine(empty)
                }
            }
        }
    }

    func groups(_ items: [Item]) -> some View {
        ForEach(Array(items.enumerated()), id: \.offset) { _, item in
            if let g = item.group {
                let rows = (g.items ?? []).compactMap(\.row).filter { only == nil || only!.contains($0.id ?? "") }
                if !rows.isEmpty {
                    VStack(alignment: .leading, spacing: 4) {
                        if only == nil {
                            Text(g.head?.label ?? "").font(.system(size: Token.Space.small - 1, weight: .semibold)).foregroundStyle(palette.subtle).padding(.leading, 26)
                        }
                        ForEach(Array(rows.enumerated()), id: \.offset) { _, row in
                            MachineLine(row: row, symbol: Token.Symbol.vm, capsule: capsules || row.cells?.first?.text == "Server", open: open).padding(.leading, only == nil ? 14 : 0)
                        }
                    }
                }
            } else if let row = item.row, only == nil || only!.contains(row.id ?? "") {
                MachineLine(row: row, symbol: Token.Symbol.vm, capsule: true, open: open).padding(.leading, 14)
            }
        }
    }
}

// value: a menu line of the popover: its symbol, its words, its key.
struct MenuLine: View {
    let action: Action
    let symbol: String
    let danger: Bool
    let tap: () -> Void
    @Environment(\.stationPalette) private var palette

    var title: String { (action.form != nil || action.confirm?.type != nil) ? "\(action.label ?? "")\(Token.Glyph.ellipsis)" : action.label ?? "" }

    var body: some View {
        Button(action: tap) {
            HStack(spacing: Token.Space.gap) {
                Image(systemName: symbol).font(.system(size: Token.Space.small)).frame(width: 18)
                Text(title).font(.system(size: Token.Space.text))
                Spacer()
                if let key = action.key {
                    Text(Token.Key.shown(key)).font(.system(size: Token.Space.small)).foregroundStyle(palette.subtle)
                }
            }
            .foregroundStyle(danger ? palette.bad : palette.text)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

// invariant: a gesture opens its sheet (a form, or a question at its
// call's level) when it has one, and runs at once when it has none.
func perform(_ action: Action, _ handlers: PopoverHandlers) {
    if action.confirm != nil || action.form != nil {
        handlers.confirm(action)
    } else {
        handlers.run(action, [:])
    }
}

// value: the sheet of the action being confirmed, if any, on this page.
struct SheetFor: View {
    let page: Page
    let state: PopoverState
    let handlers: PopoverHandlers

    var body: some View {
        if let id = state.confirming, let action = page.action(id) {
            let answers = Binding(get: { state.answers[id] ?? [:] }, set: { new in for (k, v) in new { handlers.answer(id, k, v) } })
            if let form = action.form {
                FormSheet(form, run: action.label ?? "", answers: answers) { yes in
                    handlers.confirm(nil)
                    if yes { handlers.run(action, state.answers[id] ?? [:]) }
                }
            } else if let confirm = action.confirm {
                let typed = Binding(get: { state.answers[id]?[ActionRunner.typedKey] ?? "" }, set: { handlers.answer(id, ActionRunner.typedKey, $0) })
                ConfirmSheet(confirm, typed: typed) { yes in
                    handlers.confirm(nil)
                    if yes { handlers.run(action, state.answers[id] ?? [:]) }
                }
            }
        }
    }
}

// invariant: SPEC §10.2's Home, vision panel a: the three measures, what
// needs you, the machines, the Awake and Accounts tiles, then the menu.
struct HomeContent: View {
    let page: Page
    let state: PopoverState
    let handlers: PopoverHandlers
    let width: CGFloat
    @Environment(\.stationPalette) private var palette

    var body: some View {
        VStack(alignment: .leading, spacing: Token.Space.gap + 2) {
            if let strip = page.strip {
                HStack(spacing: 6) {
                    MetricTile(label: "CPU", value: strip.cpu, spark: strip.spark)
                    MetricTile(label: "Memory", value: strip.memory, spark: strip.memory_spark)
                    MetricTile(label: "Network", value: strip.net?.first, spark: strip.net_spark)
                }
            }
            if let needs = page.section(TodoBadge.section) { needsTile(needs) }
            if let machines = page.section("machines") {
                VStack(alignment: .leading, spacing: 6) {
                    Text(machines.title ?? "").font(.system(size: Token.Space.small, weight: .semibold)).foregroundStyle(palette.muted)
                    MachinesTree(section: machines, capsules: false, only: nil, open: handlers.openMachine)
                }
            }
            HStack(alignment: .top, spacing: 6) {
                if let awake = page.section("awake") { awakeTile(awake) }
                if let accounts = page.section("accounts") { accountsTile(accounts) }
            }
            SheetFor(page: page, state: state, handlers: handlers)
            if state.confirming == nil || page.action(state.confirming ?? "")?.on == "row" {
                menu
            }
        }
    }

    func needsTile(_ s: Section) -> some View {
        TileBox {
            VStack(alignment: .leading, spacing: 4) {
                HStack(spacing: 6) {
                    Image(systemName: Token.Symbol.warn).font(.system(size: Token.Space.small)).foregroundStyle(palette.warn)
                    Text(s.caption?.first?.text ?? s.title ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
                }
                ForEach(Array((s.items ?? []).enumerated()), id: \.offset) { _, item in
                    if let row = item.row {
                        HStack(spacing: 6) {
                            MarkView(row.mark)
                            Text(row.label ?? "").font(.system(size: Token.Space.small)).foregroundStyle(palette.text).lineLimit(1)
                            Spacer(minLength: 4)
                            ForEach(Array(page.rowActions(row.id ?? "").enumerated()), id: \.offset) { _, a in
                                if state.running == a.id {
                                    PendingDot()
                                } else {
                                    SheetButton(a.label ?? "", role: .accent) { perform(a, handlers) }.disabled(state.running != nil)
                                }
                            }
                        }
                    } else if let empty = item.empty {
                        EmptyLine(empty)
                    } else if let link = item.link {
                        Text(link.label ?? "").font(.system(size: Token.Space.small)).foregroundStyle(palette.accent)
                    }
                }
            }
        }
    }

    func awakeTile(_ s: Section) -> some View {
        let toggle = (page.actions ?? []).first { $0.rows?.contains("awake") == true && $0.run != nil }
        let on = toggle?.id == "power-sleep"
        let words = (s.items?.first?.row?.cells?.first).map { Format.show($0) } ?? ""
        return TileBox {
            VStack(alignment: .leading, spacing: 4) {
                HStack(spacing: 6) {
                    Image(systemName: Token.Symbol.sun).font(.system(size: Token.Space.small)).foregroundStyle(palette.warn)
                    Text(s.title ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
                    Spacer(minLength: 0)
                    if let toggle {
                        if state.running == toggle.id {
                            PendingDot()
                        } else {
                            Button { handlers.run(toggle, state.answers[toggle.id ?? ""] ?? [:]) } label: { Switch(on: on) }
                                .buttonStyle(.plain).disabled(state.running != nil)
                        }
                    }
                }
                Text(words).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).fixedSize(horizontal: false, vertical: true)
            }
        }
    }

    func accountsTile(_ s: Section) -> some View {
        let cells = s.items?.first?.row?.cells?.first
        return TileBox {
            VStack(alignment: .leading, spacing: 4) {
                HStack(spacing: 6) {
                    Image(systemName: Token.Symbol.person).font(.system(size: Token.Space.small)).foregroundStyle(palette.accent)
                    Text(s.title ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
                    Spacer(minLength: 0)
                    Image(systemName: Token.Symbol.forward).font(.system(size: Token.Space.small - 2)).foregroundStyle(palette.subtle)
                }
                ValueText(cells, muted: true, size: Token.Space.small).fixedSize(horizontal: false, vertical: true)
            }
        }
    }

    var menu: some View {
        VStack(alignment: .leading, spacing: 6) {
            Divider()
            ForEach(Array(page.pageActions.enumerated()), id: \.offset) { _, a in
                if state.running == a.id {
                    PendingDot()
                } else {
                    let line = MenuLine(action: a, symbol: Self.symbol(a), danger: a.id == "stop-all") { perform(a, handlers) }.disabled(state.running != nil)
                    switch a.key {
                    case "cmd+n": line.keyboardShortcut("n", modifiers: .command)
                    case "cmd+t": line.keyboardShortcut("t", modifiers: .command)
                    default: line
                    }
                }
            }
        }
    }

    static func symbol(_ a: Action) -> String {
        switch a.id {
        case "new-vm": Token.Symbol.plus
        case "open-station", "start": Token.Symbol.terminal
        case "stop-all": Token.Symbol.stop
        default: Token.Symbol.forward
        }
    }
}

// invariant: SPEC §10.2's switcher, vision panel b: a search, the machines
// opened last, every machine as a tree, then Show all machines and Add a
// Mac.
struct SwitcherContent: View {
    let page: Page
    let state: PopoverState
    let handlers: PopoverHandlers
    @Environment(\.stationPalette) private var palette

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

    var body: some View {
        VStack(alignment: .leading, spacing: Token.Space.gap + 2) {
            HStack(spacing: 6) {
                Image(systemName: Token.Symbol.search).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
                TypedField("Search machines", text: Binding(get: { state.search }, set: handlers.search))
            }
            if let tree {
                if !state.recent.isEmpty, state.search.isEmpty {
                    title("Recent")
                    MachinesTree(section: tree, capsules: true, only: Set(state.recent), open: handlers.openMachine)
                }
                title(tree.title ?? "")
                MachinesTree(section: tree, capsules: true, only: matching(tree), open: handlers.openMachine)
            }
            SheetFor(page: page, state: state, handlers: handlers)
            VStack(alignment: .leading, spacing: 6) {
                Divider()
                Button(action: handlers.back) {
                    HStack(spacing: Token.Space.gap) {
                        Image(systemName: Token.Symbol.mac).font(.system(size: Token.Space.small)).frame(width: 18)
                        Text("Show all machines").font(.system(size: Token.Space.text))
                        Spacer()
                        Text(Token.Key.shown("cmd+0")).font(.system(size: Token.Space.small)).foregroundStyle(palette.subtle)
                    }
                    .foregroundStyle(palette.text)
                }
                .buttonStyle(.plain)
                ForEach(Array(page.pageActions.enumerated()), id: \.offset) { _, a in
                    MenuLine(action: a, symbol: Token.Symbol.plus, danger: false) { perform(a, handlers) }
                }
            }
        }
    }

    func title(_ t: String) -> some View {
        Text(t).font(.system(size: Token.Space.small, weight: .semibold)).foregroundStyle(palette.muted)
    }
}

// invariant: SPEC §10.2's page of one machine, vision panels c and f: its
// name, kind and state, its sections, its actions; each action names its
// call and asks at its call's level.
struct MachineContent: View {
    let page: Page
    let state: PopoverState
    let handlers: PopoverHandlers
    let width: CGFloat
    @Environment(\.stationPalette) private var palette

    var head: Section? { page.section("machine") }

    var body: some View {
        VStack(alignment: .leading, spacing: Token.Space.gap + 2) {
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
        let symbol = page.context?.kind == "mac" ? Token.Symbol.mac : Token.Symbol.vm
        return HStack(alignment: .top, spacing: Token.Space.gap) {
            Image(systemName: symbol).font(.system(size: Token.Tile.title - 2)).foregroundStyle(palette.accent).frame(width: 24)
            VStack(alignment: .leading, spacing: 2) {
                Text(s.title ?? "").font(.system(size: Token.Tile.title, weight: .bold)).foregroundStyle(palette.text)
                Text((s.caption ?? []).map { Format.show($0) }.joined(separator: Token.Glyph.sep)).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Spacer(minLength: 0)
            if let row {
                HStack(spacing: 4) {
                    MarkView(row.mark)
                    Text(row.cells.flatMap { $0.first }.map { Format.show($0) } ?? "").font(.system(size: Token.Space.small, weight: .semibold)).foregroundStyle(palette.text)
                }
                .padding(.horizontal, 8).padding(.vertical, 3)
                .background(Capsule().fill(palette.of(row.mark ?? .ok).opacity(0.18)))
            }
        }
    }

    @ViewBuilder
    func section(_ s: Section) -> some View {
        switch s.id {
        case "activity":
            HStack(spacing: 6) {
                ForEach(Array((s.items ?? []).compactMap(\.row).enumerated()), id: \.offset) { _, r in
                    MetricTile(label: r.label ?? "", value: r.cells?.first, spark: nil)
                }
            }
        case "vms":
            VStack(alignment: .leading, spacing: 6) {
                title(s)
                MachinesTree(section: s, capsules: true, only: nil, open: handlers.openMachine)
            }
        case "ports":
            VStack(alignment: .leading, spacing: 6) {
                title(s)
                TileBox {
                    VStack(alignment: .leading, spacing: 6) {
                        ItemsView(s.items ?? [], columns: s.columns ?? [], width: width - 20, links: LinkActions(open: handlers.open, qr: state.qr, toggleQR: handlers.toggleQR))
                        ForEach(Array(page.pageActions.filter { $0.id == "share-port" }.enumerated()), id: \.offset) { _, a in
                            MenuLine(action: a, symbol: Token.Symbol.plus, danger: false) { perform(a, handlers) }
                        }
                    }
                }
            }
        default:
            VStack(alignment: .leading, spacing: 4) {
                title(s)
                TileBox {
                    VStack(alignment: .leading, spacing: 3) {
                        ForEach(Array((s.items ?? []).enumerated()), id: \.offset) { _, item in
                            if let row = item.row, !page.rowActions(row.id ?? "").isEmpty {
                                HStack(spacing: 6) {
                                    RowView(row, columns: s.columns ?? [], width: width - 110)
                                    Spacer(minLength: 0)
                                    ForEach(Array(page.rowActions(row.id ?? "").enumerated()), id: \.offset) { _, a in
                                        SheetButton(a.label ?? "", role: .plain) { perform(a, handlers) }.disabled(state.running != nil)
                                    }
                                }
                            } else {
                                ItemsView([item], columns: s.columns ?? [], width: width - 20)
                            }
                        }
                    }
                }
            }
        }
    }

    func title(_ s: Section) -> some View {
        HStack(spacing: 6) {
            Text(s.title ?? "").font(.system(size: Token.Space.small, weight: .semibold)).foregroundStyle(palette.muted)
            ForEach(Array((s.caption ?? []).enumerated()), id: \.offset) { _, c in ValueText(c, muted: true, size: Token.Space.small) }
        }
    }

    var buttons: some View {
        let actions = page.pageActions.filter { $0.id != "share-port" && $0.id != "open-station" }
        return VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 6) {
                ForEach(Array(actions.enumerated()), id: \.offset) { _, a in
                    if state.running == a.id {
                        PendingDot()
                    } else {
                        let label = (a.form != nil || a.confirm?.type != nil) ? "\(a.label ?? "")\(Token.Glyph.ellipsis)" : a.label ?? ""
                        SheetButton(label, role: a.id == "restart" || a.id == "unlock" ? .accent : (a.confirm?.danger == true ? .danger : .plain)) { perform(a, handlers) }
                            .disabled(state.running != nil)
                    }
                }
                Spacer(minLength: 0)
            }
            if let open = page.action("open-station") {
                Divider()
                MenuLine(action: open, symbol: Token.Symbol.terminal, danger: false) { handlers.run(open, [:]) }
                    .keyboardShortcut("t", modifiers: .command)
            }
        }
    }
}
