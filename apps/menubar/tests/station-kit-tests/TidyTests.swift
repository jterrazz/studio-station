@testable import StationKit
import SwiftUI
import XCTest

@MainActor
final class TidyTests: XCTestCase {
    static func home(linux: String, api: String, extra: String = "") throws -> Page {
        func vm(_ name: String, _ lifetime: String, _ state: String) -> String {
            state == "stopped"
                ? #"{"row":{"id":"\#(name)","mark":"absent","label":"\#(name)","cells":[{"kind":"text","text":"Workspace"},{"kind":"list","items":[{"kind":"text","text":"\#(lifetime)"},{"kind":"text","text":"Stopped"}]}],"reason":"stopped"}}"#
                : #"{"row":{"id":"\#(name)","mark":"ok","label":"\#(name)","cells":[{"kind":"text","text":"Workspace"},{"kind":"list","items":[{"kind":"text","text":"\#(lifetime)"}]}]}}"#
        }
        let json = #"{"schema":"station.page/v1","id":"menubar","sections":[{"id":"machines","title":"Machines","items":[{"group":{"head":{"id":"jterrazz-macbook-pro","mark":"ok","label":"This Mac","cells":[{"kind":"text","text":"Mac"}]},"items":[{"group":{"head":{"id":"jterrazz-macbook-pro/workspaces","label":"Workspaces"},"items":[\#(extra)\#(vm("linux-1", "Throwaway", linux)),\#(vm("api", "Permanent", api))]}}]}},{"group":{"head":{"id":"jterrazz-mac-mini","mark":"ok","label":"mac-mini","cells":[{"kind":"text","text":"Mac"}]},"items":[{"group":{"head":{"id":"jterrazz-mac-mini/workspaces","label":"Workspaces"},"items":[\#(vm("web", "Permanent", "running"))]}},{"group":{"head":{"id":"jterrazz-mac-mini/servers","label":"Servers"},"items":[{"row":{"id":"infra","mark":"ok","label":"infra","cells":[{"kind":"text","text":"Server"},{"kind":"list","items":[{"kind":"text","text":"Always on"}]}]}}]}}]}}]}]}"#
        return try Page.decode(Data(json.utf8))
    }

    // invariant: Escape on a machine's page opened from Home (it says
    // All machines) goes back to Home, never closes the panel; on Home it
    // closes it; a sheet open takes it.
    func testEscapeOnAMachinePageOpenedFromHomeGoesBackToHome() throws {
        let home = try Self.home(linux: "running", api: "stopped")
        let linux = try Page.decode(Data(#"{"schema":"station.page/v1","id":"menubar.machine","context":{"machine":"linux-1","kind":"workspace"},"sections":[{"id":"machine","title":"linux-1"}]}"#.utf8))
        var state = PopoverState(page: home, connection: .live)
        XCTAssertEqual(BackKey.gesture(state), .close)
        state.pushed = linux
        XCTAssertNil(state.back)
        XCTAssertEqual(BackKey.gesture(state), .back)
        state.confirming = "stop"
        XCTAssertNil(BackKey.gesture(state))
    }

    // invariant: a section header upper-cases its words, never a value:
    // an age, a figure and its unit keep the case they are said in.
    func testASectionHeaderKeepsTheCaseOfItsValues() {
        XCTAssertEqual(SectionHeader.cased("Health\(Token.Glyph.sep)after every boot\(Token.Glyph.sep)57 s ago"), "HEALTH\(Token.Glyph.sep)AFTER EVERY BOOT\(Token.Glyph.sep)57 s ago")
        XCTAssertEqual(SectionHeader.cased("Over the last 1 h"), "OVER THE LAST 1 h")
        XCTAssertEqual(SectionHeader.cased("Private\(Token.Glyph.sep)shared on Tailscale\(Token.Glyph.sep)2"), "PRIVATE\(Token.Glyph.sep)SHARED ON TAILSCALE\(Token.Glyph.sep)2")
        XCTAssertEqual(SectionHeader.cased("linux-1"), "LINUX-1")
    }

    // invariant: a machine's line keeps its identity when a VM is inserted
    // above it: the api line is the same line before and after, so it is
    // never drawn for a moment with the icon of the line it replaced.
    func testAMachineLineKeepsItsIdentityWhenAVMIsInserted() throws {
        let before = try Self.home(linux: "running", api: "stopped")
        let after = try Self.home(linux: "running", api: "stopped", extra: #"{"row":{"id":"new-vm","mark":"pending","label":"new-vm","cells":[{"kind":"text","text":"Workspace"}]}},"#)
        func keys(_ p: Page) throws -> [String] {
            MachineTree.keyed(MachineTree(section: try XCTUnwrap(p.section("machines")), kinds: false, only: nil, open: { _ in }).lines()).map(\.key)
        }
        let was = try keys(before), now = try keys(after)
        XCTAssertEqual(Set(now).count, now.count)
        XCTAssertEqual(Set(was).count, was.count)
        let wasAt = try XCTUnwrap(was.firstIndex(of: "row:api")), nowAt = try XCTUnwrap(now.firstIndex(of: "row:api"))
        XCTAssertNotEqual(wasAt, nowAt)
        XCTAssertTrue(now.contains("row:new-vm"))
    }
}
