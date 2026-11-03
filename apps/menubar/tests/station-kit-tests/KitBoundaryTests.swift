import XCTest

final class KitBoundaryTests: XCTestCase {
    struct Finding: Equatable, CustomStringConvertible {
        let file: String
        let rule: String
        var description: String { "\(file): \(rule)" }
    }

    static let forbidden = ["ProgressView", "Gauge(", "List(", "List {", ".toggleStyle(.switch)", "WindowGroup", "NSWindow(", "Settings {"]
    static let glyphs: [Character] = ["→", "·", "…", "⏻", "⚡", "●", "◐", "○", "✗", "◌"]

    // invariant: the Swift side of the kit's boundary: colours, symbol names
    // and glyphs live in Token only, no AppKit-backed control the offscreen
    // renderer cannot draw, and Page and Daemon stay Foundation.
    static func scan(_ files: [(path: String, text: String)]) -> [Finding] {
        var out: [Finding] = []
        for (path, text) in files {
            let token = path.hasPrefix("token/")
            if !token {
                if text.contains("Color(") || text.range(of: #"Color\.(red|green|blue|yellow|orange|black|white|gray)"#, options: .regularExpression) != nil {
                    out.append(Finding(file: path, rule: "a colour outside Token"))
                }
                if text.contains("systemName: \"") {
                    out.append(Finding(file: path, rule: "a symbol name outside Token"))
                }
                if text.contains(where: { glyphs.contains($0) }) {
                    out.append(Finding(file: path, rule: "a glyph outside Token"))
                }
            }
            for word in forbidden where text.contains(word) {
                out.append(Finding(file: path, rule: "\(word) is not drawn offscreen"))
            }
            if path.hasPrefix("page/") || path.hasPrefix("daemon/") {
                for line in text.split(separator: "\n") where line.hasPrefix("import ") && line != "import Foundation" {
                    out.append(Finding(file: path, rule: "\(line): Page and Daemon import Foundation only"))
                }
            }
        }
        return out
    }

    func testTheKitKeepsItsBoundary() throws {
        let root = Tree.sources
        let files = FileManager.default.enumerator(at: root, includingPropertiesForKeys: nil)!.compactMap { $0 as? URL }
            .filter { $0.pathExtension == "swift" }
            .map { (String($0.path.dropFirst(root.path.count + 1)), try! String(contentsOf: $0, encoding: .utf8)) }
        XCTAssertGreaterThan(files.count, 5)
        XCTAssertEqual(Self.scan(files), [])
    }

    func testTheScanCatchesWhatItForbids() {
        let caught = Self.scan([
            ("atom/Dot.swift", "import SwiftUI\nstruct D: View { var body: some View { ProgressView().foregroundStyle(Color(red: 1, green: 0, blue: 0)) } }"),
            ("molecule/Row.swift", "Image(systemName: \"circle\"); Text(\"→\")"),
            ("page/Page.swift", "import SwiftUI"),
        ]).map(\.rule)
        XCTAssertEqual(caught, [
            "a colour outside Token", "ProgressView is not drawn offscreen",
            "a symbol name outside Token", "a glyph outside Token",
            "import SwiftUI: Page and Daemon import Foundation only",
        ])
    }
}
