import CoreImage.CIFilterBuiltins
import SwiftUI

public struct MarkView: View {
    let mark: Mark?
    @Environment(\.stationPalette) private var palette

    public init(_ mark: Mark?) { self.mark = mark }

    public var body: some View {
        if let mark {
            if mark == .pending {
                PendingDot()
            } else {
                Image(systemName: Token.Symbol.of(mark))
                    .font(.system(size: Token.Space.mark))
                    .foregroundStyle(palette.of(mark))
                    .frame(width: Token.Space.mark + 2)
            }
        }
    }
}

public struct PendingDot: View {
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationStill) private var frozen
    let pinned: Bool

    public init(still: Bool = false) { pinned = still }

    var still: Bool { pinned || frozen }

    public var body: some View {
        Image(systemName: Token.Symbol.pending)
            .font(.system(size: Token.Space.mark))
            .foregroundStyle(palette.muted)
            .symbolEffect(.rotate, isActive: !still)
            .symbolEffect(.pulse, isActive: !still)
            .frame(width: Token.Space.mark + 2)
    }
}

public struct ValueText: View {
    let value: Value?
    let muted: Bool
    let size: CGFloat
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationNow) private var now

    public init(_ value: Value?, muted: Bool = false, size: CGFloat = Token.Space.text) {
        self.value = value
        self.muted = muted
        self.size = size
    }

    public var body: some View {
        if let value {
            if value.kind == "pending" {
                PendingDot()
            } else {
                let text = Text(Format.show(value, now: now))
                    .font(.system(size: size))
                    .contentTransition(.numericText())
                    .foregroundStyle(muted || value.kind == "command" || value.stale == true ? palette.muted : palette.text)
                    .lineLimit(1)
                    .truncationMode(value.kind == "path" ? .middle : .tail)
                if Format.numeric(value) { text.monospacedDigit() } else { text }
            }
        }
    }
}

public struct Meter: View {
    let fraction: Double
    @Environment(\.stationPalette) private var palette

    public init(_ fraction: Double) { self.fraction = min(max(fraction, 0), 1) }

    public var body: some View {
        ZStack(alignment: .leading) {
            Capsule().fill(.fill.tertiary)
            Capsule().fill(palette.accent).frame(width: Token.Space.meterWidth * fraction)
        }
        .frame(width: Token.Space.meterWidth, height: Token.Space.meterHeight)
    }
}

// value: a measure's sparkline, as wide as it is given: Swift Charts'
// catmull-rom line over an area fading down, its axes hidden.
public struct Spark: View {
    let points: [Int]
    let h: CGFloat
    let tint: Color?
    @Environment(\.stationPalette) private var palette

    public init(_ points: [Int], height: CGFloat = Token.Space.sparkHeight, tint: Color? = nil) {
        self.points = points
        h = height
        self.tint = tint
    }

    public var body: some View {
        Graph(points, height: h, tint: tint, fit: true)
    }
}

// invariant: a switch as macOS draws it: green when on, its knob white.
public struct Switch: View {
    let on: Bool
    @Environment(\.stationPalette) private var palette

    public init(on: Bool) { self.on = on }

    public var body: some View {
        ZStack(alignment: on ? .trailing : .leading) {
            Capsule().fill(on ? AnyShapeStyle(palette.good) : AnyShapeStyle(.fill.tertiary)).frame(width: 38, height: 22)
            Circle().fill(palette.onTint).frame(width: 18, height: 18).shadow(color: .black.opacity(0.25), radius: 1.5, y: 1).padding(.horizontal, 2)
        }
        .animation(.snappy, value: on)
    }
}

public enum QR {
    public static func modules(_ text: String) -> CGImage? {
        let filter = CIFilter.qrCodeGenerator()
        filter.message = Data(text.utf8)
        filter.correctionLevel = "M"
        guard let image = filter.outputImage else { return nil }
        return CIContext().createCGImage(image, from: image.extent)
    }
}

public struct QRView: View {
    let text: String
    @Environment(\.stationPalette) private var palette

    public init(_ text: String) { self.text = text }

    public var body: some View {
        if let image = QR.modules(text) {
            let quiet: CGFloat = 4
            let scale = max(1, (Token.Space.qr / (CGFloat(image.width) + quiet * 2)).rounded(.down))
            Image(decorative: image, scale: 1)
                .interpolation(.none)
                .resizable()
                .frame(width: CGFloat(image.width) * scale, height: CGFloat(image.height) * scale)
                .padding(quiet * scale)
                .background(palette.qrLight)
        }
    }
}
