import AppKit
import StationKit
import SwiftUI
import XCTest

@MainActor
final class ActivityTests: XCTestCase {
    func until(_ condition: @escaping () -> Bool) async {
        for _ in 0..<500 where !condition() {
            try? await Task.sleep(nanoseconds: 10_000_000)
        }
    }

    // invariant: AC-25: a machine's Activity page draws every measure as a
    // graph over the window picked, 1 h unless picked, the picker offering
    // 6 min, 1 h and 24 h; picking 6 min makes the engine stream This Mac's
    // activity over 6 min in its place, and runs nothing.
    func testActivityPagePicker() async throws {
        let page = try Tree.page("menubar.activity.mac")
        XCTAssertEqual(ActivityPage.picked(page), "1 h")
        let graphs = try XCTUnwrap(page.section("graphs"))
        XCTAssertEqual(graphs.rows.filter { ($0.cells?.last?.points.count ?? 0) > 1 }.map(\.label),
                       ["CPU", "Memory", "Swap", "Disk read", "Disk write", "Net in", "Net out"])
        XCTAssertEqual(page.section("processes")?.rows.map(\.label), ["Virtualization", "Zed", "WindowServer"])
        XCTAssertEqual(page.actions ?? [], [])

        let engine = try FakeEngine("mb-pick")
        defer { engine.stop() }
        engine.serveOnce(try Streamed.answer("menubar.activity", "menubar.activity.mac"), thenClose: false)
        engine.serveOnce(try Streamed.answer("menubar.activity:6m", "menubar.activity.mac"), thenClose: false)
        let shown = Shown()
        let nav = Navigation(socket: engine.path, clock: TestClock()) { shown.add($0, $1) }
        let handlers = nav.wire(PopoverHandlers())
        handlers.push(Navigation.activity)
        await until { shown.all.count == 1 }
        let picker = ActivityPage.picker(page, handlers: handlers)
        XCTAssertEqual(picker.map(\.words), ["6 min", "1 h", "24 h"])
        XCTAssertEqual(picker.map(\.picked), [false, true, false])
        picker[0].tap()
        await until { shown.all.count == 2 }
        XCTAssertEqual(engine.received, [Streamed.watch("menubar.activity"), Streamed.watch("menubar.activity:6m")])
        XCTAssertEqual(shown.all, ["menubar.activity menubar.activity", "menubar.activity:6m menubar.activity"])
        XCTAssertEqual(nav.watched, "menubar.activity:6m")
        handlers.back()
    }
}
