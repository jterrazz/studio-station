import Charts
import SwiftUI

// invariant: a measure's graph over its window: its points 0 to 100,
// oldest first, a 1.5 pt catmull-rom line over an area fading to nothing,
// no axes, as wide as it is given.
public struct Graph: View {
    let points: [Int]
    let height: CGFloat
    let tint: Color?
    let fit: Bool
    @Environment(\.stationPalette) private var palette

    public init(_ points: [Int], height: CGFloat = Token.Space.graphHeight, tint: Color? = nil, fit: Bool = false) {
        self.points = points
        self.height = height
        self.tint = tint
        self.fit = fit
    }

    // value: the graph's scale: 0 to 100, or for a sparkline the band its
    // points move in, so a steady series still shows its swing.
    var domain: ClosedRange<Int> {
        guard fit, let lo = points.min(), let hi = points.max() else { return 0...100 }
        let pad = max(hi - lo, 10)
        return max(lo - pad, 0)...min(hi + pad / 2, 100)
    }

    public var body: some View {
        let ink = tint ?? palette.accent
        Chart(Array(points.enumerated()), id: \.offset) { i, p in
            AreaMark(x: .value("t", i), yStart: .value("floor", domain.lowerBound), yEnd: .value("v", min(max(p, 0), 100)))
                .interpolationMethod(.catmullRom)
                .foregroundStyle(LinearGradient(colors: [ink.opacity(0.38), ink.opacity(0)], startPoint: .top, endPoint: .bottom))
            LineMark(x: .value("t", i), y: .value("v", min(max(p, 0), 100)))
                .interpolationMethod(.catmullRom)
                .lineStyle(StrokeStyle(lineWidth: 1.5, lineCap: .round, lineJoin: .round))
                .foregroundStyle(ink)
        }
        .chartXAxis(.hidden)
        .chartYAxis(.hidden)
        .chartLegend(.hidden)
        .chartYScale(domain: domain)
        .chartXScale(domain: 0...max(points.count - 1, 1))
        .chartPlotStyle { $0.frame(maxWidth: .infinity) }
        .frame(height: height)
    }
}

extension Value {
    // value: the points a spark value carries after its lead value.
    public var points: [Int] {
        guard kind == "spark" else { return [] }
        return (items ?? []).dropFirst().map { Int($0.n ?? 0) }
    }
}
