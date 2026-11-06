import SwiftUI

// invariant: SPEC §10.2 Awake, vision panel e: This Mac first, Keep awake
// and its switch, Keep awake for, the switches its capabilities allow,
// its guards; then the other Macs by their mode, each with its Change,
// whose y/n sheet opens in its row. No grouping: rows on the panel.
public struct AwakePage: View {
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
        let here = page.section("this-mac")
        VStack(alignment: .leading, spacing: Token.Tile.section) {
            PageHeader(back: Page.ThisMacWord, title: "Awake", subtitle: "Every Mac\(Token.Glyph.sep)this one first",
                       symbol: Token.Symbol.awake, action: handlers.back)
            if let here, let row = here.rows.first { thisMac(here, row) }
            if let guards = page.section("guards") { guardsSection(guards) }
            if let others = page.section("other-macs") {
                SectionHeader(others.title ?? "")
                Rows {
                    ForEach(Array(others.rows.enumerated()), id: \.offset) { _, r in other(r) }
                }
                ForEach(Array(others.notes.enumerated()), id: \.offset) { _, n in FootNote(n) }
            }
        }
    }

    func words(_ v: Value?) -> String { Format.show(v ?? Value()) }

    // value: This Mac: Keep awake and its switch on a tile, how long as a
    // segmented choice on another, then Lid closed and On battery with
    // their switches in a list.
    @ViewBuilder
    func thisMac(_ s: Section, _ r: Row) -> some View {
        let actions = page.actions(for: r.id ?? "")
        let keeps = actions.filter { $0.id?.hasPrefix("keep") == true }
        let switches = actions.filter { $0.id == "lid" || $0.id == "battery" }
        let on = r.mark == .ok
        let flip = on ? actions.first { $0.id == "let-sleep" } : keeps.first
        SectionHeader(s.title ?? Page.ThisMacWord)
        MenuRow("Keep awake", subtitle: words(r.cells?.first), bare: true) {
            EmptyView()
        } trailing: {
            if let flip {
                if state.running == flip.id { PendingDot() } else { StationSwitch(on: on) { act.tap(flip) }.disabled(state.running != nil) }
            }
        }
        .stationTile()
        if !keeps.isEmpty {
            VStack(alignment: .leading, spacing: 8) {
                SubText("Keep awake for\(Token.Glyph.ellipsis)")
                if let running = keeps.first(where: { state.running == $0.id }) {
                    HStack(spacing: 6) {
                        PendingDot()
                        SubText(act.label(running))
                    }
                } else {
                    Segments(options: keeps.map { k in (Self.held(k) ? k.label ?? "" : act.label(k), on && Self.held(k), { act.tap(k) }) })
                }
            }
            .stationTile()
            ForEach(Array(keeps.enumerated()), id: \.offset) { _, k in act.sheet(k) }
        }
        if !switches.isEmpty {
            Rows {
                ForEach(Array(switches.enumerated()), id: \.offset) { _, a in toggle(a) }
            }
        }
    }

    // value: Until while a keep-awake holds: it says the time it holds
    // until (`Until 18:00`), the term chosen.
    static func held(_ k: Action) -> Bool { k.form != nil && (k.label ?? "").contains(where: \.isNumber) }

    // invariant: a switch's action flips it, so its input holds the other
    // side: Lid closed is on when its action sends Lid false.
    func on(_ a: Action) -> Bool {
        let key = a.id == "lid" ? "Lid" : "Battery"
        return a.run?.in?[key] == "false"
    }

    func toggle(_ a: Action) -> some View {
        let lid = a.id == "lid"
        let on = on(a)
        return MenuRow(a.label ?? "", subtitle: lid ? "Stays awake with the lid shut" : "Stays awake when unplugged") {
            IconTile(lid ? Token.Symbol.lid : Token.Symbol.battery, active: on)
        } trailing: {
            if state.running == a.id { PendingDot() } else { StationSwitch(on: on) { act.tap(a) }.disabled(state.running != nil) }
        }
    }

    func symbol(_ key: String) -> String {
        switch key {
        case "Temperature": Token.Symbol.thermo
        case "Battery": Token.Symbol.battery
        default: Token.Symbol.screen
        }
    }

    @ViewBuilder
    func guardsSection(_ s: Section) -> some View {
        SectionHeader(s.title ?? "")
        Rows {
            ForEach(Array((s.items ?? []).compactMap(\.fact).enumerated()), id: \.offset) { _, f in
                let parts = (f.value?.kind == "list" ? f.value?.items : nil)?.map { Format.show($0) }
                if let parts, parts.count > 1 {
                    MenuRow("\(f.key ?? "") \(parts[0])", subtitle: parts.dropFirst().joined(separator: Token.Glyph.sep).capitalizedFirst) {
                        IconTile(symbol(f.key ?? ""))
                    }
                } else {
                    MenuRow(f.key ?? "") {
                        IconTile(symbol(f.key ?? ""))
                    } trailing: {
                        TrailingValue(words(f.value).capitalizedFirst)
                    }
                }
            }
        }
        ForEach(Array(s.notes.enumerated()), id: \.offset) { _, n in FootNote(n) }
    }

    func other(_ r: Row) -> some View {
        let change = page.actions(for: r.id ?? "").first { $0.id?.hasPrefix("change:") == true }
        return VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: Token.Tile.gap) {
                IconTile(Token.Symbol.mac)
                RowText(title: r.short ?? r.label ?? "", subtitle: words(r.cells?.first))
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .layoutPriority(1)
                StateDot(mark: r.mark)
                if let change { StationButton("\(change.label ?? "")\(Token.Glyph.ellipsis)") { act.tap(change) } }
            }
            if let detail = r.detail { SubText(words(detail).capitalizedFirst) }
            if let change { act.sheet(change) }
        }
        .padding(EdgeInsets(top: 10, leading: 12, bottom: 10, trailing: 12))
    }
}

extension String {
    // value: the words with their first letter capital, as a line of a
    // menu starts.
    var capitalizedFirst: String { prefix(1).uppercased() + dropFirst() }
}
