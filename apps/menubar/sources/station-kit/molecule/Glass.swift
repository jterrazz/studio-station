import SwiftUI

private struct GlassSpaceKey: EnvironmentKey {
    static let defaultValue: Namespace.ID? = nil
}

extension EnvironmentValues {
    // value: the namespace a page's glass shapes morph in, from a row of
    // Home to the header of the page it pushes.
    public var stationGlassSpace: Namespace.ID? {
        get { self[GlassSpaceKey.self] }
        set { self[GlassSpaceKey.self] = newValue }
    }
}

// invariant: the one way a view of the kit is glass : live, the
// system's Liquid Glass, tinted only where a state asks it, and morphing
// by its id; drawn still (an offscreen snapshot, which cannot sample what
// lies behind), the same shape as a translucent body, a sheen and a
// specular rim.
struct StationGlass<S: InsettableShape>: ViewModifier {
    let shape: S
    let tint: Color?
    let id: String?
    let interactive: Bool
    @Environment(\.stationStill) private var still
    @Environment(\.stationGlassSpace) private var space
    @Environment(\.colorScheme) private var scheme

    func body(content: Content) -> some View {
        if still {
            content.background { GlassDrawn(shape: shape, tint: tint, scheme: scheme) }
        } else {
            let glass = (tint.map { Glass.regular.tint($0.opacity(Token.Glass.tint * 1.6)) } ?? .regular).interactive(interactive)
            if let id, let space {
                content.glassEffect(glass, in: shape).glassEffectID(id, in: space)
            } else {
                content.glassEffect(glass, in: shape)
            }
        }
    }
}

struct GlassDrawn<S: InsettableShape>: View {
    let shape: S
    let tint: Color?
    let scheme: ColorScheme

    var body: some View {
        ZStack {
            shape.fill(Token.Glass.body(scheme))
            if let tint { shape.fill(tint.opacity(Token.Glass.tint)) }
            shape.fill(LinearGradient(colors: Token.Glass.sheen(scheme), startPoint: .top, endPoint: .center))
            shape.strokeBorder(LinearGradient(colors: Token.Glass.rim(scheme), startPoint: .topLeading, endPoint: .bottomTrailing), lineWidth: 0.8)
        }
    }
}

extension View {
    // value: a tile or a grouped card of glass, 16 pt round unless said.
    public func stationGlass(tint: Color? = nil, radius: CGFloat = Token.Glass.radius, id: String? = nil) -> some View {
        modifier(StationGlass(shape: RoundedRectangle(cornerRadius: radius, style: .continuous), tint: tint, id: id, interactive: false))
    }

    public func stationGlassCapsule(tint: Color? = nil, id: String? = nil) -> some View {
        modifier(StationGlass(shape: Capsule(), tint: tint, id: id, interactive: true))
    }

    // value: the panel's own window: the MenuBarExtra's system glass shows
    // through live, so nothing is painted; a snapshot draws that window.
    public func stationPanel() -> some View { modifier(PanelGlass()) }
}

struct PanelGlass: ViewModifier {
    @Environment(\.stationStill) private var still
    @Environment(\.colorScheme) private var scheme

    func body(content: Content) -> some View {
        if still {
            let shape = RoundedRectangle(cornerRadius: Token.Glass.window, style: .continuous)
            content.background {
                ZStack {
                    shape.fill(Token.Glass.pane(scheme))
                    shape.fill(LinearGradient(colors: Token.Glass.sheen(scheme), startPoint: .top, endPoint: .center))
                    shape.strokeBorder(LinearGradient(colors: Token.Glass.rim(scheme), startPoint: .topLeading, endPoint: .bottomTrailing), lineWidth: 1)
                }
                .shadow(color: Token.Glass.shade(scheme), radius: 18, y: 10)
            }
        } else {
            content
        }
    }
}

// invariant: one GlassEffectContainer per page, so its tiles blend and
// morph as one surface and no glass is drawn inside another.
public struct GlassPage<Content: View>: View {
    let content: Content
    @Environment(\.stationStill) private var still

    public init(@ViewBuilder content: () -> Content) { self.content = content() }

    public var body: some View {
        if still {
            content
        } else {
            GlassEffectContainer(spacing: Token.Glass.spacing) { content }
        }
    }
}

// invariant: the menu bar's pages are Liquid Glass (VISION-v3 panels a-g):
// each group one rounded pane of glass, its rows inside it.
public struct GlassCard<Content: View>: View {
    let tint: Color?
    let id: String?
    let padding: CGFloat
    let spacing: CGFloat
    let content: Content

    public init(tint: Color? = nil, id: String? = nil, padding: CGFloat = Token.Glass.padding, spacing: CGFloat = Token.Space.gap, @ViewBuilder content: () -> Content) {
        self.tint = tint
        self.id = id
        self.padding = padding
        self.spacing = spacing
        self.content = content()
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: spacing) { content }
            .padding(padding)
            .frame(maxWidth: .infinity, alignment: .leading)
            .stationGlass(tint: tint, id: id)
    }
}

// value: Control Center's state circle: a white glyph on the state's
// colour, the glyph alone on a faint disc when off.
public struct IconCircle: View {
    let symbol: String
    let tint: Color?
    let size: CGFloat
    let pulse: Bool
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationStill) private var still

    public init(_ symbol: String, tint: Color?, size: CGFloat = Token.Tile.circle, pulse: Bool = false) {
        self.symbol = symbol
        self.tint = tint
        self.size = size
        self.pulse = pulse
    }

    public var body: some View {
        Image(systemName: symbol)
            .font(.system(size: size * 0.46, weight: .semibold))
            .foregroundStyle(tint == nil ? palette.muted : palette.onTint)
            .contentTransition(.symbolEffect(.replace))
            .symbolEffect(.pulse, isActive: pulse && !still)
            .frame(width: size, height: size)
            .background(Circle().fill(tint ?? palette.faint))
    }
}

// value: a line the pointer is over lights up, 8 pt round, as a menu's.
struct Hover: ViewModifier {
    @State private var over = false
    @Environment(\.stationPalette) private var palette

    func body(content: Content) -> some View {
        content
            .contentShape(Rectangle())
            .background(RoundedRectangle(cornerRadius: Token.Tile.hover, style: .continuous).fill(over ? palette.hover : .clear).padding(.horizontal, -6))
            .onHover { over = $0 }
    }
}

extension View {
    public func stationHover() -> some View { modifier(Hover()) }
}

public struct PushedHeader: View {
    let back: String
    let title: String
    let subtitle: String?
    let symbol: String?
    let tint: Color?
    let id: String?
    let action: () -> Void
    @Environment(\.stationPalette) private var palette

    public init(back: String, title: String, subtitle: String? = nil, symbol: String? = nil, tint: Color? = nil, id: String? = nil, action: @escaping () -> Void = {}) {
        self.back = back
        self.title = title
        self.subtitle = subtitle
        self.symbol = symbol
        self.tint = tint
        self.id = id
        self.action = action
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Button(action: action) {
                HStack(spacing: 3) {
                    Image(systemName: Token.Symbol.back).font(.system(size: Token.Space.small, weight: .semibold))
                    Text(back).font(.system(size: Token.Space.text, weight: .medium))
                }
                .foregroundStyle(palette.accent)
            }
            .buttonStyle(.plain)
            HStack(spacing: Token.Space.gap + 2) {
                if let symbol { IconCircle(symbol, tint: tint, size: Token.Tile.circle + 4) }
                VStack(alignment: .leading, spacing: 1) {
                    Text(title).font(.system(size: Token.Space.title, weight: .bold)).foregroundStyle(palette.text)
                    if let subtitle, !subtitle.isEmpty {
                        Text(subtitle).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).lineLimit(2)
                    }
                }
                Spacer(minLength: 0)
            }
            .padding(.horizontal, 2)
            .modifier(MorphID(id: id))
        }
    }
}

// value: a header that takes over the glass id of the tile that pushed it.
struct MorphID: ViewModifier {
    let id: String?
    @Environment(\.stationStill) private var still
    @Environment(\.stationGlassSpace) private var space

    func body(content: Content) -> some View {
        if let id, let space, !still {
            content.glassEffect(.identity, in: .rect(cornerRadius: Token.Glass.radius)).glassEffectID(id, in: space)
        } else {
            content
        }
    }
}

public struct SectionHead: View {
    let text: String
    @Environment(\.stationPalette) private var palette

    public init(_ text: String) { self.text = text }

    public var body: some View {
        Text(text.uppercased()).font(.system(size: Token.Space.small, weight: .semibold)).tracking(0.4).foregroundStyle(palette.muted).padding(.horizontal, 4).padding(.top, 2)
    }
}

public enum PillRole { case plain, primary, danger }

// invariant: every button of the kit is a glass button: `.glass`, the one
// primary action `.glassProminent`; drawn still, a capsule of the same.
struct GlassButton: ViewModifier {
    let role: PillRole
    let picked: Bool
    @Environment(\.stationStill) private var still
    @Environment(\.stationPalette) private var palette

    func body(content: Content) -> some View {
        if still {
            content.buttonStyle(DrawnGlassButton(role: role, picked: picked))
        } else if role == .primary || picked {
            content.buttonStyle(.glassProminent).tint(palette.accent)
        } else {
            content.buttonStyle(.glass)
        }
    }
}

struct DrawnGlassButton: ButtonStyle {
    let role: PillRole
    let picked: Bool
    @Environment(\.stationPalette) private var palette

    func makeBody(configuration: Configuration) -> some View {
        let filled = role == .primary || picked
        return configuration.label
            .padding(.horizontal, 12)
            .padding(.vertical, 5)
            .background {
                if filled {
                    Capsule().fill(palette.accent)
                } else {
                    Capsule().fill(.clear).stationGlassCapsule()
                }
            }
            .opacity(configuration.isPressed ? 0.7 : 1)
    }
}

public struct Pill: View {
    let label: String
    let role: PillRole
    let picked: Bool
    let wide: Bool
    let action: () -> Void
    @Environment(\.stationPalette) private var palette

    public init(_ label: String, role: PillRole = .plain, picked: Bool = false, wide: Bool = false, action: @escaping () -> Void = {}) {
        self.label = label
        self.role = role
        self.picked = picked
        self.wide = wide
        self.action = action
    }

    var ink: Color {
        switch role {
        case .primary: palette.onTint
        case .danger: palette.bad
        case .plain: picked ? palette.onTint : palette.text
        }
    }

    public var body: some View {
        Button(action: action) {
            Text(label).font(.system(size: wide ? Token.Space.text : Token.Space.small + 1, weight: .semibold)).foregroundStyle(ink).lineLimit(1)
                .frame(maxWidth: wide ? .infinity : nil)
                .padding(.vertical, wide ? 3 : 0)
        }
        .modifier(GlassButton(role: role, picked: picked))
        .controlSize(wide ? .large : .regular)
    }
}
