import AppKit
import Foundation
import StationKit
import UserNotifications

@MainActor
final class Model: ObservableObject {
    @Published var state = PopoverState(page: nil, connection: .connecting)
    @Published var loginItemOn = LoginItem.on

    let socket = EnginePath.socket()
    private var lastPage: Date?
    private var subscription: Task<Void, Never>?
    private var clearing: Task<Void, Never>?
    private var asked = false

    var icon: String { MenuBarIcon.symbol(connection: state.connection, page: state.page) }
    var label: String { MenuBarIcon.label(connection: state.connection, page: state.page) }

    func start() {
        guard subscription == nil else { return }
        let sub = Subscription(path: socket) { event in
            Task { @MainActor in self.receive(event) }
        }
        subscription = Task.detached { await sub.run() }
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
            notify(text)
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
        if let last = lastPage, Date().timeIntervalSince(last) > 60 {
            Task { _ = await Client.call(path: socket, name: "engine.refresh", input: ["pages": ["menubar"]]) }
        }
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
        state.status = Status(text, failed: false)
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
