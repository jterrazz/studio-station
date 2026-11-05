import SwiftUI

// invariant: SPEC §10.2 Awake, vision panel e: This Mac first, its term
// and its Keep awake for, the switches its capabilities allow, its guards;
// then the other Macs by their mode, each with its Change, whose y/n sheet
// opens in its row.
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
        VStack(alignment: .leading, spacing: Token.Space.gap + 2) {
            PushedHeader(back: "This Mac", title: "Awake", subtitle: "Every Mac\(Token.Glyph.sep)this one first",
                         symbol: on ? Token.Symbol.awakeOn : Token.Symbol.awakeOff, tint: on ? palette.warn : nil, id: Navigation.awake, action: handlers.back)
            if let here, let row = here.rows.first { thisMac(here, row) }
            if let guards = page.section("guards") { guardsCard(guards) }
            if let others = page.section("other-macs") {
                GroupCard(others.title ?? "") {
                    ForEach(Array(others.rows.enumerated()), id: \.offset) { i, r in other(others, r, rule: i > 0) }
                }
                ForEach(Array(others.notes.enumerated()), id: \.offset) { _, n in
                    Text(n).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).padding(.horizontal, 4)
                }
            }
        }
    }

    func words(_ v: Value?) -> String { Format.show(v ?? Value()) }

    @ViewBuilder
    func thisMac(_ s: Section, _ r: Row) -> some View {
        let actions = page.actions(for: r.id ?? "")
        let keeps = actions.filter { $0.id?.hasPrefix("keep") == true }
        let switches = actions.filter { $0.id == "lid" || $0.id == "battery" }
        GlassCard {
            HStack(spacing: Token.Space.gap + 2) {
                IconCircle(r.mark == .ok ? Token.Symbol.awakeOn : Token.Symbol.awakeOff, tint: r.mark == .ok ? palette.warn : nil)
                VStack(alignment: .leading, spacing: 1) {
                    Text(r.short ?? r.label ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
                    Text(words(r.cells?.first)).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).lineLimit(2)
                }
                Spacer(minLength: 0)
                if let off = actions.first(where: { $0.id == "let-sleep" }) { Pill(off.label ?? "") { act.tap(off) } }
            }
        }
        if !keeps.isEmpty {
            GlassCard(spacing: 6) {
                CardHead("Keep awake for\(Token.Glyph.ellipsis)")
                if let running = keeps.first(where: { state.running == $0.id }) {
                    HStack(spacing: 6) { PendingDot(); Text(act.label(running)).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted) }
                } else {
                    Segments(options: keeps.map { k in (act.label(k), false, { act.tap(k) }) })
                }
                ForEach(Array(keeps.enumerated()), id: \.offset) { _, k in act.sheet(k) }
            }
        }
        if !switches.isEmpty {
            GlassCard(padding: 12, spacing: 0) {
                ForEach(Array(switches.enumerated()), id: \.offset) { i, a in toggle(a, rule: i > 0) }
            }
        }
    }

    // invariant: a switch's action flips it, so its input holds the other
    // side: Lid closed is on when its action sends Lid false.
    func on(_ a: Action) -> Bool {
        let key = a.id == "lid" ? "Lid" : "Battery"
        return a.run?.in?[key] == "false"
    }

    func toggle(_ a: Action, rule: Bool) -> some View {
        let lid = a.id == "lid"
        let on = on(a)
        return Button { act.tap(a) } label: {
            HStack(spacing: Token.Space.gap + 2) {
                IconCircle(lid ? Token.Symbol.lid : Token.Symbol.battery, tint: on ? (lid ? palette.indigo : palette.good) : nil)
                VStack(alignment: .leading, spacing: 1) {
                    Text(a.label ?? "").font(.system(size: Token.Space.text, weight: .medium)).foregroundStyle(palette.text)
                    Text(lid ? "Stays awake with the lid shut" : "Stays awake when unplugged").font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
                }
                Spacer(minLength: 0)
                if state.running == a.id { PendingDot() } else { Switch(on: on) }
            }
            .frame(minHeight: Token.Tile.rowHeight)
            .contentShape(Rectangle())
            .overlay(alignment: .top) { if rule { Rule() } }
        }
        .buttonStyle(.plain)
        .disabled(state.running != nil)
    }

    func symbol(_ key: String) -> String {
        switch key {
        case "Temperature": Token.Symbol.thermo
        case "Battery": Token.Symbol.battery
        default: Token.Symbol.screen
        }
    }

    @ViewBuilder
    func guardsCard(_ s: Section) -> some View {
        GroupCard(s.title ?? "") {
            ForEach(Array((s.items ?? []).compactMap(\.fact).enumerated()), id: \.offset) { i, f in
                HStack(spacing: Token.Space.gap + 2) {
                    IconCircle(symbol(f.key ?? ""), tint: palette.good, size: Token.Tile.smallCircle)
                    Text(f.key ?? "").font(.system(size: Token.Space.text, weight: .medium)).foregroundStyle(palette.text).layoutPriority(1)
                    Spacer(minLength: Token.Space.gap)
                    Text(words(f.value)).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).lineLimit(1).monospacedDigit()
                }
                .frame(minHeight: Token.Tile.rowHeight - 6)
                .overlay(alignment: .top) { if i > 0 { Rule() } }
            }
        }
        ForEach(Array(s.notes.enumerated()), id: \.offset) { _, n in
            Text(n).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).padding(.horizontal, 4)
        }
    }

    func other(_ s: Section, _ r: Row, rule: Bool) -> some View {
        let change = page.actions(for: r.id ?? "").first { $0.id?.hasPrefix("change:") == true }
        return VStack(alignment: .leading, spacing: 6) {
            HStack(alignment: .top, spacing: Token.Space.gap + 2) {
                IconTile(Token.Symbol.mac, tint: r.mark == .absent ? nil : palette.accent)
                VStack(alignment: .leading, spacing: 1) {
                    Text(r.short ?? r.label ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
                    Text(words(r.cells?.first)).font(.system(size: Token.Space.small)).foregroundStyle(palette.text)
                    if let detail = r.detail { Text(words(detail)).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).lineLimit(2) }
                }
                Spacer(minLength: 0)
                if let change { Pill("\(change.label ?? "")\(Token.Glyph.ellipsis)") { act.tap(change) } }
            }
            .padding(.vertical, 4)
            if let change { act.sheet(change) }
        }
        .overlay(alignment: .top) { if rule { Rule() } }
    }
}
