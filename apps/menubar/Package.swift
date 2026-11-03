// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "StationMenuBar",
    platforms: [.macOS(.v14)],
    targets: [
        .target(name: "StationKit", path: "sources/station-kit"),
        .executableTarget(name: "StationMenuBar", dependencies: ["StationKit"], path: "sources/station-menu-bar"),
        .testTarget(name: "StationKitTests", dependencies: ["StationKit"], path: "tests/station-kit-tests", exclude: ["testdata"]),
    ],
    swiftLanguageModes: [.v5]
)
