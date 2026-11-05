import SwiftUI

// value: the menu bar's own pages of SPEC §10.2 a page id names; any other
// page draws as PageView.
public enum MenuPage {
    public static let ports = Navigation.ports
    public static let vm = Navigation.vm
    public static let awake = Navigation.awake
    public static let activity = Navigation.activity

    @MainActor
    public static func view(_ state: PopoverState, handlers: PopoverHandlers = PopoverHandlers()) -> AnyView? {
        guard let page = state.page else { return nil }
        let body: AnyView
        switch page.id {
        case ports, vm: body = AnyView(PortsPage(state, handlers: handlers))
        case awake: body = AnyView(AwakePage(state, handlers: handlers))
        case activity: body = AnyView(ActivityPage(state, handlers: handlers))
        default: return nil
        }
        return AnyView(MenuPageFrame(state: state) { body })
    }
}

struct MenuPageFrame<Content: View>: View {
    let state: PopoverState
    let content: Content
    @Environment(\.stationPalette) private var palette

    init(state: PopoverState, @ViewBuilder content: () -> Content) {
        self.state = state
        self.content = content()
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Token.Space.gap + 2) {
            content
            if let status = state.status { StatusBar(status.line, failed: status.failed) }
        }
        .padding(Token.Space.margin)
        .frame(width: Token.Space.width, alignment: .topLeading)
        .background(RoundedRectangle(cornerRadius: 24, style: .continuous).fill(palette.surface))
        .environment(\.stationHints, false)
    }
}

extension Page {
    // value: the actions a row offers, in their priority.
    public func actions(for row: String) -> [Action] {
        (actions ?? []).filter { $0.on == "row" && $0.rows?.contains(row) == true && $0.run != nil }.sorted { ($0.priority ?? 0) < ($1.priority ?? 0) }
    }

    public func summary() -> Row? { section("summary")?.items?.first?.row }
}

extension Section {
    // value: a row's cell under the column a key names; the first column
    // is the row's label.
    public func cell(_ row: Row, _ key: String) -> Value? {
        guard let i = columns?.firstIndex(where: { $0.key == key }), i > 0, let cells = row.cells, i - 1 < cells.count else { return nil }
        let v = cells[i - 1]
        return v.isNone ? nil : v
    }

    public var rows: [Row] { (items ?? []).compactMap(\.row) }
    public var notes: [String] { (items ?? []).compactMap { $0.note?.text } }
}

// invariant: an action is run as SPEC §10.2 says: a confirmation or a form
// opens its sheet first, any other runs at once.
struct Act {
    let state: PopoverState
    let handlers: PopoverHandlers

    func tap(_ action: Action) {
        if action.confirm != nil || action.form != nil {
            handlers.confirm(action)
        } else {
            handlers.run(action, state.answers[action.id ?? ""] ?? [:])
        }
    }

    func open(_ action: Action) -> Bool { state.confirming == action.id }

    @ViewBuilder
    func sheet(_ action: Action) -> some View {
        if open(action), let confirm = action.confirm {
            ConfirmSheet(confirm) { yes in
                handlers.confirm(nil)
                if yes { handlers.run(action, state.answers[action.id ?? ""] ?? [:]) }
            }
        } else if open(action), action.form != nil {
            FormSheet(action, answers: state.answers[action.id ?? ""] ?? [:], answer: { k, v in handlers.answer(action.id ?? "", k, v) }) { yes in
                handlers.confirm(nil)
                if yes { handlers.run(action, state.answers[action.id ?? ""] ?? [:]) }
            }
        }
    }

    func label(_ action: Action) -> String {
        (action.label ?? "") + (action.form != nil ? Token.Glyph.ellipsis : "")
    }
}
