import Foundation

// invariant: SPEC §10.3: a page pushed over Home is the engine's watch of
// that page, held only while it is shown: opening another, picking another
// window or going back ends it, and a page of a watch that ended is never
// shown.
public final class Navigation: @unchecked Sendable {
    public static let ports = "menubar.ports"
    public static let awake = "menubar.awake"
    public static let activity = "menubar.activity"

    // value: the watch of a page of one subject: a Mac's ports
    // (`menubar.ports:jterrazz-mac-mini`), the activity over a window
    // (`menubar.activity:6m`).
    public static func page(_ id: String, of subject: String) -> String { "\(id):\(subject)" }

    let socket: String
    let clock: StreamClock
    let shown: @Sendable (String, Page?) -> Void
    private let lock = NSLock()
    private var task: Task<Void, Never>?
    private var current: String?

    public init(socket: String, clock: StreamClock = SystemClock(), shown: @escaping @Sendable (String, Page?) -> Void) {
        self.socket = socket
        self.clock = clock
        self.shown = shown
    }

    public var watched: String? { lock.withLock { current } }

    public func open(_ page: String) {
        let sub = Subscription(path: socket, pages: [page], clock: clock) { [weak self] event in
            guard case .page(let p) = event, let self, self.watched == page else { return }
            self.shown(page, p)
        }
        lock.withLock {
            task?.cancel()
            current = page
            task = Task.detached { await sub.run() }
        }
    }

    public func window(_ flag: String) { open(Self.page(Self.activity, of: flag)) }

    public func back() {
        let was = lock.withLock { () -> String? in
            task?.cancel()
            task = nil
            defer { current = nil }
            return current
        }
        if let was { shown(was, nil) }
    }

    // value: the popover's handlers with its navigation: open a page, pick
    // Activity's window, go back to Home.
    public func wire(_ handlers: PopoverHandlers) -> PopoverHandlers {
        var h = handlers
        h.push = { [weak self] in self?.open($0) }
        h.window = { [weak self] in self?.window($0) }
        h.back = { [weak self] in self?.back() }
        return h
    }
}
