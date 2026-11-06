import SwiftUI

// value: a measure as the vision's three side by side, drawn on the
// panel with no frame: its name, its figure with its unit small, its
// window as a Swift Charts line in the accent.
struct Metric: View {
    let label: String
    let value: Value?
    let spark: [Int]?
    @Environment(\.stationNow) private var now

    var waiting: Bool { value == nil || value?.kind == "pending" }

    var body: some View {
        let (number, unit, of) = Self.parts(value.map { Format.show($0, now: now) } ?? "")
        VStack(alignment: .leading, spacing: 3) {
            Text(label).font(.subheadline).foregroundStyle(.secondary).lineLimit(1)
            if waiting || number.isEmpty {
                Self.figure(Token.Glyph.missing, "").foregroundStyle(.tertiary)
            } else {
                Self.figure(number, of ?? unit)
            }
            SwiftUI.Group {
                if let spark, spark.count > 1, !waiting {
                    Spark(Self.scaled(spark), height: Token.Tile.spark)
                } else {
                    Color.clear
                }
            }
            .frame(height: Token.Tile.spark)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    // value: a figure: the number in a semibold 17 pt, its unit after it
    // in the subheadline, secondary.
    static func figure(_ number: String, _ unit: String) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 3) {
            Text(number).font(.system(size: Token.Tile.figure, weight: .semibold)).monospacedDigit().contentTransition(.numericText())
            if !unit.isEmpty {
                Text(unit).font(.subheadline).foregroundStyle(.secondary)
            }
        }
        .lineLimit(1)
        .minimumScaleFactor(0.8)
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
}

// value: the three measures of a machine, side by side.
struct Metrics: View {
    let measures: [(label: String, value: Value?, spark: [Int]?)]

    var body: some View {
        HStack(alignment: .top, spacing: 14) {
            ForEach(Array(measures.enumerated()), id: \.offset) { _, m in
                Metric(label: m.label, value: m.value, spark: m.spark)
            }
        }
    }
}

// value: a machine's line as the vision's Machines: its tile (its glyph
// in the accent for This Mac, where the owner is), its name and kind, its
// words, its state dot; the line opens its machine's page.
struct MachineRow: View {
    let row: Row
    let symbol: String
    let kinds: Bool
    var sub: String? = nil
    var picked = false
    let open: (String) -> Void
    @Environment(\.stationPalette) private var palette

    var kind: String { row.cells?.first?.text ?? "" }
    var words: String {
        if let sub { return sub }
        let rest = (row.cells ?? []).dropFirst().filter { !$0.isNone }.map { Format.show($0) }
        return ([kind == "Mac" ? kind : nil] + rest.map { Optional($0) }).compactMap { $0 }.joined(separator: Token.Glyph.sep)
    }

    var badge: String? { (kinds || kind == "Server") && kind != "Mac" && !kind.isEmpty ? kind : nil }

    var body: some View {
        Button { open(row.id ?? "") } label: {
            HStack(spacing: Token.Space.gap) {
                IconTile(symbol, active: row.label == Page.ThisMacWord, pulse: row.mark == .pending)
                VStack(alignment: .leading, spacing: 1) {
                    HStack(alignment: .firstTextBaseline, spacing: 5) {
                        Text(row.label ?? "").font(.body).foregroundStyle(.primary).lineLimit(1)
                        if let badge { Text(badge).font(.subheadline).foregroundStyle(.secondary).lineLimit(1) }
                    }
                    if !words.isEmpty {
                        Text(words).font(.subheadline).foregroundStyle(.secondary).lineLimit(1)
                    }
                }
                Spacer(minLength: 4)
                if picked {
                    Image(systemName: Token.Symbol.check).font(.subheadline.weight(.semibold)).foregroundStyle(palette.accent)
                } else {
                    StateDot(mark: row.mark)
                }
            }
            .frame(minHeight: Token.Tile.row)
            .padding(.vertical, 2)
            .stationHover()
        }
        .buttonStyle(.plain)
    }
}

// invariant: SPEC §10.2's Machines as the vision draws them: each Mac,
// then its groups (Workspaces, Servers) named in small secondary words
// and their VMs indented under it; a line opens its machine's page.
struct MachineTree: View {
    let section: Section
    let kinds: Bool
    let only: Set<String>?
    let open: (String) -> Void

    enum Line {
        case caption(String)
        case row(Row)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            ForEach(Array((section.items ?? []).enumerated()), id: \.offset) { _, item in
                if let g = item.group, g.head?.cells?.isEmpty ?? true {
                    nested(lines([item]), indent: false)
                } else if let mac = item.group {
                    if let head = mac.head, only == nil || only!.contains(head.id ?? "") {
                        MachineRow(row: head, symbol: Token.Symbol.mac, kinds: false, open: open)
                    }
                    nested(lines(mac.items ?? []), indent: only == nil)
                } else if let row = item.row, only == nil || only!.contains(row.id ?? "") {
                    MachineRow(row: row, symbol: Token.Symbol.machine(row.cells?.first?.text), kinds: kinds, open: open)
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

    func nested(_ lines: [Line], indent: Bool) -> some View {
        ForEach(Array(lines.enumerated()), id: \.offset) { _, l in
            switch l {
            case let .caption(text):
                Text(text).font(.caption.weight(.semibold)).foregroundStyle(.secondary)
                    .padding(.leading, indent ? Token.Tile.indent : 0).padding(.top, 4).padding(.bottom, 1)
            case let .row(row):
                MachineRow(row: row, symbol: Token.Symbol.machine(row.cells?.first?.text), kinds: kinds, open: open)
                    .padding(.leading, indent ? Token.Tile.indent : 0)
            }
        }
    }
}

// value: a menu line of the panel, as a macOS menu's item: its symbol,
// its words, its key on the right.
struct MenuLine: View {
    let action: Action
    let symbol: String
    let danger: Bool
    let tap: () -> Void

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
            Image(systemName: symbol).font(.system(size: 12, weight: .medium)).frame(width: 18)
                .foregroundStyle(danger ? AnyShapeStyle(palette.bad) : AnyShapeStyle(.secondary))
            Text(title).font(.body).foregroundStyle(danger ? AnyShapeStyle(palette.bad) : AnyShapeStyle(.primary))
            Spacer()
            if let key {
                Text(key).font(.body).foregroundStyle(.tertiary)
            }
        }
        .frame(minHeight: 22)
        .stationHover()
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

enum Symbols {
    static func of(_ a: Action) -> String {
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

extension Section {
    // value: a section's header in the vision's words: its title, a
    // colon cut into a separator, then its caption.
    var header: String {
        let words = (caption ?? []).map { Format.show($0) }
        return ([(title ?? "").replacingOccurrences(of: ": ", with: Token.Glyph.sep)] + words).filter { !$0.isEmpty }.joined(separator: Token.Glyph.sep)
    }
}
