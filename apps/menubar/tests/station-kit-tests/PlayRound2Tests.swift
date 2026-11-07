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

    // invariant: SPEC §10.2 Ports: a port that needs you is said in red on
    // top, even when nothing is reachable outside Tailscale undeclared: a
    // public port declared whose tunnel is not seen.
    func testPortsSayWhatNeedsYouOnTop() throws {
        let page = try Page.decode(Data(#"{"schema":"station.page/v1","id":"menubar.ports","sections":[{"id":"public","title":"Public: reachable from anywhere else","columns":[{"key":"machine","kind":"text"},{"key":"port","kind":"port"},{"key":"class","kind":"text"},{"key":"what","kind":"text"},{"key":"bind","kind":"list"},{"key":"declared","kind":"text"}],"items":[{"row":{"id":"infra:443","mark":"attention","label":"infra","cells":[{"kind":"port","n":443},{"kind":"text","text":"public"},{"kind":"text","text":"nginx"},{"kind":"list","items":[]},{"kind":"text","text":"declared, but no tunnel seen on infra"}],"reason":"degraded"}},{"row":{"id":"infra:80","mark":"ok","label":"infra","cells":[{"kind":"port","n":80},{"kind":"text","text":"public"},{"kind":"text","text":"nginx"},{"kind":"list","items":[]},{"kind":"text","text":"tunnel seen"}]}}]}]}"#.utf8))
        let needs = PortsPage.needing(page)
        XCTAssertEqual(needs.map(\.title), ["infra\(Token.Glyph.sep)443"])
        XCTAssertEqual(needs.map(\.words), ["nginx\(Token.Glyph.sep)declared, but no tunnel seen on infra"])
        let reachable = try Page.decode(Data(#"{"schema":"station.page/v1","id":"menubar.ports","sections":[{"id":"needs-you","title":"Needs you: reachable outside Tailscale, not declared public","items":[{"row":{"id":"n","mark":"broken","label":"mac-mini","cells":[]}}]},{"id":"public","items":[{"row":{"id":"a","mark":"attention","label":"infra","cells":[]}}]}]}"#.utf8))
        XCTAssertEqual(PortsPage.needing(reachable).count, 0)
    }

    // invariant: while a server restarts, its health checks wait for the
    // boot that follows: none is drawn green from the last boot, and the
    // address it checks stays named.
    func testHealthWaitsWhileTheServerRestarts() throws {
        let page = try Page.decode(Data(#"{"schema":"station.page/v1","id":"menubar.machine","sections":[{"id":"health","title":"Health","items":[{"row":{"id":"running","mark":"ok","label":"Running","cells":[{"kind":"none"}]}},{"row":{"id":"address","mark":"ok","label":"Has an address","cells":[{"kind":"text","text":"192.168.64.5"}]}},{"row":{"id":"check","mark":"ok","label":"Check passes","cells":[{"kind":"list","items":[{"kind":"text","text":"https://infra:6443/readyz"},{"kind":"text","text":"200"}]}]}}]}]}"#.utf8))
        let health = try XCTUnwrap(page.section("health"))
        XCTAssertEqual(MachineContent.checks(health, restarting: false), health.rows)
        let waiting = MachineContent.checks(health, restarting: true)
        XCTAssertEqual(waiting.map(\.mark), [.pending, .pending, .pending])
        XCTAssertEqual(waiting.map { $0.cells?.first.map { Format.show($0) } ?? "" }, ["", "", "https://infra:6443/readyz"])
        XCTAssertEqual(MachineContent.doing("restart"), "Restarting")
    }
}
