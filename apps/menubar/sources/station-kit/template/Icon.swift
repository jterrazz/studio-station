import AppKit
import SwiftUI

public struct MenuBarIcon: Equatable, Sendable {
    public enum Mark: Equatable, Sendable {
        case plain, needs(Int), down, stopped, connecting
    }

    public var mark: Mark
    public var spark: [Int]?

    public init(_ mark: Mark, spark: [Int]? = nil) {
        self.mark = mark
        self.spark = spark
    }

    // invariant: SPEC §10.1: the icon draws the page's badge and never
    // judges it: a server down wins over any count, a count is orange, and
    // nothing needing you is the symbol alone; the engine stopped dims it
    // under an orange `!`, so its silence is never mistaken for all good. The CPU line is the owner's
    // option, drawn only while the engine streams.
    public static func of(connection: Connection, page: Page?, sparkline: Bool = false) -> MenuBarIcon {
        switch connection {
        case .connecting: return MenuBarIcon(.connecting)
        case .stopped, .notInstalled: return MenuBarIcon(.stopped)
        case .live: break
        }
        let spark = sparkline ? page?.strip?.spark.flatMap { $0.isEmpty ? nil : $0 } : nil
        guard let badge = page?.badge else { return MenuBarIcon(.plain, spark: spark) }
        if badge.down == true { return MenuBarIcon(.down, spark: spark) }
        if let n = badge.needs, n > 0 { return MenuBarIcon(.needs(n), spark: spark) }
        return MenuBarIcon(.plain, spark: spark)
    }

    public var label: String {
        switch mark {
        case .plain: "station: nothing needs you"
        case .needs(1): "station: 1 thing needs you"
        case .needs(let n): "station: \(n) things need you"
        case .down: "station: a server is down"
        case .stopped: "station: engine stopped"
        case .connecting: "station: connecting"
        }
    }

    // value: an icon with no colour of its own: macOS tints it for the menu
    // bar's appearance.
    public var template: Bool {
        switch mark {
        case .plain, .connecting: true
        case .needs, .down, .stopped: false
        }
    }
}

// value: the Orbit mark on its 18-unit grid: one Mac, the rounded square
// at the centre, and two of its machines riding a broken ring around it.
enum Orbit {
    static let grid: CGFloat = 18
    static let core = CGRect(x: 6, y: 6, width: 6, height: 6)
    static let coreRadius: CGFloat = 1.75
    static let ring: CGFloat = 7.15
    static let satellite: CGFloat = 1.85
    static let gap = Angle.radians(2 * asin(3.1 / (2 * ring)))
    static let satellites = [Angle.degrees(210), Angle.degrees(30)]
    static let notch = Angle.degrees(340)

    static func point(_ a: Angle, _ r: CGFloat, _ u: CGFloat) -> CGPoint {
        CGPoint(x: (9 + r * cos(a.radians)) * u, y: (9 + r * sin(a.radians)) * u)
    }
}

// invariant: a notched mark ends its ring inside the notch, so no stub of
// ring is left between the badge and the satellite.
struct OrbitBody: Shape {
    var notched = false

    func path(in rect: CGRect) -> Path {
        let u = rect.width / Orbit.grid
        let c = Orbit.core
        var p = Path(roundedRect: CGRect(x: c.minX * u, y: c.minY * u, width: c.width * u, height: c.height * u), cornerRadius: Orbit.coreRadius * u)
        for from in Orbit.satellites {
            let end = from == Orbit.satellites[0] && notched ? Orbit.notch : from + .degrees(180) - Orbit.gap
            p.move(to: Orbit.point(from + Orbit.gap, Orbit.ring, u))
            p.addRelativeArc(center: CGPoint(x: 9 * u, y: 9 * u), radius: Orbit.ring * u, startAngle: from + Orbit.gap, delta: end - from - Orbit.gap)
        }
        return p
    }
}

struct OrbitSatellites: Shape {
    func path(in rect: CGRect) -> Path {
        let u = rect.width / Orbit.grid
        var p = Path()
        for a in Orbit.satellites {
            let c = Orbit.point(a, Orbit.ring, u)
            let r = Orbit.satellite * u
            p.addEllipse(in: CGRect(x: c.x - r, y: c.y - r, width: 2 * r, height: 2 * r))
        }
        return p
    }
}

struct IconSpark: Shape {
    let points: [Int]

    func path(in rect: CGRect) -> Path {
        var p = Path()
        guard points.count > 1 else { return p }
        let step = rect.width / CGFloat(points.count - 1)
        for (i, v) in points.enumerated() {
            let pt = CGPoint(x: CGFloat(i) * step, y: rect.height - rect.height * CGFloat(min(max(v, 0), 100)) / 100)
            if i == 0 { p.move(to: pt) } else { p.addLine(to: pt) }
        }
        return p
    }
}

public struct MenuBarIconView: View {
    let icon: MenuBarIcon
    let scheme: ColorScheme

    public init(_ icon: MenuBarIcon, scheme: ColorScheme) {
        self.icon = icon
        self.scheme = scheme
    }

    var ink: Color { Token.Icon.ink(scheme) }

    var dimmed: Bool { icon.mark == .stopped }

    var working: Bool { icon.mark == .connecting }

    public var body: some View {
        HStack(spacing: Token.Icon.gap) {
            ZStack {
                OrbitBody(notched: text != nil)
                    .stroke(ink, style: StrokeStyle(lineWidth: Token.Icon.line, lineCap: .butt, lineJoin: .round))
                    .opacity(working ? Token.Icon.dimmed : 1)
                OrbitSatellites().fill(ink)
                knockout.blendMode(.destinationOut)
            }
            .compositingGroup()
            .frame(width: Token.Icon.symbol, height: Token.Icon.symbol)
            .opacity(dimmed ? Token.Icon.dimmed : 1)
            .overlay(alignment: .topLeading) { badge.offset(x: Token.Icon.badgeAt.x, y: Token.Icon.badgeAt.y) }
            .padding(.trailing, room)
            if let spark = icon.spark {
                IconSpark(points: spark)
                    .stroke(ink, style: StrokeStyle(lineWidth: Token.Icon.line - 0.2, lineCap: .round, lineJoin: .round))
                    .frame(width: Token.Icon.sparkWidth, height: Token.Icon.sparkHeight)
            }
        }
        .padding(.vertical, 3)
        .padding(.leading, 2)
        .padding(.trailing, 6)
    }

    var text: String? {
        switch icon.mark {
        case .needs(let n): n > 99 ? "99+" : "\(n)"
        case .down, .stopped: Token.Glyph.down
        case .plain, .connecting: nil
        }
    }

    // value: the room the badge takes past the mark, wider as its count
    // grows, so a count is drawn whole and the CPU line moves aside.
    var room: CGFloat {
        guard let text else { return 0 }
        return Token.Icon.badge + Token.Icon.badgeAt.x - Token.Icon.symbol + CGFloat(text.count - 1) * Token.Icon.digit
    }

    var fill: Color {
        icon.mark == .down ? Token.Icon.down(scheme) : Token.Icon.needs(scheme)
    }

    @ViewBuilder
    var badge: some View {
        if let text { pill(text).background(Capsule().fill(fill)) }
    }

    // invariant: the badge sits in a notch cut out of the mark, never on
    // its strokes, so the mark and the badge each read whole.
    @ViewBuilder
    var knockout: some View {
        if let text {
            pill(text)
                .opacity(0)
                .padding(Token.Icon.notch)
                .background(Capsule().fill(ink))
                .offset(x: Token.Icon.badgeAt.x - Token.Icon.notch, y: Token.Icon.badgeAt.y - Token.Icon.notch)
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        }
    }

    func pill(_ text: String) -> some View {
        Text(text)
            .font(.system(size: Token.Icon.badgeText, weight: .bold))
            .foregroundStyle(Token.Icon.badgeInk)
            .lineLimit(1)
            .fixedSize()
            .padding(.horizontal, 2)
            .frame(minWidth: Token.Icon.badge, minHeight: Token.Icon.badge)
    }
}

extension MenuBarIcon {
    // value: the icon as the menu bar's label draws it: rendered offscreen,
    // at the screen's scale, for the menu bar's appearance.
    @MainActor
    public func image(scheme: ColorScheme, scale: CGFloat = 2) -> CGImage? {
        let renderer = ImageRenderer(content: MenuBarIconView(self, scheme: template ? .light : scheme))
        renderer.scale = scale
        return renderer.cgImage
    }

    // value: the label's image with a bitmap per screen scale, so the mark
    // lands on whole pixels at 1x, 2x and 3x alike.
    @MainActor
    public func nsImage(scheme: ColorScheme) -> NSImage {
        let image = NSImage()
        for scale in [1, 2, 3] as [CGFloat] {
            guard let cg = self.image(scheme: scheme, scale: scale) else { continue }
            let rep = NSBitmapImageRep(cgImage: cg)
            rep.size = NSSize(width: CGFloat(cg.width) / scale, height: CGFloat(cg.height) / scale)
            image.addRepresentation(rep)
            image.size = rep.size
        }
        image.isTemplate = template
        image.accessibilityDescription = label
        return image
    }
}
