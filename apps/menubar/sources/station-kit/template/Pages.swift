import SwiftUI

// value: a rounded tile of glass (vision a-f).
struct TileBox<Content: View>: View {
    let id: String?
    let tint: Color?
    let content: Content

    init(id: String? = nil, tint: Color? = nil, @ViewBuilder _ content: () -> Content) {
        self.id = id
        self.tint = tint
        self.content = content()
    }

    var body: some View {
        GlassCard(tint: tint, id: id) { content }
    }
}

// value: a measure as the references' KPI tile: its name top left, its
// value large with its unit small, its bar meter with the latest bar in
// the accent, and a secondary line under it ("Peak 55 %", "of 17.2 GB").
struct MetricTile: View {
    let label: String
    let value: Value?
    let spark: [Int]?
    var id: String? = nil
    var caption: String? = nil
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationNow) private var now

    var body: some View {
        let (number, unit, of) = Self.parts(value.map { Format.show($0, now: now) } ?? "")
        GlassCard(id: id, fill: true) {
            VStack(alignment: .leading, spacing: 3) {
                Text(label).font(.system(size: Token.Space.small + 1, weight: .medium)).foregroundStyle(palette.muted).lineLimit(1)
                Self.figure(number, unit, palette: palette).frame(height: Token.Tile.figure + 4, alignment: .leading)
                if let spark, spark.count > 1 {
                    Bars(Array(spark.suffix(12))).padding(.vertical, 1)
                }
                if let line = caption ?? of {
                    Text(line).font(.system(size: Token.Space.small - 1)).foregroundStyle(palette.muted).lineLimit(1).minimumScaleFactor(0.8).monospacedDigit()
                }
            }
        }
    }

    // value: a figure as the references draw one: the number 22 pt, its
    // unit after it 11 pt secondary.
    static func figure(_ number: String, _ unit: String, palette: Token.Palette) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 2) {
            Text(number).font(.system(size: Token.Tile.figure, weight: .semibold)).foregroundStyle(palette.text)
                .monospacedDigit().contentTransition(.numericText())
            if !unit.isEmpty {
                Text(unit).font(.system(size: Token.Space.small, weight: .medium)).foregroundStyle(palette.muted)
            }
        }
        .lineLimit(1)
        .minimumScaleFactor(0.5)
    }

    // value: a shown value cut into its number, its unit, and what it is
    // out of: `14.1 /17.2 GB` is 14.1, GB, of 17.2 GB.
    static func parts(_ shown: String) -> (String, String, String?) {
        let (number, unit) = split(shown)
        if let cut = unit.range(of: Token.Glyph.sep) { return (number, String(unit[..<cut.lowerBound]), String(unit[cut.upperBound...])) }
        guard unit.hasPrefix(Token.Glyph.slash) else { return (number, unit, nil) }
        let total = unit.dropFirst().trimmingCharacters(in: .whitespaces)
        return (number, String(total.split(separator: " ").last ?? ""), "of \(total)")
    }

    static func split(_ shown: String) -> (String, String) {
        let figure = Set("0123456789.,")
        let head = shown.prefix { figure.contains($0) }
        guard !head.isEmpty else { return (shown, "") }
        return (String(head), shown.dropFirst(head.count).trimmingCharacters(in: .whitespaces))
    }

    // value: a series drawn against its own peak, so a rate in KB/s draws
    // like a share in percent.
    static func scaled(_ points: [Int]) -> [Int] {
        let top = max(points.max() ?? 1, 1)
        return top <= 100 ? points : points.map { $0 * 100 / top }
    }

    // value: the peak line under a measure: its highest point, said in
    // the measure's own words.
    static func peak(_ points: [Int]?, _ say: (Int) -> String) -> String? {
        guard let top = points?.max(), (points?.count ?? 0) > 1 else { return nil }
        return "Peak \(say(top))"
    }
}

// value: a machine's state as a dot: filled in its mark's colour, hollow
// when it is not running.
struct StateDot: View {
    let mark: Mark?
    @Environment(\.stationPalette) private var palette

    var body: some View {
        if let mark {
            if mark == .pending {
                PendingDot()
            } else if mark == .absent {
                Circle().strokeBorder(palette.grey, lineWidth: 1.5).frame(width: Token.Tile.dot + 1, height: Token.Tile.dot + 1)
            } else {
                Circle().fill(palette.of(mark)).frame(width: Token.Tile.dot, height: Token.Tile.dot)
            }
        }
    }
}

// value: a kind's capsule beside a machine's name: Server, Workspace.
struct KindCapsule: View {
    let kind: String
    @Environment(\.stationPalette) private var palette

    var body: some View {
        Text(kind).font(.system(size: Token.Space.small - 1.5, weight: .semibold)).foregroundStyle(palette.teal)
            .padding(.horizontal, 5).padding(.vertical, 1)
            .background(Capsule().fill(palette.teal.opacity(0.2)))
    }
}

// value: a machine's line: its state circle, its name, a kind capsule,
// the words under it, its state dot; the line opens its machine's page.
struct MachineLine: View {
    let row: Row
    let symbol: String
    let capsule: Bool
    var rule = false
    var selected = false
    var morph = true
    let open: (String) -> Void
    @Environment(\.stationPalette) private var palette

    var kind: String { row.cells?.first?.text ?? "" }
    var sub: String {
        let rest = (row.cells ?? []).dropFirst().filter { !$0.isNone }.map { Format.show($0) }
        return ([kind == "Mac" ? kind : nil] + rest.map { Optional($0) }).compactMap { $0 }.joined(separator: Token.Glyph.sep)
    }

    var tint: Color? {
        switch row.mark {
        case .absent, .offHere, .notKnown, .pending, nil: return nil
        case .broken: return palette.bad
        case .attention: return palette.warn
        default: return symbol == Token.Symbol.mac ? palette.accent : palette.teal
        }
    }

    var body: some View {
        Button { open(row.id ?? "") } label: {
            HStack(spacing: Token.Space.gap + 2) {
                IconTile(symbol, tint: tint, pulse: row.mark == .pending)
                VStack(alignment: .leading, spacing: 1) {
                    HStack(spacing: 5) {
                        Text(row.label ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(selected ? palette.onTint : palette.text)
                        if capsule && kind != "Mac" { KindCapsule(kind: kind) }
                    }
                    if !sub.isEmpty {
                        Text(sub).font(.system(size: Token.Space.small)).foregroundStyle(selected ? palette.onTint.opacity(0.85) : palette.muted).lineLimit(1)
                    }
                }
                Spacer(minLength: 0)
                StateDot(mark: row.mark)
            }
            .frame(minHeight: Token.Tile.rowHeight - 6)
            .padding(.vertical, 3)
            .modifier(MorphID(id: morph ? row.label.map { PageID.of($0) } : nil))
            .stationHover()
            .overlay(alignment: .top) {
                if rule { Rectangle().fill(palette.faint).frame(height: 0.5).offset(y: -2) }
            }
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
            ForEach(Array((section.items ?? []).enumerated()), id: \.offset) { i, item in
                if let g = item.group, g.head?.cells?.isEmpty ?? true {
                    groups([item])
                } else if let mac = item.group {
                    if let head = mac.head, only == nil || only!.contains(head.id ?? "") {
                        MachineLine(row: head, symbol: Token.Symbol.mac, capsule: false, rule: i > 0, morph: only == nil, open: open)
                    }
                    groups(mac.items ?? [])
                } else if let row = item.row {
                    MachineLine(row: row, symbol: Token.Symbol.vm, capsule: capsules, rule: i > 0, morph: only == nil, open: open)
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
                            Text(g.head?.label ?? "").font(.system(size: Token.Space.small, weight: .medium))
                                .foregroundStyle(palette.subtle).padding(.leading, Token.Tile.icon + Token.Space.gap + 2).padding(.top, 2)
                        }
                        ForEach(Array(rows.enumerated()), id: \.offset) { j, row in
                            MachineLine(row: row, symbol: Token.Symbol.vm, capsule: capsules || row.cells?.first?.text == "Server", rule: j > 0, morph: only == nil, open: open)
                                .padding(.leading, only == nil ? Token.Tile.icon + Token.Space.gap + 2 : 0)
                        }
                    }
                }
            } else if let row = item.row, only == nil || only!.contains(row.id ?? "") {
                MachineLine(row: row, symbol: Token.Symbol.vm, capsule: true, morph: only == nil, open: open).padding(.leading, Token.Tile.icon + Token.Space.gap + 2)
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
            MenuText(symbol: symbol, title: title, key: action.key.map(Token.Key.shown), danger: danger)
        }
        .buttonStyle(.plain)
    }
}

struct MenuText: View {
    let symbol: String
    let title: String
    let key: String?
    let danger: Bool
    @Environment(\.stationPalette) private var palette

    var body: some View {
        HStack(spacing: Token.Space.gap) {
            Image(systemName: symbol).font(.system(size: Token.Space.small + 1, weight: .medium)).frame(width: 18)
                .foregroundStyle(danger ? palette.bad : palette.muted)
            Text(title).font(.system(size: Token.Space.text))
            Spacer()
            if let key {
                Text(key).font(.system(size: Token.Space.small + 1)).foregroundStyle(palette.subtle)
            }
        }
        .foregroundStyle(danger ? palette.bad : palette.text)
        .padding(.vertical, 4)
        .stationHover()
    }
}

// value: the thin rule over a menu, as the vision's.
struct Rule: View {
    @Environment(\.stationPalette) private var palette

    var body: some View { Rectangle().fill(palette.faint).frame(height: 0.5) }
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
                HStack(spacing: Token.Glass.spacing) {
                    MetricTile(label: "CPU", value: strip.cpu, spark: strip.spark, id: Navigation.activity, caption: MetricTile.peak(strip.spark) { "\($0) %" })
                    MetricTile(label: "Memory", value: strip.memory, spark: strip.memory_spark)
                    MetricTile(label: "Network", value: strip.net?.first, spark: strip.net_spark, caption: MetricTile.peak(strip.net_spark) { Self.rate($0) })
                }
                .fixedSize(horizontal: false, vertical: true)
            }
            if let needs = page.section(TodoBadge.section) { needsTile(needs) }
            if let machines = page.section("machines") {
                GroupCard(machines.title ?? "") {
                    MachinesTree(section: machines, capsules: false, only: nil, open: handlers.openMachine)
                }
            }
            HStack(alignment: .top, spacing: Token.Glass.spacing) {
                if let awake = page.section("awake") { awakeTile(awake) }
                if let accounts = page.section("accounts") { accountsTile(accounts) }
            }
            if state.connection == .live || state.connection == .connecting {
                HStack(spacing: 6) {
                    ForEach(Array(Popover.pages.enumerated()), id: \.offset) { _, p in
                        Pill(p.label) { handlers.push(p.watch) }
                    }
                    Spacer(minLength: 0)
                }
            }
            SheetFor(page: page, state: state, handlers: handlers)
            if state.confirming == nil || page.action(state.confirming ?? "")?.on == "row" {
                menu
            }
        }
    }

    // value: a rate of the strip's network series, in KB/s, as a size.
    static func rate(_ kb: Int) -> String {
        var v = Value(kind: "size")
        v.n = Int64(kb) * 1000
        v.unit = Token.Glyph.slash + "s"
        return Format.show(v)
    }

    func needsTile(_ s: Section) -> some View {
        let todo = TodoBadge.todo(page)
        let tint: Color = todo?.mark == .broken ? palette.bad : palette.warn
        return TileBox {
            HStack(alignment: .top, spacing: Token.Space.gap + 2) {
                IconCircle(Token.Symbol.needs, tint: (todo?.n ?? 0) > 0 ? tint : nil)
                VStack(alignment: .leading, spacing: 5) {
                    Text(s.caption?.first?.text ?? s.title ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
                        .frame(minHeight: Token.Tile.circle)
                    ForEach(Array((s.items ?? []).enumerated()), id: \.offset) { _, item in
                        if let row = item.row {
                            HStack(spacing: 6) {
                                MarkView(row.mark)
                                Text(row.label ?? "").font(.system(size: Token.Space.small + 1)).foregroundStyle(palette.text).lineLimit(1)
                                Spacer(minLength: 4)
                                ForEach(Array(page.rowActions(row.id ?? "").enumerated()), id: \.offset) { _, a in
                                    if state.running == a.id {
                                        PendingDot()
                                    } else {
                                        Pill(a.label ?? "", role: .primary) { perform(a, handlers) }.disabled(state.running != nil)
                                    }
                                }
                            }
                        } else if let empty = item.empty {
                            EmptyLine(empty)
                        } else if let link = item.link {
                            Text(link.label ?? "").font(.system(size: Token.Space.small + 1, weight: .medium)).foregroundStyle(palette.accent)
                        }
                    }
                }
            }
        }
    }

    func awakeTile(_ s: Section) -> some View {
        let toggle = (page.actions ?? []).first { $0.rows?.contains("awake") == true && $0.run != nil }
        let on = toggle?.id == "awake-off"
        let words = (s.items?.first?.row?.cells?.first).map { Format.show($0) } ?? ""
        return TileBox(id: Navigation.awake) {
            VStack(alignment: .leading, spacing: 6) {
                HStack(spacing: 6) {
                    IconCircle(on ? Token.Symbol.awakeOn : Token.Symbol.awakeOff, tint: on ? palette.warn : nil)
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
            VStack(alignment: .leading, spacing: 6) {
                HStack(spacing: 6) {
                    IconCircle(Token.Symbol.accounts, tint: palette.indigo)
                    Text(s.title ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
                    Spacer(minLength: 0)
                    Image(systemName: Token.Symbol.forward).font(.system(size: Token.Space.small - 1, weight: .semibold)).foregroundStyle(palette.subtle)
                }
                ValueText(cells, muted: true, size: Token.Space.small).fixedSize(horizontal: false, vertical: true)
            }
        }
    }

    var menu: some View {
        VStack(alignment: .leading, spacing: 2) {
            Rule().padding(.bottom, 4)
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
        .padding(.horizontal, 4)
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
            TypedField("Search machines", text: Binding(get: { state.search }, set: handlers.search), symbol: Token.Symbol.search, key: Token.Key.shown("cmd+k"))
            if let tree {
                if !state.recent.isEmpty, state.search.isEmpty {
                    GroupCard("Recent") {
                        MachinesTree(section: tree, capsules: true, only: Set(state.recent), open: handlers.openMachine)
                    }
                }
                GroupCard(tree.title ?? "") {
                    MachinesTree(section: tree, capsules: true, only: matching(tree), open: handlers.openMachine)
                }
            }
            SheetFor(page: page, state: state, handlers: handlers)
            VStack(alignment: .leading, spacing: 2) {
                Rule().padding(.bottom, 4)
                Button(action: handlers.back) {
                    MenuText(symbol: Token.Symbol.mac, title: "Show all machines", key: Token.Key.shown("cmd+0"), danger: false)
                }
                .buttonStyle(.plain)
                ForEach(Array(page.pageActions.enumerated()), id: \.offset) { _, a in
                    MenuLine(action: a, symbol: Token.Symbol.plus, danger: false) { perform(a, handlers) }
                }
            }
            .padding(.horizontal, 4)
        }
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

    // value: the action the header's switch runs: Stop while it runs,
    // Start while it does not.
    var power: Action? { page.action("stop") ?? page.action("start") }

    func header(_ s: Section) -> some View {
        let row = s.items?.first?.row
        let mac = page.context?.kind == "mac"
        let mark = row?.mark ?? .ok
        return HStack(alignment: .center, spacing: Token.Space.gap + 2) {
            IconTile(mac ? Token.Symbol.mac : Token.Symbol.vm, tint: mark == .ok ? (mac ? palette.accent : palette.teal) : mark == .attention ? palette.warn : nil, size: Token.Tile.icon + 8)
            VStack(alignment: .leading, spacing: 1) {
                Text(s.title ?? "").font(.system(size: Token.Space.title, weight: .bold)).foregroundStyle(palette.text)
                Text((s.caption ?? []).map { Format.show($0) }.joined(separator: Token.Glyph.sep)).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Spacer(minLength: 0)
            if let row {
                let ink = mark == .absent ? palette.muted : palette.of(mark)
                StateSwitch(row.cells.flatMap { $0.first }.map { Format.show($0) } ?? "", ink: ink, on: mark != .absent, busy: power.map { state.running == $0.id } ?? false,
                            tap: power.map { a in { perform(a, handlers) } })
                    .disabled(state.running != nil)
            }
        }
        .modifier(MorphID(id: s.title.map { PageID.of($0) }))
    }

    @ViewBuilder
    func section(_ s: Section) -> some View {
        switch s.id {
        case "activity":
            HStack(spacing: Token.Glass.spacing) {
                ForEach(Array((s.items ?? []).compactMap(\.row).enumerated()), id: \.offset) { _, r in
                    MetricTile(label: r.label ?? "", value: r.cells?.first, spark: nil)
                }
            }
            .fixedSize(horizontal: false, vertical: true)
        case "vms":
            GroupCard(s.title ?? "", words: words(s)) {
                MachinesTree(section: s, capsules: true, only: nil, open: handlers.openMachine)
            }
        case "ports":
            let share = page.action("share-port")
            GroupCard(s.title ?? "", words: words(s), action: share.map { a in (Token.Glyph.add + "Share", { perform(a, handlers) }) }, spacing: 10) {
                ForEach(Array((s.items ?? []).compactMap(\.link).enumerated()), id: \.offset) { _, l in link(l) }
            }
        case "up":
            HStack(spacing: Token.Glass.spacing) {
                ForEach(Array(s.rows.enumerated()), id: \.offset) { _, r in
                    MetricTile(label: r.label ?? "", value: r.cells?.first, spark: nil)
                }
            }
            .fixedSize(horizontal: false, vertical: true)
        default:
            GroupCard(s.title ?? "", words: words(s)) {
                ForEach(Array((s.items ?? []).enumerated()), id: \.offset) { i, item in
                    if let row = item.row {
                        CardRow(row: row, rule: i > 0, actions: page.rowActions(row.id ?? ""), busy: state.running != nil, secret: s.id == "shared") { perform($0, handlers) }
                    } else {
                        ItemsView([item], columns: s.columns ?? [], width: width - 24)
                    }
                }
                if s.id == "shared" {
                    Text(Self.lent).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).fixedSize(horizontal: false, vertical: true).padding(.top, 6)
                }
            }
        }
    }

    // value: the calm line under the accounts a VM holds (SPEC §5.7: never
    // a token shown, every share ends by itself).
    static let lent = "Copies of this Mac's secrets, never shown. Each is taken back when its time runs out."

    func words(_ s: Section) -> String {
        (s.caption ?? []).map { Format.show($0) }.joined(separator: Token.Glyph.sep)
    }

    // value: a shared port as the references' link field: its name and
    // port, then its address in a well with Copy and QR; the QR unfolds
    // under it.
    func link(_ l: Link) -> some View {
        let url = l.value.flatMap(Format.url)
        let address = l.value.map { Format.show($0) } ?? ""
        return VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 6) {
                if let lead = l.lead, !lead.isNone { ValueText(lead).fontWeight(.medium) } else { Text(l.label ?? "").font(.system(size: Token.Space.text, weight: .medium)).foregroundStyle(palette.text) }
                Spacer(minLength: 0)
            }
            LinkField(address, qr: url.map { state.qr == $0.absoluteString } ?? false, copy: { handlers.copy(address) },
                      toggleQR: url.map { u in { handlers.toggleQR(u.absoluteString) } }, open: url.map { u in { handlers.open(u) } })
            if let url, state.qr == url.absoluteString {
                QRView(url.absoluteString).clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous)).frame(maxWidth: .infinity)
            }
        }
    }

    // invariant: the page's own actions: the plain ones as glass
    // buttons, the primary one prominent; Stop and Start are the header's
    // switch; Open in Terminal a menu line; what destroys a red text
    // button at the bottom.
    var buttons: some View {
        let actions = page.pageActions.filter { !["share-port", "open-station", "stop", "start", "delete"].contains($0.id ?? "") }
        let destroy = page.pageActions.filter { $0.id == "delete" }
        return VStack(alignment: .leading, spacing: 6) {
            if !actions.isEmpty {
                HStack(spacing: Token.Glass.spacing) {
                    ForEach(Array(actions.enumerated()), id: \.offset) { _, a in
                        if state.running == a.id {
                            PendingDot().frame(maxWidth: .infinity)
                        } else {
                            Pill(Self.label(a), role: a.id == "restart" || a.id == "unlock" ? .primary : (a.confirm?.danger == true ? .danger : .plain), wide: true) { perform(a, handlers) }
                                .disabled(state.running != nil)
                        }
                    }
                }
            }
            if let open = page.action("open-station") {
                Rule().padding(.vertical, 2)
                MenuLine(action: open, symbol: Token.Symbol.terminal, danger: false) { handlers.run(open, [:]) }
                    .keyboardShortcut("t", modifiers: .command)
                    .padding(.horizontal, 4)
            }
            ForEach(Array(destroy.enumerated()), id: \.offset) { _, a in
                HStack {
                    Spacer()
                    TextAction(Self.label(a), symbol: Token.Symbol.trash, danger: true) { perform(a, handlers) }.disabled(state.running != nil)
                }
                .padding(.horizontal, 4).padding(.top, 2)
            }
        }
    }

    static func label(_ a: Action) -> String {
        (a.form != nil || a.confirm?.type != nil) ? "\(a.label ?? "")\(Token.Glyph.ellipsis)" : a.label ?? ""
    }
}

// value: a line of a grouped card, as the vision's lists: its circle or
// its mark, its name, its value on the right, its actions as buttons.
struct CardRow: View {
    let row: Row
    let rule: Bool
    let actions: [Action]
    let busy: Bool
    var secret = false
    let tap: (Action) -> Void
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationNow) private var now

    var value: String {
        (row.cells ?? []).filter { !$0.isNone }.map { Format.show($0, now: now) }.filter { !$0.isEmpty }.joined(separator: Token.Glyph.sep)
    }

    var body: some View {
        HStack(spacing: Token.Space.gap + 2) {
            if row.mark == nil, let symbol = Token.Symbol.line(row.id ?? "") {
                IconTile(symbol, tint: palette.grey, size: Token.Tile.circle)
            } else if row.mark != nil {
                MarkView(row.mark)
            }
            Text(row.short ?? row.label ?? "").font(.system(size: Token.Space.text, weight: .medium)).foregroundStyle(palette.text).lineLimit(2)
                .fixedSize(horizontal: false, vertical: true).layoutPriority(1)
            Spacer(minLength: Token.Space.gap)
            if secret {
                Text(Token.Glyph.secret).font(.system(size: Token.Space.small - 2, weight: .bold)).tracking(1).foregroundStyle(palette.subtle).lineLimit(1).fixedSize()
            }
            if !value.isEmpty {
                Text(value).font(.system(size: Token.Space.small + 1)).foregroundStyle(palette.muted).lineLimit(1).truncationMode(.middle).monospacedDigit()
                    .contentTransition(.numericText())
                    .layoutPriority(value.count <= 20 ? 2 : 0)
            }
            ForEach(Array(actions.enumerated()), id: \.offset) { _, a in
                Pill(a.label ?? "") { tap(a) }.disabled(busy)
            }
        }
        .frame(minHeight: Token.Tile.rowHeight - 8)
        .padding(.vertical, 2)
        .overlay(alignment: .top) {
            if rule { Rectangle().fill(palette.faint).frame(height: 0.5).offset(y: -1) }
        }
    }
}
