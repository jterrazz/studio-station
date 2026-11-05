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
            XCTAssertNil(try Golden.check("icon." + name, AnyView(MenuBarIconView(icon, scheme: scheme)), scheme: scheme))
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
}
