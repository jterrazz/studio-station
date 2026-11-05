import AppKit
import Foundation
import StationKit
import UserNotifications

@MainActor
final class Model: ObservableObject {
    @Published var state = PopoverState(page: nil, connection: .connecting)
    @Published var loginItemOn = LoginItem.on
    @Published var sparkline = UserDefaults.standard.bool(forKey: Model.sparklineKey)

    // value: the menu bar's own display option, in its macOS user defaults,
    // never in station's preferences.
    static let sparklineKey = "cpuSparkline"
    // value: the machines last opened, the switcher's Recent: the menu bar's
    // own, in its macOS user defaults.
    static let recentKey = "recentMachines"
    static let recentKept = 3

    let socket = EnginePath.socket()
    private var lastPage: Date?
    private var subscription: Task<Void, Never>?
    private var events: Task<Void, Never>?
    private var clearing: Task<Void, Never>?
    private var watching: Task<Void, Never>?
    private var pushing: Task<Void, Never>?
    private var asked = false

    var icon: MenuBarIcon { MenuBarIcon.of(connection: state.connection, page: state.page, sparkline: sparkline) }

    func iconImage(dark: Bool) -> NSImage {
        let icon = self.icon
        guard let cg = icon.image(scheme: dark ? .dark : .light) else { return NSImage() }
        let image = NSImage(cgImage: cg, size: NSSize(width: CGFloat(cg.width) / 2, height: CGFloat(cg.height) / 2))
        image.isTemplate = icon.template
        image.accessibilityDescription = icon.label
        return image
    }

    func start() {
        guard subscription == nil else { return }
        let sub = Subscription(path: socket) { event in
            Task { @MainActor in self.receive(event) }
        }
        subscription = Task.detached { await sub.run() }
        let told = Subscription(path: socket, watch: Notifications.watch) { event in
            guard let text = Notifications.posted(event, from: Notifications.watch) else { return }
            Task { @MainActor in self.notify(text) }
        }
        events = Task.detached { await told.run() }
    }

    // invariant: a pushed page is watched only while it is shown: opening
    // another or going back ends its watch (SPEC §10.3).
    func push(_ page: String) {
        pushing?.cancel()
        state.confirming = nil
        let sub = Subscription(path: socket, pages: [page]) { event in
            guard case .page(let p) = event else { return }
            Task { @MainActor in self.state.pushed = p }
        }
        pushing = Task.detached { await sub.run() }
    }

    func openMachine(_ name: String) {
        state.recent = Array(([name] + state.recent.filter { $0 != name }).prefix(Self.recentKept))
        UserDefaults.standard.set(state.recent, forKey: Self.recentKey)
        push(PageID.of(name))
    }

    func openSwitcher() {
        state.recent = UserDefaults.standard.stringArray(forKey: Self.recentKey) ?? []
        push(PageID.switcher)
    }

    func back() {
        pushing?.cancel()
        pushing = nil
        state.pushed = nil
        state.search = ""
        state.confirming = nil
    }

    func setSparkline(_ on: Bool) {
        UserDefaults.standard.set(on, forKey: Self.sparklineKey)
        sparkline = on
    }

    func receive(_ event: StreamEvent) {
        switch event {
        case .connected:
            state.connection = .live
        case .page(let page):
            state.connection = .live
            state.page = page
            lastPage = Date()
        case .tick:
            break
        case .notice(_, let text):
            state.status = Status(text, failed: false)
        case .down:
            if FileManager.default.fileExists(atPath: socket) && state.page != nil && state.connection == .live {
                state.connection = .connecting
            } else {
                showStopped()
            }
        }
    }

    func showStopped() {
        let installed = FileManager.default.fileExists(atPath: EnginePath.agentPlist())
        state.connection = installed ? .stopped : .notInstalled
        guard let url = Bundle.main.url(forResource: "stopped", withExtension: "json"),
              let data = try? Data(contentsOf: url), var page = try? Page.decode(data) else { return }
        page.context = Context(machine: Host.current().localizedName, kind: page.context?.kind)
        if !installed {
            page = notInstalled(page)
        }
        state.page = page
    }

    func notInstalled(_ page: Page) -> Page {
        var p = page
        p.actions = p.actions?.filter { $0.id == "open-station" }
        if var s = p.sections?.first, var row = s.items?.first?.row {
            row.cells = [Value(kind: "text", text: "not installed")]
            row.hint = Hint(command: "station setup")
            s.items = [Item(row: row)]
            p.sections = [s]
        }
        return p
    }

    func opened() {
        let socket = self.socket
        watching?.cancel()
        watching = Task.detached { await Client.hold(path: socket, watch: "engine.watch") }
        if let last = lastPage, Date().timeIntervalSince(last) > 60 {
            Task { _ = await Client.call(path: socket, name: "engine.refresh", input: ["pages": ["menubar"]]) }
        }
    }

    func closed() {
        watching?.cancel()
        watching = nil
        back()
    }

    func run(_ action: Action, answers: [String: String]) {
        state.running = action.id
        state.status = nil
        let socket = self.socket
        Task {
            let executable = await Client.executable(path: socket) ?? Self.fallbackExecutable
            let status = await ActionRunner(executable: executable, socket: socket).run(action, answers: answers)
            state.running = nil
            state.status = status
            if let text = status?.posted { notify(text) }
            clearing?.cancel()
            if status != nil {
                clearing = Task {
                    try? await Task.sleep(nanoseconds: 30_000_000_000)
                    if !Task.isCancelled { state.status = nil }
                }
            }
        }
    }

    static var fallbackExecutable: String { NSHomeDirectory() + "/.local/bin/station" }

    func setLoginItem(_ on: Bool) {
        do {
            try LoginItem.set(on)
        } catch {
            state.status = Status(error.localizedDescription, failed: true)
        }
        loginItemOn = LoginItem.on
    }

    func notify(_ text: String) {
        let center = UNUserNotificationCenter.current()
        let post = {
            let content = UNMutableNotificationContent()
            content.title = "station"
            content.body = text
            center.add(UNNotificationRequest(identifier: UUID().uuidString, content: content, trigger: nil))
        }
        if asked {
            post()
            return
        }
        asked = true
        center.requestAuthorization(options: [.alert]) { granted, _ in
            if granted { post() }
        }
    }
}
