import Foundation
import StationKit
import XCTest

final class TestClock: StreamClock, @unchecked Sendable {
    private let lock = NSLock()
    private var asked: [Double] = []
    private var waiters: [Int: CheckedContinuation<Void, Error>] = [:]
    private var next = 0

    var backoffs: [Double] { lock.withLock { asked.filter { $0 != Client.silence } } }

    func sleep(_ seconds: Double) async throws {
        lock.withLock { asked.append(seconds) }
        guard seconds == Client.silence else {
            if backoffs.count >= 3 { throw CancellationError() }
            return
        }
        let id = lock.withLock { next += 1; return next }
        try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { (c: CheckedContinuation<Void, Error>) in
                lock.withLock { waiters[id] = c }
                if Task.isCancelled { lock.withLock { waiters.removeValue(forKey: id) }?.resume(throwing: CancellationError()) }
            }
        } onCancel: {
            lock.withLock { waiters.removeValue(forKey: id) }?.resume(throwing: CancellationError())
        }
    }

    func releaseSilence() {
        let all = lock.withLock { () -> [CheckedContinuation<Void, Error>] in
            defer { waiters.removeAll() }
            return Array(waiters.values)
        }
        all.forEach { $0.resume() }
    }
}

final class FakeDaemon: @unchecked Sendable {
    let path: String
    let fd: Int32
    private let lock = NSLock()
    private var requests: [String] = []

    init(_ name: String) throws {
        try FileManager.default.createDirectory(atPath: "/private/tmp/station-work", withIntermediateDirectories: true)
        path = "/private/tmp/station-work/\(name)-\(getpid()).sock"
        unlink(path)
        fd = socket(AF_UNIX, SOCK_STREAM, 0)
        var addr = sockaddr_un()
        addr.sun_family = sa_family_t(AF_UNIX)
        let bytes = Array(path.utf8)
        withUnsafeMutableBytes(of: &addr.sun_path) { raw in
            raw.copyBytes(from: bytes)
            raw[bytes.count] = 0
        }
        let bound = withUnsafePointer(to: &addr) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { bind(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
        }
        guard bound == 0, listen(fd, 4) == 0 else { throw SocketError.refused(errno) }
    }

    var received: [String] { lock.withLock { requests } }

    // value: accepts one client, reads its request line, answers with these
    // bytes, then stops listening so a reconnect is refused.
    func serveOnce(_ answer: Data, thenClose: Bool) {
        Thread.detachNewThread { [self] in
            let client = accept(fd, nil, nil)
            if thenClose {
                close(fd)
                unlink(path)
            }
            var line = Data()
            var byte: UInt8 = 0
            while read(client, &byte, 1) == 1, byte != 0x0A { line.append(byte) }
            lock.withLock { requests.append(String(decoding: line, as: UTF8.self)) }
            _ = answer.withUnsafeBytes { write(client, $0.baseAddress, $0.count) }
            while read(client, &byte, 1) == 1 {}
            close(client)
        }
    }

    func stop() {
        close(fd)
        unlink(path)
    }
}

final class Recorder: @unchecked Sendable {
    private let lock = NSLock()
    private var list: [StreamEvent] = []
    func add(_ e: StreamEvent) { lock.withLock { list.append(e) } }
    var events: [StreamEvent] { lock.withLock { list } }
}

struct RecordingLauncher: Launcher {
    let calls: Recorded
    func run(_ path: String, _ args: [String], environment: [String: String]) async -> Launched {
        calls.add([path] + args)
        return Launched(code: 0, stderr: "")
    }
}

final class Recorded: @unchecked Sendable {
    private let lock = NSLock()
    private var list: [[String]] = []
    func add(_ c: [String]) { lock.withLock { list.append(c) } }
    var all: [[String]] { lock.withLock { list } }
}

final class ClientTests: XCTestCase {
    func until(_ condition: @escaping () -> Bool) async {
        for _ in 0..<500 where !condition() {
            try? await Task.sleep(nanoseconds: 10_000_000)
        }
    }

    func testTheRecordedStreamIsReadInOrderThenSilenceReconnectsWithBackoff() async throws {
        let daemon = try FakeDaemon("mb-stream")
        defer { daemon.stop() }
        daemon.serveOnce(try Data(contentsOf: Tree.stream), thenClose: true)
        let clock = TestClock(), recorder = Recorder()
        let sub = Subscription(path: daemon.path, clock: clock) { recorder.add($0) }
        let task = Task { await sub.run() }
        await until { recorder.events.count >= 5 }
        let kinds = recorder.events.prefix(5).map { e -> String in
            switch e {
            case .connected: "connected"
            case .page(let p): "page \(p.state ?? "")"
            case .tick: "tick"
            case .notice(let kind, let text): "notice \(kind): \(text)"
            case .down: "down"
            }
        }
        XCTAssertEqual(kinds, ["connected", "page loading", "page ready", "tick", "notice power_ended: Power ended: battery 20% on battery"])
        XCTAssertEqual(daemon.received, [#"{"id":"1","op":"subscribe","pages":["menubar"],"v":1}"#])
        for _ in 0..<200 where clock.backoffs.isEmpty {
            clock.releaseSilence()
            try await Task.sleep(nanoseconds: 10_000_000)
        }
        await task.value
        XCTAssertEqual(clock.backoffs, [1, 2, 4])
    }

    func fixtureAction(_ id: String) throws -> Action {
        try XCTUnwrap(Tree.page("menubar.full").action(id) ?? Tree.page("menubar.empty").action(id))
    }

    func fake(_ name: String, exit code: Int32) throws -> (path: String, log: String) {
        let dir = "/private/tmp/station-work"
        let path = "\(dir)/\(name)-\(getpid()).sh", log = "\(dir)/\(name)-\(getpid()).log"
        let script = """
        #!/bin/sh
        printf '%s\\n' "argv=$*" "NO_COLOR=$NO_COLOR" "stdin=$(cat)" > \(log)
        if [ \(code) -ne 0 ]; then printf 'Error: api did not stop\\nHint: station machines\\n' >&2; fi
        exit \(code)
        """
        try script.write(toFile: path, atomically: true, encoding: .utf8)
        chmod(path, 0o755)
        return (path, log)
    }

    func testAnArgvActionRunsStationWithTheFormsAnswersNoColourAndNoStdin() async throws {
        let (exe, log) = try fake("mb-ok", exit: 0)
        let status = await ActionRunner(executable: exe, socket: "/nonexistent").run(try fixtureAction("new-vm"), answers: ["lifetime": "lasting"])
        XCTAssertNil(status)
        XCTAssertEqual(try String(contentsOfFile: log, encoding: .utf8), "argv=machines create --lifetime lasting --memory 4G\nNO_COLOR=1\nstdin=\n")
    }

    func testAFailingActionsErrorAndHintMakeTheStatusLine() async throws {
        let (exe, log) = try fake("mb-fail", exit: 1)
        let status = await ActionRunner(executable: exe, socket: "/nonexistent").run(try fixtureAction("stop-all"))
        XCTAssertEqual(status, Status("api did not stop  → station machines", failed: true))
        XCTAssertTrue(try String(contentsOfFile: log, encoding: .utf8).hasPrefix("argv=stop --yes\n"))
    }

    func testTheTerminalActionOpensStationInTheTerminal() async throws {
        let calls = Recorded()
        let status = await ActionRunner(executable: "/u/bin/station", socket: "/nonexistent", launcher: RecordingLauncher(calls: calls)).run(try fixtureAction("open-station"))
        XCTAssertNil(status)
        XCTAssertEqual(calls.all, [["/usr/bin/open", "-a", "Terminal", "/u/bin/station"]])
    }

    func testAnOpActionIsOneRequestLineOnItsOwnConnection() async throws {
        let daemon = try FakeDaemon("mb-op")
        defer { daemon.stop() }
        daemon.serveOnce(Data(#"{"v":1,"id":"1","ok":false,"error":{"code":"power_guard","message":"Power stays off: the battery is at 18% on battery","hint":"station power"}}"#.utf8 + [0x0A]), thenClose: false)
        let status = await ActionRunner(executable: "/u/bin/station", socket: daemon.path).run(try fixtureAction("power-awake"), answers: ["lid": "lid=true battery=true"])
        XCTAssertEqual(daemon.received, [#"{"battery":true,"for":14400,"id":"1","lid":true,"op":"power_awake","v":1}"#])
        XCTAssertEqual(status, Status("Power stays off: the battery is at 18% on battery  → station power", failed: true))
    }
}
