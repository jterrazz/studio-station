import XCTest

final class KitBoundaryTests: XCTestCase {
    struct Finding: Equatable, CustomStringConvertible {
        let file: String
        let rule: String
        var description: String { "\(file): \(rule)" }
    }

    static let forbidden = ["ProgressView", "Gauge(", "List(", "List {", "WindowGroup", "NSWindow(", "Settings {"]
    static let models = ["\"laptopcomputer", "\"macmini", "\"macbook", "\"macpro", "\"imac", "\"macstudio"]
    // value: L50d, Apple's rules: the MenuBarExtra's window is the only
    // glass, so no view of the kit is glass or a material; L50e: the
    // vision's tiles are translucent fills drawn by molecule/Surface only.
    static let glass = ["glassEffect", "GlassEffectContainer", "Material", ".containerBackground"]
    static let surface = "molecule/Surface.swift"
    static let surfaceOnly = [".quinary", "Token.Fill.tile", ".glassProminent", ".buttonStyle(.bordered", ".toggleStyle(.switch)"]
    static let glyphs: [Character] = ["→", "·", "…", "⏻", "⚡", "●", "◐", "○", "✗", "◌"]

    // invariant: the Swift side of the kit's boundary: colours, symbol names
    // and glyphs live in Token only, no AppKit-backed control the offscreen
    // renderer cannot draw, every Mac drawn with one symbol (DECISIONS E1/E2:
    // the page reports no model), and Page and Engine stay Foundation.
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
            for model in models where text.contains(model) {
                out.append(Finding(file: path, rule: "\(model.dropFirst()) draws a Mac's model, which no capability reports"))
            }
            for word in forbidden where text.contains(word) {
                out.append(Finding(file: path, rule: "\(word) is not drawn offscreen"))
            }
            for word in glass where text.contains(word) {
                out.append(Finding(file: path, rule: "\(word) in the content"))
            }
            if path != surface {
                for word in surfaceOnly where text.contains(word) {
                    out.append(Finding(file: path, rule: "\(word) outside the kit's surface"))
                }
            }
            if text.range(of: #"\.background\(palette\.(?!qrLight)"#, options: .regularExpression) != nil {
                out.append(Finding(file: path, rule: "a painted background"))
            }
            // invariant: a page holds one primary action at most.
            if path.hasPrefix("template/") {
                let primaries = text.components(separatedBy: "\n").filter { $0.range(of: #"role: [^,)]*\.primary"#, options: .regularExpression) != nil }.count
                if primaries > 1 { out.append(Finding(file: path, rule: "\(primaries) primary actions on one page")) }
            }
            if path.hasPrefix("page/") || path.hasPrefix("engine/") {
                for line in text.split(separator: "\n") where line.hasPrefix("import ") && line != "import Foundation" {
                    out.append(Finding(file: path, rule: "\(line): Page and Engine import Foundation only"))
                }
            }
        }
        return out
    }

    static let drawnByTheKit = ["Popover(", "PageView(", "MenuBarIconView(", "MenuPage.", "HomeContent(", "MachineContent(", "StoryView("]
    static let engine = ["Client.", "UnixSocket.", "EnginePath.", "ActionRunner(", "Navigation(", "Terminal."]

    static func scanCatalog(_ files: [(path: String, text: String)]) -> [Finding] {
        var out: [Finding] = []
        for (path, text) in files {
            if text.contains("Color(") || text.range(of: #"Color\.(red|green|blue|yellow|orange|black|white|gray)"#, options: .regularExpression) != nil {
                out.append(Finding(file: path, rule: "a colour outside Token"))
            }
            for word in drawnByTheKit where text.contains(word) {
                out.append(Finding(file: path, rule: "\(word.dropLast()) drawn by the Catalog, not through StoryStage"))
            }
            for word in engine where text.contains(word) {
                out.append(Finding(file: path, rule: "\(word.dropLast()): the Catalog reaches no engine"))
            }
        }
        return out
    }

    static func swiftFiles(_ root: URL) -> [(path: String, text: String)] {
        FileManager.default.enumerator(at: root, includingPropertiesForKeys: nil)!.compactMap { $0 as? URL }
            .filter { $0.pathExtension == "swift" }
            .map { (String($0.path.dropFirst(root.path.count + 1)), try! String(contentsOf: $0, encoding: .utf8)) }
    }

    func testTheCatalogDrawsOnlyThroughTheKitAndIsNeverBundled() throws {
        let files = Self.swiftFiles(Tree.root.appendingPathComponent("apps/menubar/sources/station-catalog"))
        XCTAssertGreaterThan(files.count, 2)
        XCTAssertEqual(Self.scanCatalog(files), [])
        let package = try String(contentsOf: Tree.root.appendingPathComponent("apps/menubar/Package.swift"), encoding: .utf8)
        let app = try XCTUnwrap(package.split(separator: "\n").first { $0.contains("name: \"StationMenuBar\"") })
        XCTAssertFalse(app.contains("StationCatalog"), "Station.app links the Catalog")
        let bundle = try String(contentsOf: Tree.root.appendingPathComponent("apps/menubar/scripts/build.sh"), encoding: .utf8)
        XCTAssertFalse(bundle.contains("StationCatalog"), "the bundle carries the Catalog")
    }

    func testTheCatalogScanCatchesWhatItForbids() {
        let caught = Self.scanCatalog([
            ("Canvas.swift", "Popover(state).background(Color.blue)"),
            ("Live.swift", "await Client.call(path: socket, name: name)"),
        ]).map(\.rule)
        XCTAssertEqual(caught, ["a colour outside Token", "Popover drawn by the Catalog, not through StoryStage", "Client: the Catalog reaches no engine"])
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
            ("token/Token.swift", "static let mini = \"macmini\""),
            ("template/Panel.swift", "x.background(palette.surface).glassEffect(.regular)"),
            ("molecule/Field.swift", "x.background(.quinary).background(.ultraThinMaterial)"),
            ("template/Many.swift", "a.background(Token.Fill.tile(scheme))\nStationButton(role: .primary)\nStationButton(role: x ? .primary : .plain)"),
            ("molecule/Surface.swift", "Toggle().toggleStyle(.switch); x.background(.quinary)"),
        ]).map(\.rule)
        XCTAssertEqual(caught, [
            "a colour outside Token", "ProgressView is not drawn offscreen",
            "a symbol name outside Token", "a glyph outside Token",
            "import SwiftUI: Page and Engine import Foundation only",
            "macmini draws a Mac's model, which no capability reports",
            "glassEffect in the content", "a painted background",
            "Material in the content", ".quinary outside the kit's surface",
            "Token.Fill.tile outside the kit's surface", "2 primary actions on one page",
        ])
    }
}
