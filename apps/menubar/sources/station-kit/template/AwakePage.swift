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
        let on = here?.rows.first?.mark == .ok
        VStack(alignment: .leading, spacing: Token.Tile.section) {
            PageHeader(back: Page.ThisMacWord, title: "Awake", subtitle: "Every Mac\(Token.Glyph.sep)this one first",
                       symbol: on ? Token.Symbol.awakeOn : Token.Symbol.awakeOff, active: on, action: handlers.back)
            if let here, let row = here.rows.first { thisMac(here, row) }
            if let guards = page.section("guards") { guardsSection(guards) }
            if let others = page.section("other-macs") {
                VStack(alignment: .leading, spacing: 4) {
                    SectionHeader(others.title ?? "")
                    ForEach(Array(others.rows.enumerated()), id: \.offset) { _, r in other(r) }
                    ForEach(Array(others.notes.enumerated()), id: \.offset) { _, n in FootNote(n).padding(.top, 4) }
                }
            }
        }
    }

    func words(_ v: Value?) -> String { Format.show(v ?? Value()) }

    // value: This Mac: Keep awake and its switch, how long as a segmented
    // choice, then Lid closed and On battery with their switches.
    func thisMac(_ s: Section, _ r: Row) -> some View {
        let actions = page.actions(for: r.id ?? "")
        let keeps = actions.filter { $0.id?.hasPrefix("keep") == true }
        let switches = actions.filter { $0.id == "lid" || $0.id == "battery" }
        let on = r.mark == .ok
        let flip = on ? actions.first { $0.id == "let-sleep" } : keeps.first
        return VStack(alignment: .leading, spacing: 4) {
            SectionHeader(s.title ?? Page.ThisMacWord)
            MenuRow("Keep awake", subtitle: words(r.cells?.first)) {
                EmptyView()
            } trailing: {
                if let flip {
                    if state.running == flip.id { PendingDot() } else { StationSwitch(on: on) { act.tap(flip) }.disabled(state.running != nil) }
                }
            }
            if !keeps.isEmpty {
                VStack(alignment: .leading, spacing: 5) {
                    Text("Keep awake for\(Token.Glyph.ellipsis)").font(.subheadline).foregroundStyle(.secondary)
                    if let running = keeps.first(where: { state.running == $0.id }) {
                        HStack(spacing: 6) {
                            PendingDot()
                            Text(act.label(running)).font(.subheadline).foregroundStyle(.secondary)
                        }
                    } else {
                        Segments(options: keeps.map { k in (act.label(k), false, { act.tap(k) }) })
                    }
                }
                .padding(.vertical, 4)
                ForEach(Array(keeps.enumerated()), id: \.offset) { _, k in act.sheet(k) }
            }
            ForEach(Array(switches.enumerated()), id: \.offset) { _, a in toggle(a) }
        }
    }

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
            IconCircle(lid ? Token.Symbol.lid : Token.Symbol.battery, active: on)
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

    func guardsSection(_ s: Section) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            SectionHeader(s.title ?? "")
            ForEach(Array((s.items ?? []).compactMap(\.fact).enumerated()), id: \.offset) { _, f in
                let parts = (f.value?.kind == "list" ? f.value?.items : nil)?.map { Format.show($0) }
                if let parts, parts.count > 1 {
                    MenuRow("\(f.key ?? "") \(parts[0])", subtitle: parts.dropFirst().joined(separator: Token.Glyph.sep).capitalizedFirst) {
                        IconCircle(symbol(f.key ?? ""))
                    }
                } else {
                    MenuRow(f.key ?? "") {
                        IconCircle(symbol(f.key ?? ""))
                    } trailing: {
                        TrailingValue(words(f.value).capitalizedFirst)
                    }
                }
            }
            ForEach(Array(s.notes.enumerated()), id: \.offset) { _, n in FootNote(n).padding(.top, 4) }
        }
    }

    func other(_ r: Row) -> some View {
        let change = page.actions(for: r.id ?? "").first { $0.id?.hasPrefix("change:") == true }
        return VStack(alignment: .leading, spacing: 4) {
            MenuRow(r.short ?? r.label ?? "", subtitle: words(r.cells?.first)) {
                IconCircle(Token.Symbol.mac)
            } trailing: {
                StateDot(mark: r.mark)
                if let change { StationButton("\(change.label ?? "")\(Token.Glyph.ellipsis)") { act.tap(change) } }
            }
            if let detail = r.detail { FootNote(words(detail).capitalizedFirst).padding(.leading, Token.Tile.circle + Token.Space.gap) }
            if let change { act.sheet(change) }
        }
    }
}

extension String {
    // value: the words with their first letter capital, as a line of a
    // menu starts.
    var capitalizedFirst: String { prefix(1).uppercased() + dropFirst() }
}
