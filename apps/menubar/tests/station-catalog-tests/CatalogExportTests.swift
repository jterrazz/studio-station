import AppKit
import Foundation
import StationKit
import XCTest

final class CatalogExportTests: XCTestCase {
    static let repository = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        .deletingLastPathComponent().deletingLastPathComponent()
    static let pages = repository.appendingPathComponent("src/ui/testdata/pages")

    var root: URL!
    var stories: URL { root.appendingPathComponent("stories") }
    var out: URL { root.appendingPathComponent("out") }

    override func setUpWithError() throws {
        root = FileManager.default.temporaryDirectory.appendingPathComponent("station-catalog-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: stories.appendingPathComponent("pages"), withIntermediateDirectories: true)
        for name in ["menubar.home.two-macs", "menubar.mac.mini"] {
            try FileManager.default.copyItem(at: Self.pages.appendingPathComponent(name + ".json"),
                                             to: stories.appendingPathComponent("pages/\(name).json"))
        }
        var item = StoryUI()
        item.statusItem = true
        for story in [
            StoryFile(id: "home.needs", group: .home, note: "Home with a need on top.", pages: ["pages/menubar.home.two-macs.json"], status: "today"),
            StoryFile(id: "home.fine", group: .home, note: "Home when all is well.", pages: ["pages/menubar.home.two-macs.json"], status: "today"),
            StoryFile(id: "mac.mini.overview", group: .mac, note: "The mini's page.",
                      pages: ["pages/menubar.home.two-macs.json", "pages/menubar.mac.mini.json"], status: "today"),
            StoryFile(id: "menubar.count", group: .shell, note: "The status item with a count.",
                      pages: ["pages/menubar.home.two-macs.json"], status: "today", ui: item),
        ] {
            try story.encoded().write(to: stories.appendingPathComponent(story.id + Stories.suffix))
        }
    }

    override func tearDownWithError() throws {
        try FileManager.default.removeItem(at: root)
    }

    struct Run {
        let code: Int32
        let output: String
        let error: String
    }

    func export(_ arguments: [String]) throws -> Run {
        let binary = Bundle(for: Self.self).bundleURL.deletingLastPathComponent().appendingPathComponent("StationCatalog")
        let process = Process()
        process.executableURL = binary
        process.arguments = arguments
        let output = Pipe(), error = Pipe()
        process.standardOutput = output
        process.standardError = error
        let done = DispatchSemaphore(value: 0)
        process.terminationHandler = { _ in done.signal() }
        try process.run()
        if done.wait(timeout: .now() + 120) == .timedOut {
            process.terminate()
            XCTFail("StationCatalog \(arguments.joined(separator: " ")) did not return: it started the app instead of exporting")
        }
        return Run(code: process.terminationStatus,
                   output: String(decoding: output.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self),
                   error: String(decoding: error.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self))
    }

    func written() throws -> [String] {
        try FileManager.default.contentsOfDirectory(atPath: out.path).sorted()
    }

    func size(_ file: String) throws -> (Int, Int) {
        let rep = try XCTUnwrap(NSBitmapImageRep(data: Data(contentsOf: out.appendingPathComponent(file))), "\(file) is not an image")
        return (rep.pixelsWide, rep.pixelsHigh)
    }

    func testTheExportDrawsEveryStoryTheFilterNamesInDarkAndLightThenReturns() throws {
        let run = try export(["--export", out.path, "--filter", "^home", "--stories", stories.path])
        XCTAssertEqual(run.code, 0, run.error)
        XCTAssertEqual(try written(), ["home.fine.dark.png", "home.fine.light.png", "home.needs.dark.png", "home.needs.light.png"])
        XCTAssertEqual(run.output.split(separator: "\n").map { URL(fileURLWithPath: String($0)).lastPathComponent }.sorted(), try written())
        let (width, height) = try size("home.needs.dark.png")
        XCTAssertEqual(width, Int((Token.Space.width + 2 * StoryStage.room) * 2))
        XCTAssertGreaterThan(height, 200)
    }

    func testTheExportWithoutAFilterDrawsEveryStory() throws {
        let run = try export(["--export", out.path, "--stories", stories.path])
        XCTAssertEqual(run.code, 0, run.error)
        XCTAssertEqual(try written().count, 8)
    }

    func testAShellStoryIsTheStatusItemInAStripOfTheMenuBar() throws {
        let run = try export(["--export", out.path, "--filter", "^menubar", "--stories", stories.path])
        XCTAssertEqual(run.code, 0, run.error)
        XCTAssertEqual(try written(), ["menubar.count.dark.png", "menubar.count.light.png"])
        let (width, height) = try size("menubar.count.light.png")
        XCTAssertEqual(width, Int(StoryStage.strip.width * 2))
        XCTAssertEqual(height, Int(StoryStage.strip.height * 2))
    }

    func testAFilterThatMatchesNothingSaysSoAndWritesNothing() throws {
        let run = try export(["--export", out.path, "--filter", "^nothing", "--stories", stories.path])
        XCTAssertEqual(run.code, 1)
        XCTAssertEqual(run.error, "Error: no story matches ^nothing\n")
        XCTAssertFalse(FileManager.default.fileExists(atPath: out.path))
    }

    @MainActor
    func testTheWidthKnobSetsThePanelsWidth() throws {
        let story = try XCTUnwrap(Stories.load(root: stories).first { $0.id == "home.needs" })
        var widths: [Int] = []
        for width in StoryKnobs.widths {
            let png = try XCTUnwrap(StoryStage.png(story, knobs: StoryKnobs(width: width, scale: 1)))
            widths.append(try XCTUnwrap(NSBitmapImageRep(data: png)).pixelsWide)
        }
        XCTAssertEqual(widths, StoryKnobs.widths.map { Int($0 + 2 * StoryStage.room) })
    }
}
