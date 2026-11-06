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

final class FakeEngine: @unchecked Sendable {
    let path: String
    let fd: Int32
    private let lock = NSLock()
    private var requests: [String] = []
    private var hungUp = false

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
    // value: the client closed its connection.
    var left: Bool { lock.withLock { hungUp } }

    // value: accepts one client, reads its request line, answers with these
    // bytes, then stops listening so a reconnect is refused.
    func serveOnce(_ answer: Data, thenClose: Bool, hangUp: Bool = false) {
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
            while !hangUp, read(client, &byte, 1) == 1 {}
            lock.withLock { hungUp = true }
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
        let engine = try FakeEngine("mb-stream")
        defer { engine.stop() }
        engine.serveOnce(try Data(contentsOf: Tree.stream), thenClose: true)
        let clock = TestClock(), recorder = Recorder()
        let sub = Subscription(path: engine.path, clock: clock) { recorder.add($0) }
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
        XCTAssertEqual(kinds, ["connected", "page loading", "page ready", "tick", "notice awake_ended: Awake stopped on This Mac: battery 20 %"])
        XCTAssertEqual(engine.received, [#"{"id":"1","in":{"pages":["menubar"]},"v":2,"watch":"page"}"#])
        for _ in 0..<200 where clock.backoffs.isEmpty {
            clock.releaseSilence()
            try await Task.sleep(nanoseconds: 10_000_000)
        }
        await task.value
        XCTAssertEqual(clock.backoffs, [1, 2, 4])
    }

    func testThePopoversWatchIsHeldOpenUntilThePopoverCloses() async throws {
        let engine = try answered("mb-watch", #"{"v":2,"id":"1","out":{"machines":[]}}"#)
        defer { engine.stop() }
        let held = Task { await Client.hold(path: engine.path, watch: "engine.watch") }
        await until { !engine.received.isEmpty }
        try await Task.sleep(nanoseconds: 100_000_000)
        XCTAssertFalse(engine.left, "the popover's watch ended while it was open")
        held.cancel()
        await held.value
        await until { engine.left }
        XCTAssertTrue(engine.left, "the popover closed and its watch stayed open")
        XCTAssertEqual(engine.received, [#"{"id":"1","v":2,"watch":"engine.watch"}"#])
    }

    func fixtureAction(_ id: String) throws -> Action {
        try XCTUnwrap(Tree.page("menubar.full").action(id) ?? Tree.page("menubar.empty").action(id))
    }

    func answered(_ name: String, _ frame: String, hangUp: Bool = false) throws -> FakeEngine {
        let engine = try FakeEngine(name)
        engine.serveOnce(Data(frame.utf8) + (frame.isEmpty ? [] : [0x0A]), thenClose: false, hangUp: hangUp)
        return engine
    }

    func testACallActionIsOneFrameWithItsFormsAnswersAndStartsNoProgram() async throws {
        let engine = try answered("mb-create", #"{"v":2,"id":"1","progress":{"task":"Booting api"}}"# + "\n" + #"{"v":2,"id":"1","out":{"VM":{"Name":"api"}}}"#)
        defer { engine.stop() }
        let calls = Recorded()
        let status = await ActionRunner(executable: "/u/bin/station", socket: engine.path, launcher: RecordingLauncher(calls: calls)).run(try fixtureAction("new-vm"), answers: ["Request.Lifetime": "permanent"])
        XCTAssertNil(status)
        XCTAssertEqual(engine.received, [#"{"call":"machines.create-workspace","id":"1","in":{"Request":{"Image":"linux","Lifetime":"permanent","Network":"nat","Size":{"MemoryBytes":4294967296}}},"v":2}"#])
        XCTAssertEqual(calls.all, [])
    }

    func testAFailingCallsMessageAndHintMakeTheStatusLine() async throws {
        let engine = try answered("mb-fail", #"{"v":2,"id":"1","error":{"type":"failed","code":"internal","message":"api did not stop","hint":"station machines"}}"#)
        defer { engine.stop() }
        let status = await ActionRunner(executable: "/u/bin/station", socket: engine.path).run(try fixtureAction("awake-keep"), answers: ["Lid": "Lid=true Battery=true"])
        XCTAssertEqual(engine.received, [#"{"call":"awake.keep","id":"1","in":{"Battery":true,"For":14400000000000,"Lid":true},"v":2}"#])
        XCTAssertEqual(status, Status("api did not stop  → station machines", failed: true, posted: "awake failed: api did not stop"))
    }

    func testStopEverythingIsDoneWhenTheEngineItStoppedHangsUp() async throws {
        let engine = try answered("mb-stop", "", hangUp: true)
        defer { engine.stop() }
        let status = await ActionRunner(executable: "/u/bin/station", socket: engine.path).run(try fixtureAction("stop-all"))
        XCTAssertEqual(engine.received, [#"{"call":"engine.stop","id":"1","in":{"By":"the menu bar","Confirm":"yes"},"v":2}"#])
        XCTAssertNil(status)
    }

    func testTheTerminalActionsOpenStationInTheTerminalAndNothingElse() async throws {
        let calls = Recorded()
        let runner = ActionRunner(executable: "/u/bin/station", socket: "/nonexistent", launcher: RecordingLauncher(calls: calls))
        let opened = await runner.run(try fixtureAction("open-station"))
        let started = await runner.run(try XCTUnwrap(Tree.page("menubar.error").action("start")))
        XCTAssertNil(opened)
        XCTAssertNil(started)
        XCTAssertEqual(calls.all, [
            ["/usr/bin/open", "-a", "Terminal", "/u/bin/station"],
            ["/usr/bin/osascript", "-e", #"tell application "Terminal" to do script "'/u/bin/station' 'engine' 'start'""#, "-e", #"tell application "Terminal" to activate"#],
        ])
    }
}
