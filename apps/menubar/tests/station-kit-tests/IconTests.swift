import StationKit
import XCTest

final class IconTests: XCTestCase {
    func testTheIconFollowsTheTablesOrder() throws {
        let full = try Tree.page("menubar.full")
        let empty = try Tree.page("menubar.empty")
        let loading = try Tree.page("menubar.loading")
        let stopped = try Tree.page("menubar.error")
        var warn = full
        warn.sections = full.sections?.filter { $0.id != "power" }
        warn.sections?[0].counts = [Tally(mark: .attention, n: 3)]
        var broken = warn
        broken.sections?[0].counts = [Tally(mark: .broken, n: 1), Tally(mark: .attention, n: 3)]
        var calm = empty
        calm.sections = empty.sections?.filter { $0.id != "fleet" }
        let cases: [(Connection, Page?, String, String)] = [
            (.connecting, nil, "circle.dashed", "station: connecting"),
            (.connecting, full, "circle.dashed", "station: connecting"),
            (.stopped, stopped, "stop.circle", "station: engine stopped"),
            (.notInstalled, stopped, "stop.circle", "station: engine stopped"),
            (.live, full, "power.circle.fill", "station: awake"),
            (.live, broken, "xmark.circle.fill", "station: 4 to do"),
            (.live, warn, "circle.lefthalf.filled", "station: 3 to do"),
            (.live, calm, "circle.fill", "station: 0 to do"),
            (.live, loading, "circle.fill", "station: 0 to do"),
            (.live, nil, "circle.dashed", "station: 0 to do"),
            (.live, empty, "circle.fill", "station: 0 to do"),
        ]
        for (i, (connection, page, symbol, label)) in cases.enumerated() {
            XCTAssertEqual(MenuBarIcon.symbol(connection: connection, page: page), symbol, "case \(i)")
            XCTAssertEqual(MenuBarIcon.label(connection: connection, page: page), label, "case \(i)")
        }
    }
}
