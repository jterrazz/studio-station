import SwiftUI

public enum Connection: Equatable, Sendable {
    case connecting, live, stopped, notInstalled
}

public enum PageID {
    public static let home = "menubar"
    public static let switcher = "menubar.switcher"
    public static let machine = "menubar.machine"

    // value: the watch name of one machine's page (SPEC §10.3).
    public static func of(_ machine: String) -> String { "\(Self.machine):\(machine)" }

    // value: the page a call made now may have changed: the page pushed,
    // else Home.
    public static func shown(_ pushed: String?) -> String { pushed ?? home }
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
    // value: the page pushed over Home: the switcher, or one machine's.
    public var pushed: Page?
    // value: the machines last opened, newest first: the menu bar's own.
    public var recent: [String] = []
    public var search = ""

    public init(page: Page?, connection: Connection, running: String? = nil, confirming: String? = nil, status: Status? = nil, qr: String? = nil, pushed: Page? = nil, recent: [String] = []) {
        self.page = page
        self.connection = connection
        self.running = running
        self.confirming = confirming
        self.status = status
        self.qr = qr
        self.pushed = pushed
        self.recent = recent
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
    public var search: (String) -> Void
    public var copy: (String) -> Void
    // value: navigation: open a machine's page, the switcher, a page by its
    // watch name, Activity's window, back to Home.
    public var openMachine: (String) -> Void
    public var openSwitcher: () -> Void
    public var push: (String) -> Void
    public var window: (String) -> Void
    public var back: () -> Void

    public init(run: @escaping (Action, [String: String]) -> Void = { _, _ in }, confirm: @escaping (Action?) -> Void = { _ in },
                answer: @escaping (String, String, String) -> Void = { _, _, _ in }, open: @escaping (URL) -> Void = { _ in },
                toggleQR: @escaping (String) -> Void = { _ in }, toggleMenu: @escaping () -> Void = {}, loginItem: @escaping (Bool) -> Void = { _ in }, loginItemOn: Bool = false,
                sparkline: @escaping (Bool) -> Void = { _ in }, sparklineOn: Bool = false, quit: @escaping () -> Void = {},
                openMachine: @escaping (String) -> Void = { _ in }, openSwitcher: @escaping () -> Void = {}, search: @escaping (String) -> Void = { _ in },
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
        self.openMachine = openMachine
        self.openSwitcher = openSwitcher
        self.search = search
        self.copy = copy
        self.window = window
        self.push = push
        self.back = back
    }
}

public enum TodoBadge {
    public static let section = "needs-you"

    public static func todo(_ page: Page) -> (mark: Mark, n: Int)? {
        guard let s = page.section(section), page.state != "loading" || s.counts?.contains(where: { $0.mark != .pending }) == true else { return nil }
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

    // invariant: the panel is the MenuBarExtra's own glass; nothing in it
    // is glass again (WWDC25: no glass on glass, none in the content).
    public var body: some View {
        if let own = pushedView {
            own
        } else {
            panel
        }
    }

    // value: a page of the menu bar's own (Ports, Awake, Activity) drawn in
    // place of Home while it is pushed.
    var pushedView: AnyView? {
        guard let pushed = state.pushed else { return nil }
        var shown = state
        shown.page = pushed
        return MenuPage.view(shown, handlers: handlers)
    }

    var panel: some View {
        VStack(alignment: .leading, spacing: Token.Tile.section) {
            header
            if state.menu { menu }
            if let shown = state.shown {
                SwiftUI.Group {
                    if shown.id == PageID.switcher {
                        SwitcherContent(page: shown, state: state, handlers: handlers)
                    } else if shown.id == PageID.machine {
                        MachineContent(page: shown, state: state, handlers: handlers)
                    } else {
                        HomeContent(page: shown, state: state, handlers: handlers)
                    }
                }
                .opacity(state.connection == .connecting ? 0.5 : 1)
                .disabled(state.connection == .connecting)
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
        .stationPanel()
        .environment(\.stationHints, false)
    }

    // invariant: Home and the switcher lead with the machine switcher,
    // This Mac's, its host's name and the settings; a machine's page leads
    // with the way back (its own header).
    @ViewBuilder
    var header: some View {
        if let pushed = state.pushed, pushed.id == PageID.machine {
            EmptyView()
        } else {
            HStack(alignment: .center, spacing: 8) {
                Button(action: state.pushed == nil ? handlers.openSwitcher : handlers.back) {
                    SwitcherPill(open: state.pushed != nil)
                }
                .buttonStyle(.plain)
                .keyboardShortcut("0", modifiers: .command)
                if state.pushed == nil {
                    Text(state.page?.context?.machine ?? "station").font(.system(size: Token.Size.sub)).foregroundStyle(.secondary).lineLimit(1).truncationMode(.middle)
                }
                Spacer(minLength: 4)
                if state.pushed == nil {
                    PlainIcon(Token.Symbol.settings, picked: state.menu, round: true, tap: handlers.toggleMenu)
                }
            }
            .padding(.horizontal, 2).padding(.top, 2)
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
            TextAction("Quit") { handlers.quit() }
        }
    }
}

extension Page {
    static let ThisMacWord = "This Mac"
}
