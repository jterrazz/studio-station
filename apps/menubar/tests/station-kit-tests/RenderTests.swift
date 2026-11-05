import StationKit
import SwiftUI
import XCTest

@MainActor
final class RenderTests: XCTestCase {
    func testEveryFixtureMatchesItsGolden() throws {
        var failures: [String] = []
        for (name, data) in try Tree.fixtures() {
            let page = try Page.decode(data)
            if let f = try Golden.check(name, Snapshot.view(name, page), scheme: .dark) { failures.append(f) }
            if page.id == "menubar" || MenuPage.view(Snapshot.state(name, page)) != nil, let f = try Golden.check(name, Snapshot.view(name, page), scheme: .light) { failures.append(f) }
        }
        XCTAssertEqual(failures, [])
    }
}
