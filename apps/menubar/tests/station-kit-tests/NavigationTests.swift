import AppKit
import StationKit
import SwiftUI
import XCTest

final class Shown: @unchecked Sendable {
    private let lock = NSLock()
    private var list: [(String, String?)] = []
    func add(_ watch: String, _ page: Page?) { lock.withLock { list.append((watch, page?.id)) } }
    var all: [String] { lock.withLock { list.map { "\($0.0) \($0.1 ?? "nil")" } } }
}

enum Streamed {
    // value: what the engine answers a page watch: its opening, then the
    // page of a fixture, on one line each.
    static func answer(_ watch: String, _ fixture: String) throws -> Data {
        let page = try JSONSerialization.jsonObject(with: Data(contentsOf: Tree.pages.appendingPathComponent(fixture + ".json")))
        let opened = try JSONSerialization.data(withJSONObject: ["v": 2, "id": "1", "out": ["stream": "page", "pages": [watch]]], options: [.sortedKeys])
        let event = try JSONSerialization.data(withJSONObject: ["v": 2, "id": "1", "out": ["event": "page", "page": page]], options: [.sortedKeys])
        return opened + [0x0A] + event + [0x0A]
    }

    static func watch(_ page: String) -> String { #"{"id":"1","in":{"pages":[""# + page + #""]},"v":2,"watch":"page"}"# }
}

@MainActor
final class NavigationTests: XCTestCase {
    func until(_ condition: @escaping () -> Bool) async {
        for _ in 0..<500 where !condition() {
            try? await Task.sleep(nanoseconds: 10_000_000)
        }
    }

    // invariant: SPEC §10.3, AW-21, PO-34: Home opens Activity, Awake and
    // Ports, each the engine's watch of that page, drawn in place of Home
    // while it is open; back ends the watch and Home is drawn again.
    func testHomeOpensTheMenuBarsOwnPagesFromTheEngine() async throws {
        XCTAssertEqual(Popover.pages.map(\.label), ["Activity", "Awake", "Ports"])
        for (label, fixture) in [("Activity", "menubar.activity.mac"), ("Awake", "menubar.awake.two-macs"), ("Ports", "menubar.ports.mini")] {
            let watch = try XCTUnwrap(Popover.pages.first { $0.label == label }?.watch)
            let engine = try FakeEngine("mb-nav")
            defer { engine.stop() }
            engine.serveOnce(try Streamed.answer(watch, fixture), thenClose: false)
            let shown = Shown()
            let nav = Navigation(socket: engine.path, clock: TestClock()) { shown.add($0, $1) }
            let handlers = nav.wire(PopoverHandlers())
            handlers.push(watch)
            await until { !shown.all.isEmpty }
            XCTAssertEqual(engine.received, [Streamed.watch(watch)], label)
            let page = try Tree.page(fixture)
            XCTAssertEqual(shown.all, ["\(watch) \(page.id ?? "")"], label)
            let home = try Tree.page("menubar.home.two-macs")
            let state = PanelState(page: home, connection: .live, pushed: page)
            XCTAssertEqual(state.shown?.id, page.id)
            XCTAssertNil(try Golden.check(fixture, AnyView(Popover(state, handlers: handlers, scrolls: false)), scheme: .dark), label)
            handlers.back()
            await until { engine.left }
            XCTAssertTrue(engine.left, "\(label): its watch stayed open after back")
            XCTAssertEqual(shown.all.last, "\(watch) nil", label)
            XCTAssertNil(nav.watched)
        }
    }
}
