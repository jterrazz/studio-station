import SwiftUI

// invariant: a measure's graph over its window: its points 0 to 100,
// oldest first, a line over a filled area as wide as it is given.
public struct Graph: View {
    let points: [Int]
    @Environment(\.stationPalette) private var palette

    public init(_ points: [Int]) { self.points = points }

    func path(in size: CGSize, closed: Bool) -> Path {
        Path { path in
            guard points.count > 1 else { return }
            let step = size.width / CGFloat(points.count - 1)
            let y = { (p: Int) in size.height - size.height * CGFloat(min(max(p, 0), 100)) / 100 }
            path.move(to: CGPoint(x: 0, y: closed ? size.height : y(points[0])))
            for (i, p) in points.enumerated() {
                path.addLine(to: CGPoint(x: CGFloat(i) * step, y: y(p)))
            }
            if closed {
                path.addLine(to: CGPoint(x: size.width, y: size.height))
                path.closeSubpath()
            }
        }
    }

    public var body: some View {
        GeometryReader { geo in
            ZStack {
                path(in: geo.size, closed: true).fill(LinearGradient(colors: [palette.accent.opacity(0.35), palette.accent.opacity(0.02)], startPoint: .top, endPoint: .bottom))
                path(in: geo.size, closed: false).stroke(palette.accent, style: StrokeStyle(lineWidth: 1.2, lineJoin: .round))
            }
        }
        .frame(height: Token.Space.graphHeight)
    }
}

extension Value {
    // value: the points a spark value carries after its lead value.
    public var points: [Int] {
        guard kind == "spark" else { return [] }
        return (items ?? []).dropFirst().map { Int($0.n ?? 0) }
    }
}
