import StationKit
import XCTest

final class DecodeTests: XCTestCase {
    func testEveryFixtureDecodesAndSurvivesARoundTrip() throws {
        let fixtures = try Tree.fixtures()
        XCTAssertEqual(fixtures.count, 100)
        for (name, data) in fixtures {
            let page = try Page.decode(data)
            XCTAssertEqual(try Page.decode(page.encoded()), page, name)
        }
    }

    func testADocumentOfAnotherSchemaIsRefused() {
        XCTAssertThrowsError(try Page.decode(Data(#"{"schema":"station.status/v1"}"#.utf8)))
    }

    func testUnknownKeysAreIgnored() throws {
        let page = try Page.decode(Data(#"{"schema":"station.page/v1","id":"x","future":{"a":1}}"#.utf8))
        XCTAssertEqual(page.id, "x")
    }
}
