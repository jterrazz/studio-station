import StationKit
import SwiftUI
import XCTest

@MainActor
final class IconTests: XCTestCase {
    func icon(_ fixture: String, _ connection: Connection = .live, sparkline: Bool = false) throws -> MenuBarIcon {
        MenuBarIcon.of(connection: connection, page: try Tree.page(fixture), sparkline: sparkline)
    }

    func drawn(_ name: String, _ icon: MenuBarIcon) throws {
        for scheme in [ColorScheme.dark, .light] {
            XCTAssertNil(try Golden.check("icon." + name, AnyView(MenuBarIconView(icon, scheme: scheme)), scheme: scheme, desk: false))
        }
    }

    // invariant: MB-01: nothing needs you on This Mac alone: the symbol
    // alone, tinted by macOS.
    func testIconPlain() throws {
        let icon = try icon("menubar.home.one-mac")
        XCTAssertEqual(icon, MenuBarIcon(.plain))
        XCTAssertEqual(icon.label, "station: nothing needs you")
        XCTAssertTrue(icon.template)
        try drawn("plain", icon)
    }

    // invariant: MB-02: five findings and no server down: the orange 5.
    func testIconOrangeCount() throws {
        let icon = try icon("menubar.home.two-macs")
        XCTAssertEqual(icon, MenuBarIcon(.needs(5)))
        XCTAssertEqual(icon.label, "station: 5 things need you")
        XCTAssertFalse(icon.template)
        try drawn("needs", icon)
    }

    // invariant: MB-03: infra in a loop on the mini: the red !, whatever
    // the count beside it.
    func testIconRedServerDown() throws {
        let page = try Tree.page("menubar.home.infra-down")
        XCTAssertEqual(page.badge?.needs, 5)
        let icon = try icon("menubar.home.infra-down")
        XCTAssertEqual(icon, MenuBarIcon(.down))
        XCTAssertEqual(icon.label, "station: a server is down")
        try drawn("down", icon)
    }

    // invariant: MB-04: the CPU line is the owner's option, This Mac's CPU
    // from the page's strip, beside the symbol and its badge.
    func testIconCPUSpark() throws {
        let spark = try XCTUnwrap(Tree.page("menubar.home.two-macs").strip?.spark)
        XCTAssertFalse(spark.isEmpty)
        let icon = try icon("menubar.home.two-macs", sparkline: true)
        XCTAssertEqual(icon, MenuBarIcon(.needs(5), spark: spark))
        try drawn("spark", icon)
        XCTAssertEqual(try self.icon("menubar.home.engine-stopped", .stopped, sparkline: true).spark, nil)
    }

    // invariant: EN-10, MB-11: the engine stopped dims the symbol and says
    // so; a connection still being made says that, with no badge either way.
    func testIconEngineStopped() throws {
        let stopped = try icon("menubar.home.engine-stopped", .stopped)
        XCTAssertEqual(stopped, MenuBarIcon(.stopped))
        XCTAssertEqual(stopped.label, "station: engine stopped")
        XCTAssertEqual(try icon("menubar.home.two-macs", .notInstalled), MenuBarIcon(.stopped))
        XCTAssertEqual(try icon("menubar.home.infra-down", .connecting).label, "station: connecting")
        XCTAssertEqual(MenuBarIcon.of(connection: .live, page: try Tree.page("menubar.loading")), MenuBarIcon(.plain))
        try drawn("stopped", stopped)
    }

    // invariant: a connection being made dims the Mac and its ring and
    // keeps the two machines lit, with no badge.
    func testIconConnecting() throws {
        let icon = try icon("menubar.home.two-macs", .connecting)
        XCTAssertEqual(icon, MenuBarIcon(.connecting))
        XCTAssertTrue(icon.template)
        try drawn("connecting", icon)
    }

    // invariant: the mark is the Orbit, a Mac inside a ring, never a
    // bubble again: across its middle row and its middle column, at 1x and
    // at 2x, the ink crosses the ring, the Mac, the hole inside it, the Mac
    // and the ring, four strokes with its centre left empty.
    func testTheMarkIsAMacInsideARing() throws {
        for scale in [1, 2] as [CGFloat] {
            let image = try XCTUnwrap(MenuBarIcon(.plain).image(scheme: .light, scale: scale))
            let bitmap = try XCTUnwrap(Bitmap(image))
            let x0 = Int(2 * scale), y0 = Int(3 * scale), side = Int(Token.Icon.symbol * scale)
            func inked(_ x: Int, _ y: Int) -> Bool { bitmap.pixels[(y * bitmap.width + x) * 4 + 3] > 110 }
            func strokes(_ at: (Int) -> Bool) -> Int {
                var n = 0, was = false
                for i in 0..<side {
                    let now = at(i)
                    if now && !was { n += 1 }
                    was = now
                }
                return n
            }
            let mid = side / 2
            XCTAssertEqual(strokes { inked(x0 + $0, y0 + mid) }, 4, "middle row at \(scale)x")
            XCTAssertEqual(strokes { inked(x0 + mid, y0 + $0) }, 4, "middle column at \(scale)x")
            XCTAssertFalse(inked(x0 + mid, y0 + mid), "the Mac's hole at \(scale)x")
        }
    }
}
