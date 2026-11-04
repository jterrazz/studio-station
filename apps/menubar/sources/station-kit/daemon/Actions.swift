import Foundation

public struct Status: Equatable, Sendable {
    public var line: String
    public var failed: Bool
    public init(_ line: String, failed: Bool) {
        self.line = line
        self.failed = failed
    }
}

public struct ActionRunner: Sendable {
    public var executable: String
    public var socket: String
    public var launcher: Launcher
    public var terminal: String

    // value: the call that ends the engine answering it: its connection's
    // end, with no answer, is its success.
    public static let endsTheEngine = "engine.stop"

    public init(executable: String, socket: String, launcher: Launcher = ProcessLauncher(), terminal: String = "Terminal") {
        self.executable = executable
        self.socket = socket
        self.launcher = launcher
        self.terminal = terminal
    }

    public static func filled(_ action: Action, answers: [String: String]) -> [String: String] {
        var out: [String: String] = [:]
        for f in action.form?.fields ?? [] {
            guard let key = f.key, let value = answers[key] ?? f.default, !value.isEmpty else { continue }
            out[key] = value
        }
        return out
    }

    // invariant: the outcome is the status line: nil when the action did what
    // it said, else the engine's own message and hint. A call goes over the
    // socket; only a terminal action starts a program, the terminal.
    public func run(_ action: Action, answers: [String: String] = [:]) async -> Status? {
        guard let run = action.run else { return nil }
        if let name = run.call {
            let reply = await Client.call(path: socket, name: name, input: Client.input(Self.filled(action, answers: answers), fixed: run.in))
            if reply.ok || (reply.closed && name == Self.endsTheEngine) { return nil }
            return Self.failure(reply.message ?? name, reply.hint)
        }
        guard run.terminal == true else { return nil }
        let open = Terminal.opening(terminal, executable: executable, words: run.argv ?? [])
        let done = await launcher.run(open.path, open.args, environment: [:])
        return done.code == 0 ? nil : Self.failure("\(terminal) did not open station", nil)
    }

    static func failure(_ message: String, _ hint: String?) -> Status {
        Status(hint.map { "\(message)  \(Token.Glyph.arrow) \($0)" } ?? message, failed: true)
    }
}
