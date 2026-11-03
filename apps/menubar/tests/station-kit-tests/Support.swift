import AppKit
import Foundation
import StationKit
import SwiftUI

enum Tree {
    static let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    static let pages = root.appendingPathComponent("src/ui/testdata/pages")
    static let stream = root.appendingPathComponent("src/ui/testdata/stream/menubar.jsonl")
    static let goldens = URL(fileURLWithPath: #filePath).deletingLastPathComponent().appendingPathComponent("testdata")
    static let sources = root.appendingPathComponent("apps/menubar/sources/station-kit")

    static func fixtures() throws -> [(name: String, data: Data)] {
        try FileManager.default.contentsOfDirectory(at: pages, includingPropertiesForKeys: nil)
            .filter { $0.pathExtension == "json" }
            .sorted { $0.lastPathComponent < $1.lastPathComponent }
            .map { ($0.deletingPathExtension().lastPathComponent, try Data(contentsOf: $0)) }
    }

    static func page(_ name: String) throws -> Page {
        try Page.decode(Data(contentsOf: pages.appendingPathComponent(name + ".json")))
    }
}

struct Bitmap {
    let width: Int
    let height: Int
    let pixels: [UInt8]

    init?(_ image: CGImage) {
        let width = image.width, height = image.height
        self.width = width
        self.height = height
        var buf = [UInt8](repeating: 0, count: width * height * 4)
        let ok = buf.withUnsafeMutableBytes { raw -> Bool in
            guard let ctx = CGContext(data: raw.baseAddress, width: width, height: height, bitsPerComponent: 8, bytesPerRow: width * 4,
                                      space: CGColorSpace(name: CGColorSpace.sRGB)!, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue) else { return false }
            ctx.draw(image, in: CGRect(x: 0, y: 0, width: width, height: height))
            return true
        }
        guard ok else { return nil }
        pixels = buf
    }

    init?(png: Data) {
        guard let rep = NSBitmapImageRep(data: png), let image = rep.cgImage else { return nil }
        self.init(image)
    }

    func differing(_ other: Bitmap) -> Int {
        var n = 0
        for i in stride(from: 0, to: pixels.count, by: 4) {
            for c in 0..<4 where abs(Int(pixels[i + c]) - Int(other.pixels[i + c])) > 16 {
                n += 1
                break
            }
        }
        return n
    }

    var placeholder: Int {
        var n = 0
        for i in stride(from: 0, to: pixels.count, by: 4)
        where abs(Int(pixels[i]) - 255) <= 4 && abs(Int(pixels[i + 1]) - 204) <= 4 && Int(pixels[i + 2]) <= 4 {
            n += 1
        }
        return n
    }
}

enum Snapshot {
    static let now = ISO8601DateFormatter().date(from: "2026-11-03T10:30:00Z")!

    static func state(_ name: String, _ page: Page) -> PopoverState {
        PopoverState(page: page, connection: name.hasSuffix(".error") ? .stopped : .live)
    }

    @MainActor
    static func view(_ name: String, _ page: Page) -> AnyView {
        if page.id == "menubar" { return AnyView(Popover(state(name, page), scrolls: false)) }
        return AnyView(PageView(page))
    }

    @MainActor
    static func render(_ view: AnyView, scheme: ColorScheme) -> (png: Data, bitmap: Bitmap)? {
        let renderer = ImageRenderer(content: view.stationTheme(scheme).environment(\.stationNow, now).environment(\.stationStill, true))
        renderer.scale = 2
        guard let image = renderer.cgImage, let bitmap = Bitmap(image) else { return nil }
        let rep = NSBitmapImageRep(cgImage: image)
        guard let png = rep.representation(using: .png, properties: [:]) else { return nil }
        return (png, bitmap)
    }
}
