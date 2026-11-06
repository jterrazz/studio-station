@testable import StationKit
import SwiftUI
import XCTest

// invariant: L50c, the owner's references taken literally: one panel,
// a card only for a top-level group, and nothing inside a card a bubble.
@MainActor
final class FlatTests: XCTestCase {
    func bitmap(_ view: some View) throws -> Bitmap {
        let themed = view.stationTheme(.dark).environment(\.stationNow, Snapshot.now).environment(\.stationStill, true)
        let renderer = ImageRenderer(content: themed)
        renderer.scale = 1
        return try XCTUnwrap(renderer.cgImage.flatMap(Bitmap.init))
    }

    func lit(_ b: Bitmap, row y: Int) -> Int {
        (0..<b.width).filter { b.pixels[(y * b.width + $0) * 4 + 3] != 0 }.count
    }

    func testACardNeverHoldsACard() throws {
        let alone = try bitmap(GlassCard { Color.clear.frame(width: 80, height: 40) })
        let nested = try bitmap(GlassCard { Color.clear.frame(width: 80, height: 40).stationGlass() })
        let deeper = try bitmap(GlassCard { GlassCard(padding: 0) { Color.clear.frame(width: 80, height: 40) } })
        XCTAssertEqual(nested.pixels, alone.pixels)
        XCTAssertEqual(deeper.pixels, alone.pixels)
        let open = try bitmap(Color.clear.frame(width: 80, height: 40).stationGlass())
        XCTAssertGreaterThan(lit(open, row: 20), 0)
    }

    func testARowPaintsNoGround() throws {
        let home = try Tree.page("menubar.home.two-macs")
        let mac = try XCTUnwrap(home.section("machines")?.items?.first?.group?.head)
        let vm = try Tree.page("menubar.vm.linux-1")
        let inside = try XCTUnwrap(vm.section("inside")?.rows.first)
        let rows: [(String, AnyView)] = [
            ("machine line", AnyView(MachineLine(row: mac, symbol: Token.Symbol.mac, capsule: false, open: { _ in }))),
            ("card row", AnyView(CardRow(row: inside, rule: false, actions: [], busy: false) { _ in })),
            ("place row", AnyView(PlaceRow(symbol: Token.Symbol.port, title: "Ports", words: nil, dim: false) {} trailing: { Chevron() })),
            ("menu line", AnyView(MenuText(symbol: Token.Symbol.plus, title: "New VM", key: nil, danger: false))),
        ]
        for (name, row) in rows {
            let b = try bitmap(row.frame(width: 300))
            XCTAssertGreaterThan(b.height, 10, name)
            XCTAssertEqual(lit(b, row: 0), 0, "\(name) paints a ground under itself")
            XCTAssertEqual(lit(b, row: b.height - 1), 0, "\(name) paints a ground under itself")
        }
    }
}
