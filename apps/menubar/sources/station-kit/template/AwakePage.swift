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
        VStack(alignment: .leading, spacing: Token.Space.gap + 2) {
            PushedHeader(back: "This Mac", title: "Awake", subtitle: "Every Mac\(Token.Glyph.sep)this one first", action: handlers.back)
            if let here = page.section("this-mac"), let row = here.rows.first { thisMac(here, row) }
            if let guards = page.section("guards") { guardsCard(guards) }
            if let others = page.section("other-macs") {
                SectionHead(others.title ?? "")
                GlassCard {
                    ForEach(Array(others.rows.enumerated()), id: \.offset) { _, r in other(others, r) }
                }
                ForEach(Array(others.notes.enumerated()), id: \.offset) { _, n in
                    Text(n).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).padding(.horizontal, 4)
                }
            }
        }
    }

    func words(_ v: Value?) -> String { Format.show(v ?? Value()) }

    func thisMac(_ s: Section, _ r: Row) -> some View {
        let actions = page.actions(for: r.id ?? "")
        let keeps = actions.filter { $0.id?.hasPrefix("keep") == true }
        return GlassCard {
            HStack(spacing: Token.Space.gap) {
                Image(systemName: Token.Symbol.sun).foregroundStyle(r.mark == .ok ? palette.warn : palette.muted)
                VStack(alignment: .leading, spacing: 1) {
                    Text(r.short ?? r.label ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
                    Text(words(r.cells?.first)).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).lineLimit(2)
                }
                Spacer(minLength: 0)
                if let off = actions.first(where: { $0.id == "let-sleep" }) { Pill(off.label ?? "") { act.tap(off) } }
            }
            HStack(spacing: 4) {
                Text("Keep awake for\(Token.Glyph.ellipsis)").font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
                ForEach(Array(keeps.enumerated()), id: \.offset) { _, k in
                    if state.running == k.id { PendingDot() } else { Pill(act.label(k)) { act.tap(k) } }
                }
            }
            ForEach(Array(keeps.enumerated()), id: \.offset) { _, k in act.sheet(k) }
            ForEach(Array(actions.filter { $0.id == "lid" || $0.id == "battery" }.enumerated()), id: \.offset) { _, a in toggle(a) }
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
        return Button { act.tap(a) } label: {
            HStack(spacing: Token.Space.gap) {
                Image(systemName: lid ? Token.Symbol.lid : Token.Symbol.battery).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).frame(width: 16)
                VStack(alignment: .leading, spacing: 1) {
                    Text(a.label ?? "").font(.system(size: Token.Space.text)).foregroundStyle(palette.text)
                    Text(lid ? "Stays awake with the lid shut" : "Stays awake when unplugged").font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
                }
                Spacer(minLength: 0)
                if state.running == a.id { PendingDot() } else { Switch(on: on(a)) }
            }
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

    func guardsCard(_ s: Section) -> some View {
        GlassCard {
            Text(s.title ?? "").font(.system(size: Token.Space.small, weight: .semibold)).foregroundStyle(palette.muted)
            ForEach(Array((s.items ?? []).compactMap(\.fact).enumerated()), id: \.offset) { _, f in
                HStack(spacing: Token.Space.gap) {
                    Image(systemName: symbol(f.key ?? "")).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).frame(width: 16)
                    Text(f.key ?? "").font(.system(size: Token.Space.text)).foregroundStyle(palette.text)
                    Spacer(minLength: 0)
                    Text(words(f.value)).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).lineLimit(1)
                }
            }
            ForEach(Array(s.notes.enumerated()), id: \.offset) { _, n in
                Text(n).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
            }
        }
    }

    func other(_ s: Section, _ r: Row) -> some View {
        let change = page.actions(for: r.id ?? "").first { $0.id?.hasPrefix("change:") == true }
        return VStack(alignment: .leading, spacing: 6) {
            HStack(alignment: .top, spacing: Token.Space.gap) {
                Image(systemName: Token.Symbol.mac).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).frame(width: 16)
                VStack(alignment: .leading, spacing: 1) {
                    Text(r.short ?? r.label ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
                    Text(words(r.cells?.first)).font(.system(size: Token.Space.small)).foregroundStyle(palette.text)
                    if let detail = r.detail { Text(words(detail)).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).lineLimit(2) }
                }
                Spacer(minLength: 0)
                if let change { Pill("\(change.label ?? "")\(Token.Glyph.ellipsis)") { act.tap(change) } }
            }
            if let change { act.sheet(change) }
        }
    }
}
