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
    // nothing needing you is the symbol alone. The CPU line is the owner's
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
        case .plain, .stopped, .connecting: true
        case .needs, .down: false
        }
    }
}

struct StationSymbol: Shape {
    func path(in rect: CGRect) -> Path {
        let u = rect.width / 16
        var p = Path(roundedRect: CGRect(x: 2.2 * u, y: 2.2 * u, width: 11.6 * u, height: 11.6 * u), cornerRadius: 3.2 * u)
        p.move(to: CGPoint(x: 5 * u, y: 6.3 * u))
        p.addLine(to: CGPoint(x: 11 * u, y: 6.3 * u))
        p.move(to: CGPoint(x: 5 * u, y: 9.7 * u))
        p.addLine(to: CGPoint(x: 8.6 * u, y: 9.7 * u))
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

    var dimmed: Bool {
        switch icon.mark {
        case .stopped, .connecting: true
        case .plain, .needs, .down: false
        }
    }

    public var body: some View {
        HStack(spacing: Token.Icon.gap) {
            StationSymbol()
                .stroke(ink, style: StrokeStyle(lineWidth: Token.Icon.line, lineCap: .round, lineJoin: .round))
                .frame(width: Token.Icon.symbol, height: Token.Icon.symbol)
                .overlay(alignment: .topTrailing) { badge.offset(Token.Icon.badgeShift) }
            if let spark = icon.spark {
                IconSpark(points: spark)
                    .stroke(ink, style: StrokeStyle(lineWidth: Token.Icon.line - 0.2, lineCap: .round, lineJoin: .round))
                    .frame(width: Token.Icon.sparkWidth, height: Token.Icon.sparkHeight)
            }
        }
        .opacity(dimmed ? Token.Icon.dimmed : 1)
        .padding(.vertical, 3)
        .padding(.leading, 2)
        .padding(.trailing, 6)
    }

    @ViewBuilder
    var badge: some View {
        switch icon.mark {
        case .needs(let n): pill("\(n)", Token.Icon.needs(scheme))
        case .down: pill(Token.Glyph.down, Token.Icon.down(scheme))
        case .plain, .stopped, .connecting: EmptyView()
        }
    }

    func pill(_ text: String, _ fill: Color) -> some View {
        Text(text)
            .font(.system(size: Token.Icon.badgeText, weight: .bold))
            .foregroundStyle(Token.Icon.badgeInk)
            .padding(.horizontal, 3)
            .frame(minWidth: Token.Icon.badge, minHeight: Token.Icon.badge)
            .background(Capsule().fill(fill))
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
}
