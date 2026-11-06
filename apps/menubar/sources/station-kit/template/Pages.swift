import SwiftUI

// value: a measure as the references' KPI tile ("Agents 10 ▮▮▮"): its
// name top left in grey, its value large with its unit small bottom left,
// its bar meter bottom right with the latest bar in the accent.
struct MetricTile: View {
    let label: String
    let value: Value?
    let spark: [Int]?
    var id: String? = nil
    var meter: Bars? = nil
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationNow) private var now

    // value: whether the measure is still being read: no value yet, or
    // the page's pending one.
    var waiting: Bool { value == nil || value?.kind == "pending" }

    var body: some View {
        let (number, unit, _) = Self.parts(value.map { Format.show($0, now: now) } ?? "")
        GlassCard(id: id, padding: Token.Tile.padding + 1, spacing: 0, fill: true) {
            Text(label).font(.system(size: Token.Space.small + 1.5, weight: .medium)).foregroundStyle(palette.muted).lineLimit(1)
            Spacer(minLength: 8)
            HStack(alignment: .lastTextBaseline, spacing: 4) {
                if waiting || number.isEmpty {
                    Self.figure(Token.Glyph.missing, "", palette: palette, ink: palette.subtle)
                } else {
                    Self.figure(number, unit, palette: palette)
                }
                Spacer(minLength: 3)
                SwiftUI.Group {
                    if waiting {
                        Bars.pending(height: Token.Tile.meter)
                    } else if let spark, spark.count > 1 {
                        Bars(Array(spark.suffix(Token.Tile.meterBars)), height: Token.Tile.meter)
                    } else if let meter {
                        meter
                    }
                }
                .frame(minWidth: 10, maxWidth: Token.Tile.meterWidth)
                .alignmentGuide(.lastTextBaseline) { $0[.bottom] - 2 }
            }
        }
    }

    // value: a figure as the references draw one: the number 20 pt, its
    // unit after it 11 pt secondary.
    static func figure(_ number: String, _ unit: String, palette: Token.Palette, ink: Color? = nil) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 2) {
            Text(number).font(.system(size: Token.Tile.figure, weight: .semibold)).foregroundStyle(ink ?? palette.text)
                .monospacedDigit().contentTransition(.numericText())
            if !unit.isEmpty {
                Text(unit).font(.system(size: Token.Space.small - 0.5, weight: .medium)).foregroundStyle(palette.muted)
            }
        }
        .lineLimit(1)
        .fixedSize()
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

    // value: the peak of a series, said in the measure's own words.
    static func peak(_ points: [Int]?, _ say: (Int) -> String) -> String? {
        guard let top = points?.max(), (points?.count ?? 0) > 1 else { return nil }
        return "Peak \(say(top))"
    }
}

// value: the references' wide chart card ("Last 24 hours"): its title
// and its words on the right, its bars with the recent ones accented, its
// start and Now under them.
struct ChartCard: View {
    let title: String
    let words: String?
    let points: [Int]
    let start: String
    var id: String? = nil
    var height: CGFloat = Token.Tile.chart
    @Environment(\.stationPalette) private var palette

    var body: some View {
        GlassCard(id: id, spacing: 8) {
            HStack(alignment: .firstTextBaseline) {
                Text(title).font(.system(size: Token.Space.small + 1.5, weight: .semibold)).foregroundStyle(palette.text.opacity(0.88))
                Spacer(minLength: Token.Space.gap)
                if let words {
                    Text(words).font(.system(size: Token.Space.small + 1.5, weight: .semibold)).foregroundStyle(palette.text.opacity(0.88)).lineLimit(1).monospacedDigit()
                        .contentTransition(.numericText())
                }
            }
            Bars(points, height: height, accented: 2, chart: true)
            HStack {
                Text(start)
                Spacer()
                Text("Now")
            }
            .font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
        }
    }
}

// value: a machine's state as a dot: filled in its mark's colour, hollow
// when it is not running; the only colour a machine's line wears.
struct StateDot: View {
    let mark: Mark?
    @Environment(\.stationPalette) private var palette

    var body: some View {
        if let mark {
            if mark == .pending {
                PendingDot()
            } else if mark == .absent {
                Circle().strokeBorder(palette.grey, lineWidth: 1.2).frame(width: Token.Tile.dot, height: Token.Tile.dot)
            } else {
                Circle().fill(palette.of(mark)).frame(width: Token.Tile.dot, height: Token.Tile.dot)
            }
        }
    }
}

// value: a machine's line as the references' "Live" list: its small
// square icon, its name and kind in grey, the words under it, its state
// dot; the line opens its machine's page.
struct MachineLine: View {
    let row: Row
    let symbol: String
    let capsule: Bool
    var morph = true
    let open: (String) -> Void
    @Environment(\.stationPalette) private var palette

    var kind: String { row.cells?.first?.text ?? "" }
    var sub: String {
        let rest = (row.cells ?? []).dropFirst().filter { !$0.isNone }.map { Format.show($0) }
        return ([kind == "Mac" ? kind : nil] + rest.map { Optional($0) }).compactMap { $0 }.joined(separator: Token.Glyph.sep)
    }

    var dim: Bool {
        switch row.mark {
        case .absent, .offHere, .notKnown, nil: true
        default: false
        }
    }

    var body: some View {
        Button { open(row.id ?? "") } label: {
            HStack(spacing: Token.Space.gap + 2) {
                IconTile(symbol, dim: dim, pulse: row.mark == .pending)
                VStack(alignment: .leading, spacing: 1) {
                    HStack(alignment: .firstTextBaseline, spacing: 6) {
                        Text(row.label ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text).lineLimit(1)
                        if capsule && kind != "Mac" && !kind.isEmpty {
                            Text(kind).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
                        }
                    }
                    if !sub.isEmpty {
                        Text(sub).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).lineLimit(1)
                    }
                }
                Spacer(minLength: 0)
                StateDot(mark: row.mark)
            }
            .frame(minHeight: Token.Tile.line)
            .padding(.vertical, 3)
            .modifier(MorphID(id: morph ? row.label.map { PageID.of($0) } : nil))
            .stationHover()
        }
        .buttonStyle(.plain)
    }
}

// invariant: SPEC §10.2's Machines as the references' tree: each Mac,
// then its Servers, then its Workspaces, each group named in grey, tied
// to its Mac by a thin line; a line opens its machine's page.
struct MachinesTree: View {
    let section: Section
    let capsules: Bool
    let only: Set<String>?
    let open: (String) -> Void
    @Environment(\.stationPalette) private var palette

    enum Line {
        case caption(String)
        case row(Row)
    }

    static let indent = Token.Tile.icon + Token.Space.gap + 2
    static let stem = Token.Tile.icon / 2

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            ForEach(Array((section.items ?? []).enumerated()), id: \.offset) { _, item in
                if let g = item.group, g.head?.cells?.isEmpty ?? true {
                    flat(lines([item]))
                } else if let mac = item.group {
                    VStack(alignment: .leading, spacing: 0) {
                        if let head = mac.head, only == nil || only!.contains(head.id ?? "") {
                            MachineLine(row: head, symbol: Token.Symbol.mac, capsule: false, morph: only == nil, open: open)
                        }
                        if only == nil { tree(lines(mac.items ?? [])) } else { flat(lines(mac.items ?? [])) }
                    }
                } else if let row = item.row, only == nil || only!.contains(row.id ?? "") {
                    MachineLine(row: row, symbol: Token.Symbol.vm, capsule: capsules, morph: only == nil, open: open)
                } else if let empty = item.empty {
                    EmptyLine(empty)
                }
            }
        }
    }

    func lines(_ items: [Item]) -> [Line] {
        var out: [Line] = []
        for item in items {
            if let g = item.group {
                let rows = (g.items ?? []).compactMap(\.row).filter { only == nil || only!.contains($0.id ?? "") }
                if !rows.isEmpty {
                    if only == nil, let label = g.head?.label { out.append(.caption(label)) }
                    out += rows.map { .row($0) }
                }
            } else if let row = item.row, only == nil || only!.contains(row.id ?? "") {
                out.append(.row(row))
            }
        }
        return out
    }

    func line(_ l: Line) -> some View {
        SwiftUI.Group {
            switch l {
            case let .caption(text):
                Text(text).font(.system(size: Token.Space.small, weight: .medium)).foregroundStyle(palette.subtle)
                    .padding(.top, 4).padding(.bottom, 1)
            case let .row(row):
                MachineLine(row: row, symbol: Token.Symbol.vm, capsule: capsules || row.cells?.first?.text == "Server", morph: only == nil, open: open)
            }
        }
    }

    func flat(_ lines: [Line]) -> some View {
        ForEach(Array(lines.enumerated()), id: \.offset) { _, l in line(l) }
    }

    func tree(_ lines: [Line]) -> some View {
        let last = lines.lastIndex { if case .row = $0 { true } else { false } } ?? -1
        return ForEach(Array(lines.enumerated()), id: \.offset) { i, l in
            let kind: Branch.Kind = { if case .caption = l { return .pass }; return i == last ? .last : .elbow }()
            if i <= last {
                line(l)
                    .padding(.leading, Self.indent)
                    .background(alignment: .leading) { Branch(kind, x: Self.stem, reach: Self.indent - 3) }
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

// invariant: SPEC §10.2's Home, vision panel a, in the references'
// order: the three measures, what needs you, the last hour, the
// machines, then Awake, Ports and Accounts as rows of one card, then the
// menu.
struct HomeContent: View {
    let page: Page
    let state: PopoverState
    let handlers: PopoverHandlers
    let width: CGFloat
    @Environment(\.stationPalette) private var palette

    var body: some View {
        VStack(alignment: .leading, spacing: Token.Glass.spacing) {
            if let strip = page.strip {
                HStack(spacing: Token.Glass.spacing) {
                    MetricTile(label: "CPU", value: strip.cpu, spark: strip.spark)
                    MetricTile(label: "Memory", value: strip.memory, spark: strip.memory_spark)
                    MetricTile(label: "Network", value: strip.net?.first, spark: strip.net_spark)
                }
                .fixedSize(horizontal: false, vertical: true)
            }
            if let needs = page.section(TodoBadge.section) { needsCard(needs) }
            if let spark = page.strip?.spark, spark.count > 1 {
                Button { handlers.push(Navigation.activity) } label: {
                    ChartCard(title: Self.lastHour, words: MetricTile.peak(spark) { "\($0) %" }, points: spark, start: Self.hourAgo, id: Navigation.activity)
                }
                .buttonStyle(.plain)
            }
            if let machines = page.section("machines") {
                GroupCard(machines.title ?? "") {
                    MachinesTree(section: machines, capsules: false, only: nil, open: handlers.openMachine)
                }
            }
            places
            SheetFor(page: page, state: state, handlers: handlers)
            if state.confirming == nil || page.action(state.confirming ?? "")?.on == "row" {
                menu
            }
        }
    }

    static let lastHour = "Last hour"
    static let hourAgo = "1 h ago"

    // value: a rate of the strip's network series, in KB/s, as a size.
    static func rate(_ kb: Int) -> String {
        var v = Value(kind: "size")
        v.n = Int64(kb) * 1000
        v.unit = Token.Glyph.slash + "s"
        return Format.show(v)
    }

    // value: what needs you as the references' "Live 10": its title and
    // count, its flat rows, each row's action a small text button at its
    // end.
    func needsCard(_ s: Section) -> some View {
        let n = TodoBadge.todo(page)?.n ?? 0
        return GroupCard(s.title ?? "", count: n > 0 ? n : nil, words: n > 0 ? nil : s.caption?.first?.text, spacing: 2) {
            ForEach(Array((s.items ?? []).enumerated()), id: \.offset) { _, item in
                if let row = item.row {
                    HStack(spacing: Token.Space.gap) {
                        MarkView(row.mark)
                        Text(row.label ?? "").font(.system(size: Token.Space.text)).foregroundStyle(palette.text).lineLimit(1)
                        Spacer(minLength: 4)
                        ForEach(Array(page.rowActions(row.id ?? "").enumerated()), id: \.offset) { _, a in
                            if state.running == a.id {
                                PendingDot()
                            } else {
                                TextAction(a.label ?? "") { perform(a, handlers) }.disabled(state.running != nil)
                            }
                        }
                    }
                    .frame(minHeight: Token.Space.row + 2)
                } else if let empty = item.empty {
                    EmptyLine(empty)
                } else if let link = item.link {
                    Text(link.label ?? "").font(.system(size: Token.Space.small + 1, weight: .medium)).foregroundStyle(palette.accent).padding(.top, 2)
                }
            }
        }
    }

    // value: the menu bar's own pages as rows of one card: Awake with its
    // switch inline, Ports, Accounts, each opening its page.
    var places: some View {
        let awake = page.section("awake")
        let accounts = page.section("accounts")
        let live = state.connection == .live || state.connection == .connecting
        return SwiftUI.Group {
            if awake != nil || accounts != nil || live {
                GlassCard(spacing: 0) {
                    if let awake { awakeRow(awake) }
                    if live {
                        if awake != nil { Rule() }
                        PlaceRow(symbol: Token.Symbol.port, title: Self.ports, words: nil, dim: false) { handlers.push(Navigation.ports) } trailing: { Chevron() }
                    }
                    if let accounts {
                        if awake != nil || live { Rule() }
                        PlaceRow(symbol: Token.Symbol.accounts, title: accounts.title ?? "", words: accounts.items?.first?.row?.cells?.first.map { Format.show($0) }, dim: false) {} trailing: { Chevron() }
                    }
                }
            }
        }
    }

    static var ports: String { Popover.pages.first { $0.watch == Navigation.ports }?.label ?? "" }

    func awakeRow(_ s: Section) -> some View {
        let toggle = (page.actions ?? []).first { $0.rows?.contains("awake") == true && $0.run != nil }
        let on = toggle?.id == "power-sleep"
        let words = (s.items?.first?.row?.cells?.first).map { Format.show($0) }
        return PlaceRow(symbol: on ? Token.Symbol.awakeOn : Token.Symbol.awakeOff, title: s.title ?? "", words: words, dim: !on) { handlers.push(Navigation.awake) } trailing: {
            if let toggle {
                if state.running == toggle.id {
                    PendingDot()
                } else {
                    Button { handlers.run(toggle, state.answers[toggle.id ?? ""] ?? [:]) } label: { Switch(on: on) }
                        .buttonStyle(.plain).disabled(state.running != nil)
                }
            }
        }
    }

    var menu: some View {
        VStack(alignment: .leading, spacing: 0) {
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
        .padding(.horizontal, 6)
        .padding(.top, 2)
    }

    static func symbol(_ a: Action) -> String {
        switch a.id {
        case "new-vm": Token.Symbol.plus
        case "open-station", "start": Token.Symbol.terminal
        case "stop-all": Token.Symbol.stop
        case "restart": Token.Symbol.restart
        case "unlock": Token.Symbol.unlock
        default: Token.Symbol.forward
        }
    }
}

// value: a row of a card that opens a page: its small square icon, its
// title and grey words, what it ends with (a switch, a chevron).
struct PlaceRow<Trailing: View>: View {
    let symbol: String
    let title: String
    let words: String?
    let dim: Bool
    let tap: () -> Void
    let trailing: Trailing
    @Environment(\.stationPalette) private var palette

    init(symbol: String, title: String, words: String?, dim: Bool, tap: @escaping () -> Void, @ViewBuilder trailing: () -> Trailing) {
        self.symbol = symbol
        self.title = title
        self.words = words
        self.dim = dim
        self.tap = tap
        self.trailing = trailing()
    }

    var body: some View {
        HStack(spacing: Token.Space.gap + 2) {
            Button(action: tap) {
                HStack(spacing: Token.Space.gap + 2) {
                    IconTile(symbol, dim: dim)
                    VStack(alignment: .leading, spacing: 1) {
                        Text(title).font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
                        if let words, !words.isEmpty {
                            Text(words).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).lineLimit(2).fixedSize(horizontal: false, vertical: true)
                        }
                    }
                    Spacer(minLength: 0)
                }
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            trailing
        }
        .frame(minHeight: Token.Tile.line + 4)
        .padding(.vertical, 3)
        .stationHover()
    }
}

// value: the chevron a row that opens a page ends with.
struct Chevron: View {
    @Environment(\.stationPalette) private var palette

    var body: some View {
        Image(systemName: Token.Symbol.forward).font(.system(size: Token.Space.small - 1, weight: .semibold)).foregroundStyle(palette.subtle)
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
            IconTile(mac ? Token.Symbol.mac : Token.Symbol.vm, dim: mark == .absent, size: Token.Tile.icon + 6)
            VStack(alignment: .leading, spacing: 1) {
                Text(s.title ?? "").font(.system(size: Token.Space.head + 2, weight: .bold)).foregroundStyle(palette.text)
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
        .padding(.horizontal, 4)
        .modifier(MorphID(id: s.title.map { PageID.of($0) }))
    }

    @ViewBuilder
    func section(_ s: Section) -> some View {
        switch s.id {
        case "activity":
            HStack(spacing: Token.Glass.spacing) {
                ForEach(Array((s.items ?? []).compactMap(\.row).enumerated()), id: \.offset) { _, r in
                    let spark = r.cells?.dropFirst().first?.points
                    MetricTile(label: r.label ?? "", value: r.cells?.first, spark: spark)
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
                    upTile(r)
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

    // value: an Overview tile: Up with its whole meter lit while its span
    // is known, Restarts with one bar lit per restart.
    func upTile(_ r: Row) -> MetricTile {
        let value = r.cells?.first
        if r.id == "up" {
            let lit = value?.isNone ?? true ? 0 : Bars.slots
            return MetricTile(label: r.label ?? "", value: Self.span(value), spark: nil, meter: Bars.meter(lit, of: Bars.slots, height: Token.Tile.meter, tint: palette.good))
        }
        let count = Int(value?.items?.first?.n ?? 0)
        return MetricTile(label: r.label ?? "", value: value, spark: nil, meter: Bars.meter(count, of: Bars.slots, height: Token.Tile.meter))
    }

    // value: the Up tile's figure without the word its label already says:
    // `up 12 d` is shown 12 d.
    static func span(_ v: Value?) -> Value? {
        guard var v, let text = v.text, text.hasPrefix("up ") else { return v }
        v.text = String(text.dropFirst(3))
        return v
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

    // invariant: the page's own actions as the references' footer: plain
    // text rows (Restart, Unlock, Open in Terminal); Stop and Start are
    // the header's switch; what destroys a red text button at the bottom.
    var buttons: some View {
        let actions = page.pageActions.filter { !["share-port", "stop", "start", "delete"].contains($0.id ?? "") }
        let destroy = page.pageActions.filter { $0.id == "delete" }
        return VStack(alignment: .leading, spacing: 0) {
            ForEach(Array(actions.enumerated()), id: \.offset) { _, a in
                if state.running == a.id {
                    PendingDot().padding(.vertical, 4)
                } else if a.id == "open-station" {
                    MenuLine(action: a, symbol: Token.Symbol.terminal, danger: false) { handlers.run(a, [:]) }
                        .keyboardShortcut("t", modifiers: .command)
                } else {
                    MenuLine(action: a, symbol: HomeContent.symbol(a), danger: a.confirm?.danger == true) { perform(a, handlers) }
                        .disabled(state.running != nil)
                }
            }
            ForEach(Array(destroy.enumerated()), id: \.offset) { _, a in
                HStack {
                    Spacer()
                    TextAction(Self.label(a), symbol: Token.Symbol.trash, danger: true) { perform(a, handlers) }.disabled(state.running != nil)
                }
                .padding(.top, 6)
            }
        }
        .padding(.horizontal, 6)
        .padding(.top, 2)
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
                IconTile(symbol)
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
                TextAction(a.label ?? "") { tap(a) }.disabled(busy)
            }
        }
        .frame(minHeight: Token.Tile.line)
        .padding(.vertical, 2)
        .overlay(alignment: .top) {
            if rule { Rectangle().fill(palette.faint).frame(height: 0.5).offset(y: -1) }
        }
    }
}
