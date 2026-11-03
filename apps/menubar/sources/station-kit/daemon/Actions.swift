import Foundation

public struct Status: Equatable, Sendable {
    public var line: String
    public var failed: Bool
    public init(_ line: String, failed: Bool) {
        self.line = line
        self.failed = failed
    }
}

public struct Launched: Sendable, Equatable {
    public var code: Int32
    public var stderr: String
    public init(code: Int32, stderr: String) {
        self.code = code
        self.stderr = stderr
    }
}

public protocol Launcher: Sendable {
    func run(_ path: String, _ args: [String], environment: [String: String]) async -> Launched
}

public struct ProcessLauncher: Launcher {
    public static let budget = 600.0

    public init() {}

    public func run(_ path: String, _ args: [String], environment: [String: String]) async -> Launched {
        await withCheckedContinuation { c in
            DispatchQueue.global(qos: .userInitiated).async {
                let p = Process()
                p.executableURL = URL(fileURLWithPath: path)
                p.arguments = args
                p.environment = ProcessInfo.processInfo.environment.merging(environment) { $1 }
                p.standardInput = FileHandle.nullDevice
                p.standardOutput = FileHandle.nullDevice
                let err = Pipe()
                p.standardError = err
                do { try p.run() } catch {
                    c.resume(returning: Launched(code: 127, stderr: "Error: \(path) could not run"))
                    return
                }
                let timer = DispatchWorkItem { if p.isRunning { p.terminate() } }
                DispatchQueue.global().asyncAfter(deadline: .now() + Self.budget, execute: timer)
                let data = err.fileHandleForReading.readDataToEndOfFile()
                p.waitUntilExit()
                timer.cancel()
                c.resume(returning: Launched(code: p.terminationStatus, stderr: String(decoding: data, as: UTF8.self)))
            }
        }
    }
}

public struct ActionRunner: Sendable {
    public var executable: String
    public var socket: String
    public var launcher: Launcher
    public var terminal: String

    public init(executable: String, socket: String, launcher: Launcher = ProcessLauncher(), terminal: String = "Terminal") {
        self.executable = executable
        self.socket = socket
        self.launcher = launcher
        self.terminal = terminal
    }

    public static func filled(_ action: Action, answers: [String: String]) -> [(key: String, value: String)] {
        (action.form?.fields ?? []).compactMap { f in
            guard let key = f.key, let value = answers[key] ?? f.default, !value.isEmpty else { return nil }
            return (key, value)
        }
    }

    public func argv(_ action: Action, answers: [String: String]) -> [String] {
        (action.run?.argv ?? []) + Self.filled(action, answers: answers).flatMap { ["--\($0.key)", $0.value] }
    }

    // invariant: the outcome is the status line: nil when the action did what
    // it said, else the command's own Error and Hint lines.
    public func run(_ action: Action, answers: [String: String] = [:]) async -> Status? {
        guard let run = action.run else { return nil }
        if let op = run.op {
            let given = Dictionary(Self.filled(action, answers: answers), uniquingKeysWith: { $1 })
            let reply = await Client.request(path: socket, op: op, args: Client.arguments(given, fixed: run.args))
            return reply.ok ? nil : Self.failure(reply.message ?? op, reply.hint)
        }
        if run.terminal == true {
            let done = await launcher.run("/usr/bin/open", ["-a", terminal, executable] , environment: [:])
            return done.code == 0 ? nil : Self.failure("\(terminal) did not open station", nil)
        }
        let done = await launcher.run(executable, argv(action, answers: answers), environment: ["NO_COLOR": "1"])
        guard done.code != 0 else { return nil }
        var message = "station exited \(done.code)", hint: String?
        for line in done.stderr.split(separator: "\n") {
            if line.hasPrefix("Error: ") { message = String(line.dropFirst(7)) }
            if line.hasPrefix("Hint: ") { hint = String(line.dropFirst(6)) }
        }
        return Self.failure(message, hint)
    }

    static func failure(_ message: String, _ hint: String?) -> Status {
        Status(hint.map { "\(message)  \(Token.Glyph.arrow) \($0)" } ?? message, failed: true)
    }
}
