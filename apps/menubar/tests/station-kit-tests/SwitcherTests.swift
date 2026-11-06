import AppKit
import StationKit
import SwiftUI
import XCTest

@MainActor
final class SwitcherTests: XCTestCase {
    // invariant: MB-06: the switcher is a search, the machines opened last,
    // every machine as a tree, each Mac's dot as Home draws it and a server
    // with its last check, Show all machines ⌘0 and Add a Mac….
    func testSwitcherLayout() throws {
        let page = try Tree.page("menubar.switcher.two-macs")
        let tree = try XCTUnwrap(page.section("machines"))
        XCTAssertEqual(tree.title, "All machines")
        XCTAssertEqual((tree.items ?? []).compactMap(\.group?.head?.label), ["This Mac", "mac-mini"])
        XCTAssertEqual((tree.items ?? []).compactMap { texts($0.group?.head?.cells?.last?.items) }, [["Engine running"], ["Engine running"]])
        XCTAssertEqual((tree.items ?? []).compactMap(\.group?.head?.mark), try XCTUnwrap(Tree.page("menubar.home.two-macs").section("machines")?.items).compactMap(\.group?.head?.mark))
        XCTAssertEqual((tree.items ?? []).last?.group?.head?.mark, .broken)
        let infra = try XCTUnwrap((tree.items ?? []).last?.group?.items?.first?.group?.items?.first?.row)
        XCTAssertEqual(infra.cells?.last.map { Format.show($0) }, "Always on · healthy · checked 40 s ago")
        XCTAssertEqual(page.pageActions.map(\.label), ["Add a Mac"])
        XCTAssertNil(try Golden.check("menubar.switcher.two-macs", AnyView(Popover(Snapshot.state("menubar.switcher.two-macs", page), scrolls: false))))
    }

    func texts(_ values: [Value]?) -> [String] { (values ?? []).compactMap(\.text) }

    // invariant: MB-13: Add a Mac… asks the Mac's name, its Add sends
    // machines.add with that name as typed, then reads the switcher again.
    func testSwitcherAddMac() async throws {
        let page = try Tree.page("menubar.switcher.two-macs")
        let add = try XCTUnwrap(page.action("add-mac"))
        XCTAssertEqual(add.form?.fields?.map(\.key), ["Name"])
        var state = Snapshot.state("menubar.switcher.two-macs", page)
        state.confirming = add.id
        state.answers[add.id ?? ""] = ["Name": "studio-1"]
        XCTAssertNil(try Golden.check("sheet.add-mac", AnyView(Popover(state, scrolls: false))))
        let engine = try FakeEngine("mb-add-mac")
        engine.serveOnce(Data(#"{"v":2,"id":"1","out":{"Name":"studio-1"}}"#.utf8) + [0x0A], thenClose: false)
        engine.serveOnce(Data(#"{"v":2,"id":"1","out":{}}"#.utf8) + [0x0A], thenClose: false)
        defer { engine.stop() }
        let status = await ActionRunner(executable: "/u/bin/station", socket: engine.path, reread: [PageID.switcher]).run(add, answers: ["Name": "studio-1"])
        XCTAssertNil(status)
        XCTAssertEqual(engine.received, [
            #"{"call":"machines.add","id":"1","in":{"Name":"studio-1"},"v":2}"#,
            #"{"call":"engine.refresh","id":"1","in":{"pages":["menubar.switcher"]},"v":2}"#,
        ])
    }
}
