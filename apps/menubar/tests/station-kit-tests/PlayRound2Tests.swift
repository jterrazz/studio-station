@testable import StationKit
import SwiftUI
import XCTest

@MainActor
final class PlayRound2Tests: XCTestCase {
    // invariant: Stop everything waits out what the engine's stop does
    // first: each VM's graceful stop (60 s) and its hard stop (10 s).
    func testStopEverythingWaitsOutTheVMsStopping() {
        let runner = ActionRunner(executable: "/u/bin/station", socket: "/nonexistent")
        XCTAssertGreaterThanOrEqual(runner.stopWithin, 70)
    }
}
