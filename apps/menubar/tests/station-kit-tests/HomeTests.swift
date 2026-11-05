import AppKit
import StationKit
import SwiftUI
import XCTest

@MainActor
final class HomeTests: XCTestCase {
    func texts(_ values: [Value]?) -> [String] { (values ?? []).compactMap(\.text) }

    // invariant: EN-10: Stop everything on This Mac asks y/n first, says
    // what stops and that the engine stays stopped, and its yes is the
    // engine.stop call, confirmed, over the socket.
    func testStopEverythingSheet() async throws {
        let page = try Tree.page("menubar.home.two-macs")
        let stop = try XCTUnwrap(page.action("stop-all"))
        let confirm = try XCTUnwrap(stop.confirm)
        XCTAssertEqual(confirm.question, "Stop everything on This Mac?")
        XCTAssertEqual(texts(confirm.lines), [
            "linux-1 will stop; its disk is kept",
            "Awake ends, and the engine stays stopped until station engine start or the next login",
        ])
        XCTAssertEqual(stop.run?.call, ActionRunner.endsTheEngine)
        XCTAssertEqual(stop.run?.in, ["Confirm": "yes", "By": "the menu bar"])
        var state = Snapshot.state("menubar.home.two-macs", page)
        state.confirming = stop.id
        XCTAssertNil(try Golden.check("sheet.stop-everything", AnyView(Popover(state, scrolls: false)), scheme: .dark))
    }

    // invariant: MB-11: the engine stopped, Home says it, with Start the
    // engine, which opens Terminal on station engine start and nothing else.
    func testEngineStoppedHome() async throws {
        let page = try Tree.page("menubar.home.engine-stopped")
        let row = try XCTUnwrap(page.section("todo")?.items?.first?.row)
        XCTAssertEqual(row.label, "engine")
        XCTAssertEqual(texts(row.cells), ["stopped"])
        XCTAssertEqual(page.pageActions.map(\.label), ["Start the engine", "Open station"])
        let calls = Recorded()
        let runner = ActionRunner(executable: "/u/bin/station", socket: "/nonexistent", launcher: RecordingLauncher(calls: calls))
        let status = await runner.run(try XCTUnwrap(page.action("start")))
        XCTAssertNil(status)
        XCTAssertEqual(calls.all, [
            ["/usr/bin/osascript", "-e", #"tell application "Terminal" to do script "'/u/bin/station' 'engine' 'start'""#, "-e", #"tell application "Terminal" to activate"#],
        ])
        XCTAssertEqual(Snapshot.state("menubar.home.engine-stopped", page).connection, .stopped)
    }

    // invariant: MB-12: every snapshot, of a page, a sheet or the icon, is
    // drawn offscreen: rendering them all opens no window.
    func testSnapshotsOffscreen() throws {
        let before = NSApplication.shared.windows.count
        var drawn = 0
        for (name, data) in try Tree.fixtures() {
            let page = try Page.decode(data)
            if Snapshot.render(Snapshot.view(name, page), scheme: .dark) != nil { drawn += 1 }
            if MenuBarIcon.of(connection: .live, page: page).image(scheme: .dark) != nil { drawn += 1 }
        }
        XCTAssertEqual(drawn, try Tree.fixtures().count * 2)
        XCTAssertEqual(NSApplication.shared.windows.count, before)
        XCTAssertEqual(before, 0)
    }
}
