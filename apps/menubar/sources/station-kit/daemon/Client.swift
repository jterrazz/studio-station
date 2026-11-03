import Foundation

public protocol StreamClock: Sendable {
    func sleep(_ seconds: Double) async throws
}

public struct SystemClock: StreamClock {
    public init() {}
    public func sleep(_ seconds: Double) async throws { try await Task.sleep(nanoseconds: UInt64(seconds * 1_000_000_000)) }
}

public struct Reply: Sendable {
    public var ok: Bool
    public var result: [String: AnySendable]
    public var message: String?
    public var hint: String?
}

public struct AnySendable: @unchecked Sendable {
    public let value: Any
}

public enum StreamEvent: Sendable {
    case connected
    case page(Page)
    case tick
    case notice(kind: String, text: String)
    case down
}

public enum Client {
    public static let silence = 45.0
    public static let backoffCap = 30.0

    static func readLine(_ socket: UnixSocket) async throws -> Data {
        try await withCheckedThrowingContinuation { (c: CheckedContinuation<Data, Error>) in
            DispatchQueue.global(qos: .utility).async {
                do { c.resume(returning: try socket.readLine()) } catch { c.resume(throwing: error) }
            }
        }
    }

    // value: an argument written `k=v k=v` sets each of those arguments; a
    // number or true/false is sent as one.
    public static func arguments(_ answers: [String: String], fixed: [String: String]? = nil) -> [String: Any] {
        var out: [String: Any] = [:]
        func put(_ key: String, _ value: String) {
            if let n = Int(value) { out[key] = n } else if value == "true" || value == "false" { out[key] = value == "true" } else { out[key] = value }
        }
        for (k, v) in fixed ?? [:] { put(k, v) }
        for (k, v) in answers {
            if v.contains("=") {
                for pair in v.split(separator: " ") {
                    let kv = pair.split(separator: "=", maxSplits: 1).map(String.init)
                    if kv.count == 2 { put(kv[0], kv[1]) }
                }
            } else {
                put(k, v)
            }
        }
        return out
    }

    public static func request(path: String, op: String, args: [String: Any] = [:]) async -> Reply {
        var body: [String: Any] = ["v": 1, "id": "1", "op": op]
        for (k, v) in args { body[k] = v }
        guard let socket = try? UnixSocket.connect(path), let data = try? JSONSerialization.data(withJSONObject: body, options: [.sortedKeys]) else {
            return Reply(ok: false, result: [:], message: "the station daemon is not running", hint: "station start")
        }
        do {
            try socket.write(line: data)
            let line = try await readLine(socket)
            let obj = (try JSONSerialization.jsonObject(with: line)) as? [String: Any] ?? [:]
            let error = obj["error"] as? [String: Any]
            let result = (obj["result"] as? [String: Any] ?? [:]).mapValues { AnySendable(value: $0) }
            return Reply(ok: obj["ok"] as? Bool ?? false, result: result, message: error?["message"] as? String, hint: error?["hint"] as? String)
        } catch {
            return Reply(ok: false, result: [:], message: "the station daemon closed the connection", hint: "station start")
        }
    }

    public static func executable(path: String) async -> String? {
        let reply = await request(path: path, op: "ping")
        return reply.result["executable"]?.value as? String
    }
}

final class Seen: @unchecked Sendable {
    private let lock = NSLock()
    private var n = 0
    func bump() { lock.lock(); n += 1; lock.unlock() }
    var value: Int { lock.lock(); defer { lock.unlock() }; return n }
}

public final class Subscription: @unchecked Sendable {
    let path: String
    let pages: [String]
    let clock: StreamClock
    let emit: @Sendable (StreamEvent) -> Void

    public init(path: String, pages: [String] = ["menubar"], clock: StreamClock = SystemClock(), emit: @escaping @Sendable (StreamEvent) -> Void) {
        self.path = path
        self.pages = pages
        self.clock = clock
        self.emit = emit
    }

    // invariant: one subscribed connection at a time; a connection that ends,
    // or says nothing for 45 s, is dropped and made again after 1, 2, 4 up to 30 s,
    // the wait starting over once a connection streamed again.
    public func run() async {
        var backoff = 1.0
        while !Task.isCancelled {
            if let socket = try? UnixSocket.connect(path), await stream(socket) {
                backoff = 1
            }
            emit(.down)
            do { try await clock.sleep(backoff) } catch { return }
            backoff = min(backoff * 2, Client.backoffCap)
        }
    }

    func stream(_ socket: UnixSocket) async -> Bool {
        guard let request = try? JSONSerialization.data(withJSONObject: ["v": 1, "id": "1", "op": "subscribe", "pages": pages], options: [.sortedKeys]),
              (try? socket.write(line: request)) != nil else { return false }
        let seen = Seen()
        return await withTaskGroup(of: Void.self) { group in
            group.addTask { [clock] in
                while true {
                    let before = seen.value
                    do { try await clock.sleep(Client.silence) } catch { return }
                    if seen.value == before {
                        socket.shutdownBoth()
                        return
                    }
                }
            }
            var streamed = false
            while let line = try? await Client.readLine(socket) {
                seen.bump()
                guard let obj = (try? JSONSerialization.jsonObject(with: line)) as? [String: Any] else { continue }
                if obj["id"] != nil {
                    guard obj["ok"] as? Bool == true else { break }
                    streamed = true
                    emit(.connected)
                    continue
                }
                switch obj["event"] as? String {
                case "page":
                    if let raw = obj["page"], let data = try? JSONSerialization.data(withJSONObject: raw), let page = try? Page.decode(data) {
                        emit(.page(page))
                    }
                case "tick":
                    emit(.tick)
                case "notice":
                    let n = obj["notice"] as? [String: Any]
                    emit(.notice(kind: n?["kind"] as? String ?? "", text: n?["text"] as? String ?? ""))
                default:
                    break
                }
            }
            socket.shutdownBoth()
            group.cancelAll()
            return streamed
        }
    }
}
