import CoreGraphics
import Foundation

// usage: swift scripts/orbit-symbols.swift <symbols.xcassets>, run by
// build.sh and `make menubar-check` before they build
// value: the Orbit as SF Symbols templates, one per state, drawn on the
// template's 100 pt grid against the system's `circle`: its ring has the
// circle's outer edge and stroke in each master, so a weight and a point
// size draw the Orbit as they draw a system symbol.

struct Master {
    let name: String
    let stroke: CGFloat
    let outer: CGFloat
    let bearing: CGFloat
    let column: CGFloat
    let baseline: CGFloat

    var ring: CGFloat { outer - stroke / 2 }
    var unit: CGFloat { ring / 7.15 }
    var bolder: CGFloat { (stroke - 1.296 * unit) / 2 }
}

let weights: [(name: String, column: CGFloat)] = [("Ultralight", 560), ("Regular", 1460), ("Black", 2400)]
let sizes: [(name: String, baseline: CGFloat, bearing: CGFloat, stroke: [CGFloat], outer: [CGFloat])] = [
    ("S", 696, 7, [2.2, 7.48, 14.17], [37, 39.5, 42]),
    ("M", 1126, 9, [2.32, 8.23, 17.24], [47, 49.5, 53]),
    ("L", 1556, 11, [2.47, 9.16, 20.32], [61.5, 64, 68.5]),
]
let masters = sizes.flatMap { size in
    weights.enumerated().map { i, w in
        Master(name: "\(w.name)-\(size.name)", stroke: size.stroke[i], outer: size.outer[i], bearing: size.bearing, column: w.column, baseline: size.baseline)
    }
}

let capHeight: CGFloat = 70.459

enum State: String, CaseIterable {
    case plain = "station.orbit"
    case badge = "station.orbit.badge"
    case slash = "station.orbit.slash"
}

// invariant: every master of a state is drawn by the same commands in the
// same order, never by a boolean of shapes, so the weights interpolate.

func polar(_ degrees: CGFloat, _ r: CGFloat) -> CGPoint {
    CGPoint(x: r * cos(degrees * .pi / 180), y: r * sin(degrees * .pi / 180))
}

func degrees(_ p: CGPoint) -> CGFloat { atan2(p.y, p.x) * 180 / .pi }

func arc(_ p: CGMutablePath, _ r: CGFloat, _ from: CGFloat, _ to: CGFloat, centre: CGPoint = .zero, pieces: Int = 4) {
    let step = (to - from) / CGFloat(pieces)
    let k = 4 / 3 * tan(step * .pi / 720) * r
    for i in 0..<pieces {
        let a = (from + CGFloat(i) * step) * .pi / 180, b = (from + CGFloat(i + 1) * step) * .pi / 180
        let p0 = CGPoint(x: centre.x + r * cos(a), y: centre.y + r * sin(a))
        let p3 = CGPoint(x: centre.x + r * cos(b), y: centre.y + r * sin(b))
        p.addCurve(to: p3, control1: CGPoint(x: p0.x - k * sin(a), y: p0.y + k * cos(a)), control2: CGPoint(x: p3.x + k * sin(b), y: p3.y - k * cos(b)))
    }
}

func circle(_ p: CGMutablePath, _ c: CGPoint, _ r: CGFloat) {
    p.move(to: CGPoint(x: c.x + r, y: c.y))
    arc(p, r, 0, 360, centre: c)
    p.closeSubpath()
}

// value: a piece of the ring between two ends, each end its outer and
// inner angles: equal for a radial end, apart for one cut along a slash.
func ringPiece(_ p: CGMutablePath, _ m: Master, outer: (CGFloat, CGFloat), inner: (CGFloat, CGFloat)) {
    let ro = m.ring + m.stroke / 2, ri = m.ring - m.stroke / 2
    p.move(to: polar(outer.0, ro))
    arc(p, ro, outer.0, outer.1)
    p.addLine(to: polar(inner.1, ri))
    arc(p, ri, inner.1, inner.0)
    p.closeSubpath()
}

func roundedSquare(_ p: CGMutablePath, half: CGFloat, radius: CGFloat, reversed: Bool) {
    let c = half - radius
    let corners: [(CGPoint, CGFloat)] = [(CGPoint(x: c, y: c), 0), (CGPoint(x: -c, y: c), 90), (CGPoint(x: -c, y: -c), 180), (CGPoint(x: c, y: -c), 270)]
    let order = reversed ? Array(corners.reversed()) : corners
    p.move(to: CGPoint(x: half, y: reversed ? c : -c))
    for (centre, a) in order {
        let (from, to) = reversed ? (a + 90, a) : (a, a + 90)
        p.addLine(to: CGPoint(x: centre.x + radius * cos(from * .pi / 180), y: centre.y + radius * sin(from * .pi / 180)))
        arc(p, radius, from, to, centre: centre, pieces: 1)
    }
    p.closeSubpath()
}

struct Geometry {
    let m: Master
    var u: CGFloat { m.unit }
    var s: CGFloat { m.stroke }
    var satellite: CGFloat { 1.85 * u + m.bolder }
    var gap: CGFloat { 2 * asin((satellite + 1.25 * u) / (2 * m.ring)) * 180 / .pi }
    var coreHalf: CGFloat { 3 * u }
    var coreRadius: CGFloat { 1.75 * u }
    let satellites: [CGFloat] = [150, -30]
    var dotAt: CGPoint { CGPoint(x: 6 * u + 1.5 * m.bolder, y: 6 * u + 1.5 * m.bolder) }
    var dot: CGFloat { 2.6 * u + m.bolder }
    var notch: CGFloat { dot + 1.5 * u }
    var band: CGFloat { s / 2 + 1.375 * u }
    let slash = CGPoint(x: -sqrt(0.5), y: sqrt(0.5))
    let side = CGPoint(x: sqrt(0.5), y: sqrt(0.5))

    func core(_ p: CGMutablePath) {
        roundedSquare(p, half: coreHalf + s / 2, radius: coreRadius + s / 2, reversed: false)
        roundedSquare(p, half: coreHalf - s / 2, radius: max(coreRadius - s / 2, 0.5), reversed: true)
    }

    // value: where the edge of the slash's gap, on the given side of it,
    // meets a circle around the centre.
    func cut(_ r: CGFloat, _ sign: CGFloat) -> CGFloat {
        let t = sqrt(r * r - band * band)
        return degrees(CGPoint(x: sign * (band * side.x + t * slash.x), y: sign * (band * side.y + t * slash.y)))
    }

    // value: the Mac cut by the slash's gap into its two corners away from
    // the slash, each drawn from the cut on its top edge round to the cut
    // on its side edge.
    func slashedCore(_ p: CGMutablePath) {
        let c = coreHalf - coreRadius
        let ho = coreHalf + s / 2, hi = coreHalf - s / 2
        let ro = coreRadius + s / 2, ri = max(coreRadius - s / 2, 0.5)
        let d = band * 2.squareRoot()
        let ao = min(max(d - ho, -c + 0.5), c - 0.5), ai = min(max(d - hi, -c + 0.5), c - 0.5)
        for sign in [1, -1] as [CGFloat] {
            func pt(_ x: CGFloat, _ y: CGFloat) -> CGPoint { CGPoint(x: sign * x, y: sign * y) }
            let turn: CGFloat = sign > 0 ? 0 : 180
            p.move(to: pt(ao, ho))
            p.addLine(to: pt(c, ho))
            arc(p, ro, 90 + turn, turn, centre: pt(c, c), pieces: 1)
            p.addLine(to: pt(ho, ao))
            p.addLine(to: pt(hi, ai))
            p.addLine(to: pt(hi, c))
            arc(p, ri, turn, 90 + turn, centre: pt(c, c), pieces: 1)
            p.addLine(to: pt(ai, hi))
            p.closeSubpath()
        }
    }

    func layers(_ state: State) -> (body: CGPath, satellites: CGPath) {
        let body = CGMutablePath(), moons = CGMutablePath()
        let ro = m.ring + s / 2, ri = m.ring - s / 2
        switch state {
        case .plain:
            ringPiece(body, m, outer: (-30 + gap, 150 - gap), inner: (-30 + gap, 150 - gap))
            ringPiece(body, m, outer: (150 + gap, 330 - gap), inner: (150 + gap, 330 - gap))
            core(body)
            for a in satellites { circle(moons, polar(a, m.ring), satellite) }
        case .badge:
            let n = (dotAt.x * dotAt.x + dotAt.y * dotAt.y).squareRoot()
            let clear = acos((ro * ro + n * n - notch * notch) / (2 * ro * n)) * 180 / .pi
            ringPiece(body, m, outer: (45 + clear, 150 - gap), inner: (45 + clear, 150 - gap))
            ringPiece(body, m, outer: (150 + gap, 330 - gap), inner: (150 + gap, 330 - gap))
            core(body)
            for a in satellites { circle(moons, polar(a, m.ring), satellite) }
            circle(moons, dotAt, dot)
        case .slash:
            ringPiece(body, m, outer: (-30 + gap, cut(ro, 1)), inner: (-30 + gap, cut(ri, 1)))
            ringPiece(body, m, outer: (150 + gap, 360 + cut(ro, -1)), inner: (150 + gap, 360 + cut(ri, -1)))
            slashedCore(body)
            let l = 10.25 * u, r = s / 2
            let a = CGPoint(x: -l * slash.x, y: -l * slash.y), b = CGPoint(x: l * slash.x, y: l * slash.y)
            let along = degrees(slash)
            body.move(to: CGPoint(x: a.x + r * side.x, y: a.y + r * side.y))
            body.addLine(to: CGPoint(x: b.x + r * side.x, y: b.y + r * side.y))
            arc(body, r, along - 90, along + 90, centre: b, pieces: 2)
            body.addLine(to: CGPoint(x: a.x - r * side.x, y: a.y - r * side.y))
            arc(body, r, along + 90, along + 270, centre: a, pieces: 2)
            body.closeSubpath()
        }
        return (body, moons)
    }
}

func f(_ v: CGFloat) -> String {
    let r = (v * 1000).rounded() / 1000
    return r == r.rounded() ? String(Int(r)) : String(format: "%.3f", r)
}

func commands(_ path: CGPath) -> String {
    var out = ""
    path.applyWithBlock { out += String(describing: $0.pointee.type.rawValue) }
    return out
}

func svg(_ path: CGPath, dx: CGFloat) -> String {
    let capMiddle = capHeight / 2
    var d = ""
    func p(_ pt: CGPoint) -> String { "\(f(pt.x + dx)) \(f(-(pt.y + capMiddle)))" }
    path.applyWithBlock { e in
        let pts = e.pointee.points
        switch e.pointee.type {
        case .moveToPoint: d += "M\(p(pts[0]))"
        case .addLineToPoint: d += "L\(p(pts[0]))"
        case .addQuadCurveToPoint: d += "Q\(p(pts[0])) \(p(pts[1]))"
        case .addCurveToPoint: d += "C\(p(pts[0])) \(p(pts[1])) \(p(pts[2]))"
        case .closeSubpath: d += "Z"
        @unknown default: break
        }
    }
    return d
}

func template(_ state: State) -> String {
    let line = "style=\"fill:none;stroke:#27AAE1;opacity:1;stroke-width:0.5;\""
    var guides = ""
    var symbols = ""
    for (i, y) in [696, 1126, 1556].enumerated() {
        let tag = ["S", "M", "L"][i]
        guides += "  <line id=\"Baseline-\(tag)\" \(line) x1=\"263\" x2=\"3036\" y1=\"\(y)\" y2=\"\(y)\"/>\n"
        guides += "  <line id=\"Capline-\(tag)\" \(line) x1=\"263\" x2=\"3036\" y1=\"\(f(CGFloat(y) - capHeight))\" y2=\"\(f(CGFloat(y) - capHeight))\"/>\n"
    }
    for m in masters {
        let (body, satellites) = Geometry(m: m).layers(state)
        let shape = commands(body) + "/" + commands(satellites)
        if shape != commands(Geometry(m: masters[0]).layers(state).body) + "/" + commands(Geometry(m: masters[0]).layers(state).satellites) {
            fatalError("\(state.rawValue): \(m.name) is not drawn as \(masters[0].name) is")
        }
        let box = body.boundingBoxOfPath.union(satellites.boundingBoxOfPath)
        let left = m.column
        let dx = m.bearing - box.minX
        let right = left + box.width + 2 * m.bearing
        let top = f(m.baseline - 95), bottom = f(m.baseline + 24)
        guides += "  <line id=\"left-margin-\(m.name)\" \(line) x1=\"\(f(left))\" x2=\"\(f(left))\" y1=\"\(top)\" y2=\"\(bottom)\"/>\n"
        guides += "  <line id=\"right-margin-\(m.name)\" \(line) x1=\"\(f(right))\" x2=\"\(f(right))\" y1=\"\(top)\" y2=\"\(bottom)\"/>\n"
        symbols += "  <g id=\"\(m.name)\" transform=\"matrix(1 0 0 1 \(f(left)) \(f(m.baseline)))\">\n"
        symbols += "   <path class=\"monochrome-0 multicolor-0:tintColor hierarchical-0:secondary SFSymbolsPreviewWireframe\" d=\"\(svg(body, dx: dx))\"/>\n"
        if !satellites.isEmpty {
            symbols += "   <path class=\"monochrome-1 multicolor-1:tintColor hierarchical-1:primary SFSymbolsPreviewWireframe\" d=\"\(svg(satellites, dx: dx))\"/>\n"
        }
        symbols += "  </g>\n"
    }
    return """
    <?xml version="1.0" encoding="UTF-8"?>
    <!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">
    <svg version="1.1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="3300" height="2200">
     <g id="Notes">
      <text id="template-version" style="stroke:none;fill:black;font-family:sans-serif;font-size:13;" transform="matrix(1 0 0 1 2952 1933)">Template v.3.0</text>
      <text style="stroke:none;fill:black;font-family:sans-serif;font-size:13;" transform="matrix(1 0 0 1 2952 1951)">\(state.rawValue)</text>
     </g>
     <g id="Guides">
    \(guides) </g>
     <g id="Symbols">
    \(symbols) </g>
    </svg>

    """
}

let catalog = URL(fileURLWithPath: CommandLine.arguments[1])
let fm = FileManager.default
try? fm.removeItem(at: catalog)
try fm.createDirectory(at: catalog, withIntermediateDirectories: true)
try #"{ "info": { "author": "xcode", "version": 1 } }"#.appending("\n").write(to: catalog.appendingPathComponent("Contents.json"), atomically: true, encoding: .utf8)
for state in State.allCases {
    let set = catalog.appendingPathComponent("\(state.rawValue).symbolset")
    try fm.createDirectory(at: set, withIntermediateDirectories: true)
    try template(state).write(to: set.appendingPathComponent("\(state.rawValue).svg"), atomically: true, encoding: .utf8)
    let contents = """
    {
        "info": { "author": "xcode", "version": 1 },
        "symbols": [{ "filename": "\(state.rawValue).svg", "idiom": "universal" }]
    }
    """
    try (contents + "\n").write(to: set.appendingPathComponent("Contents.json"), atomically: true, encoding: .utf8)
}
