import SwiftUI

public enum Connection: Equatable, Sendable {
    case connecting, live, stopped, notInstalled
}

public struct PopoverState: Equatable, Sendable {
    public var page: Page?
    public var connection: Connection
    public var running: String?
    public var confirming: String?
    public var status: Status?
    public var qr: String?
    public var menu = false
    public var answers: [String: [String: String]] = [:]
    // value: the menu bar's own page pushed over Home: Ports, Awake or
    // Activity, as the engine streams it.
    public var pushed: Page?

    public init(page: Page?, connection: Connection, running: String? = nil, confirming: String? = nil, status: Status? = nil, qr: String? = nil, pushed: Page? = nil) {
        self.page = page
        self.connection = connection
        self.running = running
        self.confirming = confirming
        self.status = status
        self.qr = qr
        self.pushed = pushed
    }

    // value: the page the owner sees, the pushed one over Home.
    public var shown: Page? { pushed ?? page }
}

public struct PopoverHandlers {
    public var run: (Action, [String: String]) -> Void
    public var confirm: (Action?) -> Void
    public var answer: (String, String, String) -> Void
    public var open: (URL) -> Void
    public var toggleQR: (String) -> Void
    public var toggleMenu: () -> Void
    public var loginItem: (Bool) -> Void
    public var loginItemOn: Bool
    public var sparkline: (Bool) -> Void
    public var sparklineOn: Bool
    public var quit: () -> Void
    public var copy: (String) -> Void
    public var window: (String) -> Void
    // value: navigation: open one of the menu bar's own pages by its watch
    // name, back to Home.
    public var push: (String) -> Void
    public var back: () -> Void

    public init(run: @escaping (Action, [String: String]) -> Void = { _, _ in }, confirm: @escaping (Action?) -> Void = { _ in },
                answer: @escaping (String, String, String) -> Void = { _, _, _ in }, open: @escaping (URL) -> Void = { _ in },
                toggleQR: @escaping (String) -> Void = { _ in }, toggleMenu: @escaping () -> Void = {}, loginItem: @escaping (Bool) -> Void = { _ in }, loginItemOn: Bool = false,
                sparkline: @escaping (Bool) -> Void = { _ in }, sparklineOn: Bool = false, quit: @escaping () -> Void = {},
                copy: @escaping (String) -> Void = { _ in }, window: @escaping (String) -> Void = { _ in },
                push: @escaping (String) -> Void = { _ in }, back: @escaping () -> Void = {}) {
        self.run = run
        self.confirm = confirm
        self.answer = answer
        self.open = open
        self.toggleQR = toggleQR
        self.toggleMenu = toggleMenu
        self.loginItem = loginItem
        self.loginItemOn = loginItemOn
        self.sparkline = sparkline
        self.sparklineOn = sparklineOn
        self.quit = quit
        self.copy = copy
        self.window = window
        self.push = push
        self.back = back
    }
}

public enum TodoBadge {
    public static func todo(_ page: Page) -> (mark: Mark, n: Int)? {
        guard let s = page.section("todo"), page.state != "loading" || s.counts?.contains(where: { $0.mark != .pending }) == true else { return nil }
        let counts = (s.counts ?? []).filter { $0.mark != .pending && $0.mark != .ok }
        let n = counts.reduce(0) { $0 + ($1.n ?? 0) }
        let worst = counts.compactMap(\.mark).max { $0.rank < $1.rank } ?? .ok
        return (worst, n)
    }

    public static func text(_ page: Page) -> String? {
        guard let (_, n) = todo(page) else { return nil }
        return n == 0 ? "all good" : "\(n) to do"
    }
}

public struct Popover: View {
    let state: PopoverState
    let handlers: PopoverHandlers
    let scrolls: Bool
    @Environment(\.stationPalette) private var palette

    public init(_ state: PopoverState, handlers: PopoverHandlers = PopoverHandlers(), scrolls: Bool = true) {
        self.state = state
        self.handlers = handlers
        self.scrolls = scrolls
    }

    // value: the menu bar's own pages Home opens: This Mac's Activity over
    // 1 h, every Mac's Awake, every Mac's Ports.
    public static let pages: [(label: String, watch: String)] = [
        ("Activity", Navigation.activity), ("Awake", Navigation.awake), ("Ports", Navigation.ports),
    ]

    var inner: CGFloat { Token.Space.width - Token.Space.margin * 2 }

    var links: LinkActions { LinkActions(open: handlers.open, qr: state.qr, toggleQR: handlers.toggleQR, push: handlers.push) }

    public var body: some View {
        if let own = pushedView {
            own
        } else {
            home
        }
    }

    var pushedView: AnyView? {
        guard let pushed = state.pushed else { return nil }
        var shown = state
        shown.page = pushed
        return MenuPage.view(shown, handlers: handlers)
    }

    var home: some View {
        VStack(alignment: .leading, spacing: Token.Space.gap) {
            header
            if state.menu { menu }
            if let page = state.page {
                content(page)
            } else {
                PendingDot(still: true)
            }
            if state.connection == .connecting {
                StatusBar("connecting\(Token.Glyph.ellipsis)", failed: false)
            } else if let status = state.status {
                StatusBar(status.line, failed: status.failed)
            }
        }
        .padding(Token.Space.margin)
        .frame(width: Token.Space.width, alignment: .topLeading)
        .background(palette.surface)
        .environment(\.stationHints, false)
    }

    var header: some View {
        HStack(spacing: Token.Space.gap / 2) {
            Text(state.page?.context?.machine ?? "station").font(.system(size: Token.Space.text + 1, weight: .bold)).foregroundStyle(palette.text)
            Spacer()
            if let page = state.page, let (mark, _) = TodoBadge.todo(page), let text = TodoBadge.text(page) {
                MarkView(mark)
                Text(text).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
            }
            Button(action: handlers.toggleMenu) {
                Image(systemName: Token.Symbol.more).font(.system(size: Token.Space.text)).foregroundStyle(palette.muted)
            }
            .buttonStyle(.plain)
        }
    }

    // invariant: the footer menu is drawn inline, never an AppKit menu: the
    // switches for launch at login and the icon's CPU line, and quit.
    var menu: some View {
        HStack(spacing: Token.Space.gap) {
            Toggle("Open at login", isOn: Binding(get: { handlers.loginItemOn }, set: { handlers.loginItem($0) }))
                .toggleStyle(.checkbox)
                .font(.system(size: Token.Space.small))
            Toggle("CPU in the menu bar", isOn: Binding(get: { handlers.sparklineOn }, set: { handlers.sparkline($0) }))
                .toggleStyle(.checkbox)
                .font(.system(size: Token.Space.small))
            Spacer()
            Button("Quit") { handlers.quit() }.font(.system(size: Token.Space.small))
        }
    }

    @ViewBuilder
    func content(_ page: Page) -> some View {
        let dimmed = state.connection == .connecting
        VStack(alignment: .leading, spacing: Token.Space.gap) {
            if let strip = page.strip { StripView(strip) }
            if let todo = page.section("todo") {
                Divider()
                SectionView(todo, width: inner)
            }
            if let fleet = page.section("fleet") {
                Divider()
                if scrolls {
                    ScrollView { SectionView(fleet, width: inner, links: links) }.frame(maxHeight: Token.Space.cap / 2)
                } else {
                    SectionView(fleet, width: inner, links: links)
                }
            }
            if let power = page.section("power") {
                Divider()
                powerSection(page, power)
            }
            Divider()
            if state.connection == .live || state.connection == .connecting {
                HStack(spacing: 4) {
                    ForEach(Array(Self.pages.enumerated()), id: \.offset) { _, p in
                        Pill(p.label) { handlers.push(p.watch) }
                    }
                    Spacer(minLength: 0)
                }
            }
            actions(page)
        }
        .opacity(dimmed ? 0.5 : 1)
        .disabled(dimmed)
    }

    func powerSection(_ page: Page, _ power: Section) -> some View {
        let toggle = (page.actions ?? []).first { $0.rows?.contains("awake") == true && $0.run != nil }
        let held = power.role == "banner"
        let title = held ? "\(Token.Glyph.power) \(power.title ?? "")" : power.title
        var shown = power
        shown.caption = nil
        return VStack(alignment: .leading, spacing: 4) {
            HStack {
                SectionView(shown, width: inner - 40, title: title)
                if let toggle {
                    if state.running == toggle.id {
                        PendingDot()
                    } else {
                        Button { handlers.run(toggle, state.answers[toggle.id ?? ""] ?? [:]) } label: { Switch(on: held) }
                            .buttonStyle(.plain)
                            .disabled(state.running != nil)
                    }
                }
            }
            if let form = toggle?.form, !held {
                FormView(form, answers: Binding(get: { state.answers[toggle?.id ?? ""] ?? [:] }, set: { new in
                    for (k, v) in new { handlers.answer(toggle?.id ?? "", k, v) }
                }))
            }
        }
    }

    @ViewBuilder
    func actions(_ page: Page) -> some View {
        if let id = state.confirming, let action = page.action(id), let confirm = action.confirm {
            ConfirmPanel(confirm) { yes in
                handlers.confirm(nil)
                if yes { handlers.run(action, [:]) }
            }
        } else {
            VStack(alignment: .leading, spacing: 4) {
                if let form = page.pageActions.first(where: { $0.form != nil && state.confirming == $0.id })?.form {
                    FormView(form, answers: .constant([:]))
                }
                HStack(spacing: Token.Space.gap) {
                    ForEach(Array(page.pageActions.enumerated()), id: \.offset) { _, action in
                        if state.running == action.id {
                            PendingDot()
                        } else {
                            Button(action.form != nil ? "\(action.label ?? "")\(Token.Glyph.ellipsis)" : action.label ?? "") {
                                if action.confirm != nil || action.form != nil {
                                    handlers.confirm(action)
                                } else {
                                    handlers.run(action, [:])
                                }
                            }
                            .font(.system(size: Token.Space.small))
                            .controlSize(.small)
                            .disabled(state.running != nil)
                        }
                    }
                    Spacer(minLength: 0)
                }
            }
        }
    }
}
