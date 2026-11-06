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
            shape.strokeBorder(LinearGradient(colors: Token.Glass.rim(scheme), startPoint: .top, endPoint: .bottom), lineWidth: 0.6)
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

    // value: a sunken well of glass a field, an address or a picker's
    // options sit in: live, the system's clear glass; drawn still, a
    // shade deeper than what lies behind, its rim lit from below.
    public func stationWell(radius: CGFloat = Token.Tile.hover) -> some View {
        modifier(StationWell(shape: RoundedRectangle(cornerRadius: radius, style: .continuous)))
    }

    // value: the panel's own window: the MenuBarExtra's system glass shows
    // through live, so nothing is painted; a snapshot draws that window.
    public func stationPanel() -> some View { modifier(PanelGlass()) }
}

struct StationWell<S: InsettableShape>: ViewModifier {
    let shape: S
    @Environment(\.stationStill) private var still
    @Environment(\.colorScheme) private var scheme

    func body(content: Content) -> some View {
        if still {
            content.background {
                ZStack {
                    shape.fill(LinearGradient(colors: Token.Glass.sunken(scheme), startPoint: .top, endPoint: .bottom))
                    shape.strokeBorder(LinearGradient(colors: Token.Glass.sunkenRim(scheme), startPoint: .top, endPoint: .bottom), lineWidth: 0.6)
                }
            }
        } else {
            content.glassEffect(.clear, in: shape)
        }
    }
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
                    shape.strokeBorder(Token.Glass.edge(scheme), lineWidth: 0.6)
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
    let fill: Bool
    let content: Content

    public init(tint: Color? = nil, id: String? = nil, padding: CGFloat = Token.Glass.padding, spacing: CGFloat = Token.Space.gap, fill: Bool = false, @ViewBuilder content: () -> Content) {
        self.tint = tint
        self.id = id
        self.padding = padding
        self.spacing = spacing
        self.fill = fill
        self.content = content()
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: spacing) { content }
            .padding(padding)
            .frame(maxWidth: .infinity, maxHeight: fill ? .infinity : nil, alignment: .topLeading)
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
                if let symbol { IconTile(symbol, tint: tint, size: Token.Tile.icon + 8) }
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
            Text(label).font(.system(size: wide ? Token.Space.text : Token.Space.small + 1, weight: .semibold)).foregroundStyle(ink).lineLimit(1).fixedSize()
                .frame(maxWidth: wide ? .infinity : nil)
                .padding(.vertical, wide ? 3 : 0)
        }
        .modifier(GlassButton(role: role, picked: picked))
        .controlSize(wide ? .large : .regular)
    }
}

// value: a grouped card's head, as the owner's references draw one: its
// small grey title, a count, words after it, and its one action on the
// right in the accent ("+ Add", "Open").
public struct CardHead: View {
    let title: String
    let count: Int?
    let words: String?
    let action: (label: String, tap: () -> Void)?
    @Environment(\.stationPalette) private var palette

    public init(_ title: String, count: Int? = nil, words: String? = nil, action: (label: String, tap: () -> Void)? = nil) {
        self.title = title
        self.count = count
        self.words = words
        self.action = action
    }

    public var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: 6) {
            Text(title).font(.system(size: Token.Space.small + 1, weight: .semibold)).foregroundStyle(palette.muted)
            if let count { Text("\(count)").font(.system(size: Token.Space.small + 1, weight: .medium)).foregroundStyle(palette.subtle).monospacedDigit() }
            if let words, !words.isEmpty { Text(words).font(.system(size: Token.Space.small)).foregroundStyle(palette.subtle).lineLimit(1) }
            Spacer(minLength: 0)
            if let action { TextAction(action.label, tap: action.tap) }
        }
        .padding(.bottom, 2)
    }
}

// value: a grouped card of glass led by its head.
public struct GroupCard<Content: View>: View {
    let head: CardHead
    let spacing: CGFloat
    let tint: Color?
    let content: Content

    public init(_ title: String, count: Int? = nil, words: String? = nil, action: (label: String, tap: () -> Void)? = nil, tint: Color? = nil, spacing: CGFloat = 0, @ViewBuilder content: () -> Content) {
        head = CardHead(title, count: count, words: words, action: action)
        self.tint = tint
        self.spacing = spacing
        self.content = content()
    }

    public var body: some View {
        GlassCard(tint: tint, padding: 12, spacing: 4) {
            head
            VStack(alignment: .leading, spacing: spacing) { content }
        }
    }
}

// value: a text button, the accent's or, for what destroys, red, with its
// symbol before it ("+ Add", "Delete").
public struct TextAction: View {
    let label: String
    let symbol: String?
    let danger: Bool
    let tap: () -> Void
    @Environment(\.stationPalette) private var palette

    public init(_ label: String, symbol: String? = nil, danger: Bool = false, tap: @escaping () -> Void) {
        self.label = label
        self.symbol = symbol
        self.danger = danger
        self.tap = tap
    }

    public var body: some View {
        Button(action: tap) {
            HStack(spacing: 5) {
                if let symbol { Image(systemName: symbol).font(.system(size: Token.Space.small + 1, weight: .medium)) }
                Text(label).font(.system(size: Token.Space.text, weight: .medium)).lineLimit(1)
            }
            .foregroundStyle(danger ? palette.bad : palette.accent)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

// value: an app's or a machine's icon as the references draw one: a
// rounded square, its glyph white on the state's colour, grey when off.
public struct IconTile: View {
    let symbol: String
    let tint: Color?
    let size: CGFloat
    let pulse: Bool
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationStill) private var still

    public init(_ symbol: String, tint: Color?, size: CGFloat = Token.Tile.icon, pulse: Bool = false) {
        self.symbol = symbol
        self.tint = tint
        self.size = size
        self.pulse = pulse
    }

    public var body: some View {
        let shape = RoundedRectangle(cornerRadius: size * 0.27, style: .continuous)
        Image(systemName: symbol)
            .font(.system(size: size * 0.48, weight: .semibold))
            .foregroundStyle(tint == nil ? palette.muted : palette.onTint)
            .contentTransition(.symbolEffect(.replace))
            .symbolEffect(.pulse, isActive: pulse && !still)
            .frame(width: size, height: size)
            .background {
                if let tint {
                    shape.fill(LinearGradient(colors: [tint.opacity(0.92), tint], startPoint: .top, endPoint: .bottom))
                } else {
                    shape.fill(palette.faint)
                }
            }
            .overlay(shape.strokeBorder(palette.hover, lineWidth: 0.5))
    }
}

// value: a round glass button that holds only a symbol: copy, QR, open.
public struct RoundButton: View {
    let symbol: String
    let picked: Bool
    let tap: () -> Void
    @Environment(\.stationPalette) private var palette

    public init(_ symbol: String, picked: Bool = false, tap: @escaping () -> Void) {
        self.symbol = symbol
        self.picked = picked
        self.tap = tap
    }

    public var body: some View {
        Button(action: tap) {
            Image(systemName: symbol).font(.system(size: Token.Space.small + 1, weight: .medium))
                .foregroundStyle(picked ? palette.onTint : palette.text)
                .frame(width: Token.Tile.circle, height: Token.Tile.circle)
                .background { if picked { Circle().fill(palette.accent) } }
                .stationGlassCapsule()
                .contentShape(Circle())
        }
        .buttonStyle(.plain)
    }
}

// value: a link as a field: the address monospaced in a sunken well, its
// copy and QR buttons beside it.
public struct LinkField: View {
    let text: String
    let qr: Bool
    let copy: () -> Void
    let toggleQR: (() -> Void)?
    let open: (() -> Void)?
    @Environment(\.stationPalette) private var palette

    public init(_ text: String, qr: Bool, copy: @escaping () -> Void, toggleQR: (() -> Void)?, open: (() -> Void)? = nil) {
        self.open = open
        self.text = text
        self.qr = qr
        self.copy = copy
        self.toggleQR = toggleQR
    }

    public var body: some View {
        HStack(spacing: 6) {
            Text(text).font(.system(size: Token.Space.small, design: .monospaced)).foregroundStyle(palette.text).lineLimit(1).truncationMode(.middle)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.horizontal, 9).padding(.vertical, 7)
                .stationWell()
            RoundButton(Token.Symbol.copy, tap: copy)
            if let toggleQR { RoundButton(Token.Symbol.qr, picked: qr, tap: toggleQR) }
            if let open { RoundButton(Token.Symbol.open, tap: open) }
        }
    }
}

// value: a header's state as the references say it: the word in its
// colour, then the switch that flips it.
public struct StateSwitch: View {
    let word: String
    let ink: Color
    let on: Bool
    let busy: Bool
    let tap: (() -> Void)?

    public init(_ word: String, ink: Color, on: Bool, busy: Bool = false, tap: (() -> Void)?) {
        self.word = word
        self.ink = ink
        self.on = on
        self.busy = busy
        self.tap = tap
    }

    public var body: some View {
        HStack(spacing: 8) {
            Text(word).font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(ink).lineLimit(1)
            if busy {
                PendingDot()
            } else if let tap {
                Button(action: tap) { Switch(on: on) }.buttonStyle(.plain)
            }
        }
    }
}
