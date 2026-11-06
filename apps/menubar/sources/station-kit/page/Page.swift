import Foundation

public let pageSchema = "station.page/v1"

public struct Page: Codable, Equatable, Sendable {
    public var schema: String?
    public var id: String?
    public var command: String?
    public var context: Context?
    public var state: String?
    public var strip: Strip?
    public var sections: [Section]?
    public var hints: [Hint]?
    public var error: PageError?
    public var actions: [Action]?
    public var badge: Badge?

    public static func decode(_ data: Data) throws -> Page {
        let page = try JSONDecoder().decode(Page.self, from: data)
        guard page.schema == pageSchema else { throw DecodeError.schema(page.schema ?? "") }
        return page
    }

    public func encoded() throws -> Data {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.sortedKeys]
        return try encoder.encode(self)
    }

    public var loading: Bool { state == "loading" }
    public func section(_ id: String) -> Section? { sections?.first { $0.id == id } }
    public func action(_ id: String) -> Action? { actions?.first { $0.id == id } }
    public var pageActions: [Action] { (actions ?? []).filter { $0.on == "page" && $0.run != nil } }
    public func rowActions(_ row: String) -> [Action] { (actions ?? []).filter { $0.on == "row" && $0.run != nil && $0.rows?.contains(row) == true } }
}

public struct Badge: Codable, Equatable, Sendable {
    public init(needs: Int?, down: Bool? = nil) {
        self.needs = needs
        self.down = down
    }
    public var needs: Int?
    public var down: Bool?
}

public enum DecodeError: Error, Equatable {
    case schema(String)
}

public struct Context: Codable, Equatable, Sendable {
    public var machine: String?
    public var kind: String?
    public var registered: Bool?
    public init(machine: String? = nil, kind: String? = nil, registered: Bool? = nil) {
        self.machine = machine
        self.kind = kind
        self.registered = registered
    }
}

public struct Section: Codable, Equatable, Sendable {
    public var id: String?
    public var title: String?
    public var role: String?
    public var fold: String?
    public var caption: [Value]?
    public var columns: [Column]?
    public var items: [Item]?
    public var counts: [Tally]?
    public var uncounted: Bool?
    public var pipe_counts: Bool?
    public var pipe_groups: Bool?
    public var `break`: Int?
}

public struct Column: Codable, Equatable, Sendable {
    public var key: String?
    public var label: String?
    public var kind: String?
    public var keep: Int?
    public var show: String?
    public var levels: [String]?
    public var whole: Bool?
}

public struct Item: Codable, Equatable, Sendable {
    public init(row: Row? = nil) { self.row = row }
    public var row: Row?
    public var fact: Fact?
    public var group: Group?
    public var empty: Empty?
    public var error: PageError?
    public var note: Note?
    public var link: Link?
}

public struct Row: Codable, Equatable, Sendable {
    public init() {}
    public var id: String?
    public var mark: Mark?
    public var label: String?
    public var short: String?
    public var cells: [Value]?
    public var detail: Value?
    public var reason: String?
    public var hint: Hint?
}

public struct Fact: Codable, Equatable, Sendable {
    public var key: String?
    public var value: Value?
}

public struct Group: Codable, Equatable, Sendable {
    public var head: Row?
    public var items: [Item]?
    public var fold: String?
}

public struct Empty: Codable, Equatable, Sendable {
    public var noun: String?
    public var hint: Hint?
}

public struct PageError: Codable, Equatable, Sendable {
    public var message: String?
    public var hint: String?
}

public struct Note: Codable, Equatable, Sendable {
    public var text: String?
}

public struct Link: Codable, Equatable, Sendable {
    public var label: String?
    public var lead: Value?
    public var value: Value?
    public var detail: Value?
    public var page: String?
    public var row: String?
}

public struct Hint: Codable, Equatable, Sendable {
    public init(command: String? = nil) { self.command = command }
    public var command: String?
    public var external: Bool?
    public var key: String?
    public var text: String?
}

public struct Action: Codable, Equatable, Sendable {
    public var key: String?
    public var label: String?
    public var on: String?
    public var rows: [String]?
    public var confirm: Confirm?
    public var priority: Int?
    public var mutates: Bool?
    public var id: String?
    public var run: Run?
    public var form: Form?
}

public struct Run: Codable, Equatable, Sendable {
    public var call: String?
    public var `in`: [String: String]?
    public var terminal: Bool?
    public var argv: [String]?
    public var target: Target?
}

public struct Target: Codable, Equatable, Sendable {
    public var name: String?
    public var on: String?
}

public struct Confirm: Codable, Equatable, Sendable {
    public var question: String?
    public var lines: [Value]?
    public var danger: Bool?
    public var type: String?
    public var yes: String?
    public var no: String?
}

public struct Form: Codable, Equatable, Sendable {
    public var title: String?
    public var fields: [Field]?
}

public struct Field: Codable, Equatable, Sendable {
    public var key: String?
    public var label: String?
    public var kind: String?
    public var options: [Option]?
    public var `default`: String?
    public var note: String?
    public var required: Bool?
}

public struct Option: Codable, Equatable, Sendable {
    public var value: String?
    public var label: String?
    public var note: String?
}

public struct Strip: Codable, Equatable, Sendable {
    public var cpu: Value?
    public var spark: [Int]?
    public var memory: Value?
    public var memory_spark: [Int]?
    public var net_spark: [Int]?
    public var battery: Value?
    public var net: [Value]?
    public var awake: Value?
    public var top: Section?
}

public struct Tally: Codable, Equatable, Sendable {
    public init(mark: Mark?, n: Int?) {
        self.mark = mark
        self.n = n
    }
    public var mark: Mark?
    public var n: Int?
}

public enum Mark: String, Codable, Sendable, CaseIterable {
    case ok, attention, absent, broken, pending
    case offHere = "off-here"
    case notForThisMachine = "not-for-this-machine"
    case notKnown = "unknown"

    public init(from decoder: Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        self = Mark(rawValue: raw) ?? .absent
    }

    public var rank: Int {
        switch self {
        case .broken: 4
        case .attention: 3
        case .pending: 2
        case .absent, .offHere, .notKnown: 1
        case .ok, .notForThisMachine: 0
        }
    }
}

public struct Value: Codable, Equatable, Sendable {
    public var kind: String?
    public var text: String?
    public var n: Int64?
    public var of: Int64?
    public var at: String?
    public var unit: String?
    public var label: String?
    public var items: [Value]?
    public var stale: Bool?

    public init(kind: String? = nil, text: String? = nil) {
        self.kind = kind
        self.text = text
    }

    public var isNone: Bool { kind == nil || kind == "none" }
}
