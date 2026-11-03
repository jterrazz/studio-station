import StationKit
import SwiftUI
import XCTest

@MainActor
final class RenderTests: XCTestCase {
    let update = ProcessInfo.processInfo.environment["STATION_UPDATE_GOLDENS"] == "1"

    func check(_ name: String, _ view: AnyView, scheme: ColorScheme) throws -> String? {
        let file = "\(name).\(scheme == .dark ? "dark" : "light")"
        let golden = Tree.goldens.appendingPathComponent(file + ".png")
        guard let (png, bitmap) = Snapshot.render(view, scheme: scheme) else { return "\(file): no image rendered" }
        if bitmap.placeholder >= 20 { return "\(file): \(bitmap.placeholder) pixels of the unsupported placeholder" }
        if update {
            try png.write(to: golden)
            return nil
        }
        guard let data = try? Data(contentsOf: golden), let want = Bitmap(png: data) else { return "\(file): no golden (STATION_UPDATE_GOLDENS=1 swift test writes it)" }
        let actual = Tree.goldens.appendingPathComponent(file + ".actual.png")
        guard want.width == bitmap.width, want.height == bitmap.height else {
            try png.write(to: actual)
            return "\(file): \(bitmap.width)x\(bitmap.height), golden \(want.width)x\(want.height)"
        }
        let differing = bitmap.differing(want)
        if Double(differing) > Double(bitmap.width * bitmap.height) * 0.001 {
            try png.write(to: actual)
            return "\(file): \(differing) pixels differ"
        }
        try? FileManager.default.removeItem(at: actual)
        return nil
    }

    func testEveryFixtureMatchesItsGolden() throws {
        var failures: [String] = []
        for (name, data) in try Tree.fixtures() {
            let page = try Page.decode(data)
            if let f = try check(name, Snapshot.view(name, page), scheme: .dark) { failures.append(f) }
            if page.id == "menubar", let f = try check(name, Snapshot.view(name, page), scheme: .light) { failures.append(f) }
        }
        XCTAssertEqual(failures, [])
    }
}
