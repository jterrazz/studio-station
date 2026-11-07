@testable import StationKit
import SwiftUI
import XCTest

@MainActor
final class PlayRound2Tests: XCTestCase {
    // invariant: a column of sizes says every figure in the unit of its
    // largest, as the CLI's: 4.4 GB, 2.0 GB, 0.6 GB, never 644.2 MB under
    // them.
    func testAColumnOfSizesSaysOneUnit() {
        XCTAssertEqual(Format.column([4_402_341_478, 2_040_109_465, 644_245_094]), ["4.4 GB", "2.0 GB", "0.6 GB"])
        XCTAssertEqual(Format.column([900_000, 12_000]), ["900.0 KB", "12.0 KB"])
        XCTAssertEqual(Format.column([]), [])
    }

    // invariant: Top processes says its memory column in one unit.
    func testTopProcessesSayMemoryInOneUnit() throws {
        let page = try Page.decode(Data(#"{"schema":"station.page/v1","id":"menubar.activity","sections":[{"id":"processes","title":"Top processes","columns":[{"key":"name","kind":"text"},{"key":"what","kind":"text"},{"key":"cpu","kind":"percent"},{"key":"memory","kind":"size"}],"items":[{"row":{"id":"1","label":"tart","cells":[{"kind":"none"},{"kind":"percent","n":180},{"kind":"size","n":4402341478}]}},{"row":{"id":"2","label":"WindowServer","cells":[{"kind":"none"},{"kind":"percent","n":40},{"kind":"size","n":644245094}]}},{"row":{"id":"3","label":"idle","cells":[{"kind":"none"},{"kind":"percent","n":10},{"kind":"none"}]}}]}]}"#.utf8))
        let top = try XCTUnwrap(page.section("processes"))
        XCTAssertEqual(ActivityPage.memory(top), ["1": "4.4 GB", "2": "0.6 GB"])
    }

    // invariant: Stop everything waits out what the engine's stop does
    // first: each VM's graceful stop (60 s) and its hard stop (10 s).
    func testStopEverythingWaitsOutTheVMsStopping() {
        let runner = ActionRunner(executable: "/u/bin/station", socket: "/nonexistent")
        XCTAssertGreaterThanOrEqual(runner.stopWithin, 70)
    }
}
