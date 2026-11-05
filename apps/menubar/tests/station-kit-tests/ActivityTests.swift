import AppKit
import StationKit
import SwiftUI
import XCTest

@MainActor
final class ActivityTests: XCTestCase {
    // invariant: AC-25: a machine's Activity page draws every measure as a
    // graph over the window picked, 1 h unless picked, the picker offering
    // 6 min, 1 h and 24 h; picking one asks for its window and runs nothing.
    func testActivityPagePicker() throws {
        let page = try Tree.page("menubar.activity.mac")
        XCTAssertEqual(ActivityPage.picked(page), "1 h")
        XCTAssertEqual(ActivityPage.windows.map(\.words), ["6 min", "1 h", "24 h"])
        let graphs = try XCTUnwrap(page.section("graphs"))
        XCTAssertEqual(graphs.rows.filter { ($0.cells?.last?.points.count ?? 0) > 1 }.map(\.label),
                       ["CPU", "Memory", "Swap", "Disk read", "Disk write", "Net in", "Net out"])
        XCTAssertEqual(page.section("processes")?.rows.map(\.label), ["Virtualization", "Zed", "WindowServer"])
        XCTAssertEqual(page.actions ?? [], [])
        var picked: [String] = []
        let view = ActivityPage(Snapshot.state("menubar.activity.mac", page), handlers: PopoverHandlers(window: { picked.append($0) }))
        XCTAssertNotNil(Snapshot.render(AnyView(view), scheme: .dark))
        XCTAssertEqual(picked, [])
    }
}
