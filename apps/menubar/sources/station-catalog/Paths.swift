import Foundation

enum Paths {
    static let package = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    static let stories = package.appendingPathComponent("stories")
    static let goldens = package.appendingPathComponent("tests/station-kit-tests/testdata/stories")

    static var design: URL? {
        ProcessInfo.processInfo.environment["STATION_DESIGN"].map { URL(fileURLWithPath: $0) }
    }

    static func capture(_ id: String, dark: Bool) -> URL? {
        guard let design else { return nil }
        let url = design.appendingPathComponent("v4/stories/\(id)\(dark ? "" : ".light").png")
        return FileManager.default.fileExists(atPath: url.path) ? url : nil
    }

    static func drifted(_ id: String) -> Bool {
        ["dark", "light"].contains { FileManager.default.fileExists(atPath: goldens.appendingPathComponent("\(id).\($0).actual.png").path) }
    }

    static func order() -> [String] {
        guard let text = try? String(contentsOf: stories.appendingPathComponent("STORIES.md"), encoding: .utf8) else { return [] }
        return text.split(separator: "\n").compactMap { line in
            guard line.hasPrefix("### `"), let end = line.dropFirst(5).firstIndex(of: "`") else { return nil }
            return String(line.dropFirst(5)[..<end])
        }
    }
}
