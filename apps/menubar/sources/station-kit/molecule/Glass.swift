import SwiftUI

private struct GlassSpaceKey: EnvironmentKey {
    static let defaultValue: Namespace.ID? = nil
}

private struct InCardKey: EnvironmentKey {
    static let defaultValue = false
}

extension EnvironmentValues {
    // value: the namespace a page's glass shapes morph in, from a row of
    // Home to the header of the page it pushes.
    public var stationGlassSpace: Namespace.ID? {
        get { self[GlassSpaceKey.self] }
        set { self[GlassSpaceKey.self] = newValue }
    }

    // value: whether a view is drawn inside a card, where nothing is glass
    // again: a card never holds a card.
    public var stationInCard: Bool {
        get { self[InCardKey.self] }
        set { self[InCardKey.self] = newValue }
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
    @Environment(\.stationInCard) private var inCard
    @Environment(\.colorScheme) private var scheme

    func body(content: Content) -> some View {
        if inCard {
            content
        } else if still {
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
    // value: a card of glass, 14 pt round unless said; inside a card,
    // nothing.
    public func stationGlass(tint: Color? = nil, radius: CGFloat = Token.Glass.radius, id: String? = nil) -> some View {
        modifier(StationGlass(shape: RoundedRectangle(cornerRadius: radius, style: .continuous), tint: tint, id: id, interactive: false))
    }

    // value: a flat inset a field or an address sits in, as the
    // references draw one in a card: a shade darker, no rim.
    public func stationWell(radius: CGFloat = Token.Tile.hover) -> some View {
        modifier(StationWell(shape: RoundedRectangle(cornerRadius: radius, style: .continuous)))
    }

    // value: the panel's own window: the MenuBarExtra's system glass shows
    // through live, so nothing is painted; a snapshot draws that window.
    public func stationPanel() -> some View { modifier(PanelGlass()) }

    // value: a card in the panel's own words: a top-level group of glass.
    public func stationCard() -> some View {
        environment(\.stationInCard, true).padding(Token.Glass.padding).frame(maxWidth: .infinity, alignment: .topLeading).stationGlass()
    }
}

struct StationWell<S: InsettableShape>: ViewModifier {
    let shape: S
    @Environment(\.colorScheme) private var scheme

    func body(content: Content) -> some View {
        content.background(shape.fill(Token.Glass.inset(scheme)))
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

// invariant: one GlassEffectContainer per page, so its cards blend and
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

// invariant: the owner's references taken literally: ONE panel, a card of
// glass only for a top-level group, and nothing inside a card is a
// bubble again: what it holds is drawn flat, its rows apart by space and
// hairlines.
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
            .environment(\.stationInCard, true)
            .padding(padding)
            .frame(maxWidth: .infinity, maxHeight: fill ? .infinity : nil, alignment: .topLeading)
            .stationGlass(tint: tint, id: id)
    }
}

// value: a line the pointer is over lights up across the whole row,
// faintly, as a menu's.
struct Hover: ViewModifier {
    @State private var over = false
    @Environment(\.stationPalette) private var palette

    func body(content: Content) -> some View {
        content
            .contentShape(Rectangle())
            .background(RoundedRectangle(cornerRadius: Token.Tile.hover - 2, style: .continuous).fill(over ? palette.hover : .clear).padding(.horizontal, -6))
            .onHover { over = $0 }
    }
}

extension View {
    public func stationHover() -> some View { modifier(Hover()) }
}

// value: a pushed page's head as the references' "Mizu Pro": the way
// back in the accent, then the title in bold and its words in grey on the
// same line, a small glyph before it, no tile.
public struct PushedHeader: View {
    let back: String
    let title: String
    let subtitle: String?
    let symbol: String?
    let id: String?
    let action: () -> Void
    @Environment(\.stationPalette) private var palette

    public init(back: String, title: String, subtitle: String? = nil, symbol: String? = nil, tint: Color? = nil, id: String? = nil, action: @escaping () -> Void = {}) {
        self.back = back
        self.title = title
        self.subtitle = subtitle
        self.symbol = symbol
        self.id = id
        self.action = action
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Button(action: action) {
                HStack(spacing: 3) {
                    Image(systemName: Token.Symbol.back).font(.system(size: Token.Space.small - 1, weight: .semibold))
                    Text(back).font(.system(size: Token.Space.small + 1, weight: .medium))
                }
                .foregroundStyle(palette.accent)
            }
            .buttonStyle(.plain)
            HStack(alignment: .firstTextBaseline, spacing: 6) {
                if let symbol { Image(systemName: symbol).font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.muted) }
                Text(title).font(.system(size: Token.Space.head, weight: .bold)).foregroundStyle(palette.text)
                if let subtitle, !subtitle.isEmpty {
                    Text(subtitle).font(.system(size: Token.Space.text)).foregroundStyle(palette.muted).lineLimit(1).truncationMode(.tail)
                }
                Spacer(minLength: 0)
            }
            .modifier(MorphID(id: id))
        }
        .padding(.horizontal, 4)
    }
}

// value: a header that takes over the glass id of the card that pushed it.
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

// invariant: a button of a sheet is small and flat, never glass: the
// plain one on a faint fill, the primary one on the accent, what
// destroys in red.
struct FlatButton: ButtonStyle {
    let role: PillRole
    let picked: Bool
    @Environment(\.stationPalette) private var palette

    func makeBody(configuration: Configuration) -> some View {
        let filled = role == .primary || picked
        return configuration.label
            .padding(.horizontal, 10)
            .padding(.vertical, 4)
            .background(RoundedRectangle(cornerRadius: Token.Tile.hover - 1, style: .continuous).fill(filled ? palette.accent : palette.hover))
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
            Text(label).font(.system(size: Token.Space.small + 1, weight: .medium)).foregroundStyle(ink).lineLimit(1).fixedSize()
                .frame(maxWidth: wide ? .infinity : nil)
        }
        .buttonStyle(FlatButton(role: role, picked: picked))
    }
}

// value: a card's head, as the references draw "Live 10": its small
// title, a count in grey, words after it, and its one action on the
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

    // value: a long title cut at its colon: the name on the head's line,
    // what it means in grey under it ("Public: reachable from anywhere").
    var cut: (String, String?) {
        guard let r = title.range(of: ": ") else { return (title, nil) }
        return (String(title[..<r.lowerBound]), String(title[r.upperBound...]))
    }

    public var body: some View {
        let (name, meaning) = cut
        VStack(alignment: .leading, spacing: 2) {
            line(name)
            if let meaning {
                Text(meaning).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).fixedSize(horizontal: false, vertical: true)
            }
        }
        .padding(.bottom, 2)
    }

    func line(_ title: String) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 6) {
            Text(title).font(.system(size: Token.Space.small + 1.5, weight: .semibold)).foregroundStyle(palette.text.opacity(0.88)).lineLimit(1)
            if let count { Text("\(count)").font(.system(size: Token.Space.small + 1.5, weight: .medium)).foregroundStyle(palette.muted).monospacedDigit() }
            if let words, !words.isEmpty { Text(words).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).lineLimit(1) }
            Spacer(minLength: 0)
            if let action { TextAction(action.label, tap: action.tap) }
        }
    }
}

// value: a card of glass led by its head.
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
        GlassCard(tint: tint, padding: Token.Glass.padding, spacing: 6) {
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
            HStack(spacing: 4) {
                if let symbol { Image(systemName: symbol).font(.system(size: Token.Space.small, weight: .medium)) }
                Text(label).font(.system(size: Token.Space.small + 1, weight: .medium)).lineLimit(1).fixedSize()
            }
            .foregroundStyle(danger ? palette.bad : palette.accent)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

// value: a machine's or a port's icon as the references draw one: a
// small rounded square, neutral and dark, its glyph white; dimmed when
// off. Colour is the state dot's, never the icon's.
public struct IconTile: View {
    let symbol: String
    let dim: Bool
    let size: CGFloat
    let pulse: Bool
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationStill) private var still

    public init(_ symbol: String, dim: Bool = false, size: CGFloat = Token.Tile.icon, pulse: Bool = false) {
        self.symbol = symbol
        self.dim = dim
        self.size = size
        self.pulse = pulse
    }

    public var body: some View {
        let shape = RoundedRectangle(cornerRadius: size * 0.28, style: .continuous)
        Image(systemName: symbol)
            .font(.system(size: size * 0.5, weight: .semibold))
            .foregroundStyle(palette.onTint.opacity(dim ? 0.45 : 0.95))
            .contentTransition(.symbolEffect(.replace))
            .symbolEffect(.pulse, isActive: pulse && !still)
            .frame(width: size, height: size)
            .background(shape.fill(palette.chip))
            .overlay(shape.strokeBorder(palette.hover, lineWidth: 0.5))
    }
}

// value: a small round button that holds only a symbol: stop, open; a
// faint disc, never glass.
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
            Image(systemName: symbol).font(.system(size: Token.Space.small - 1, weight: .semibold))
                .foregroundStyle(picked ? palette.onTint : palette.text)
                .frame(width: Token.Tile.smallCircle, height: Token.Tile.smallCircle)
                .background(Circle().fill(picked ? palette.accent : palette.hover))
                .contentShape(Circle())
        }
        .buttonStyle(.plain)
    }
}

// value: a bare symbol that is a button, as the references' header and
// link field draw them: no disc, no glass.
public struct PlainIcon: View {
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
            Image(systemName: symbol).font(.system(size: Token.Space.text - 1, weight: .medium))
                .foregroundStyle(picked ? palette.accent : palette.muted)
                .frame(width: 20, height: 22)
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

// value: a link as the references' field: the address monospaced in a
// flat inset, its copy, QR and open as bare symbols beside it.
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
        HStack(spacing: 4) {
            Text(text).font(.system(size: Token.Space.small, design: .monospaced)).foregroundStyle(palette.text).lineLimit(1).truncationMode(.middle)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.horizontal, 9).padding(.vertical, 6)
                .stationWell()
            PlainIcon(Token.Symbol.copy, tap: copy)
            if let toggleQR { PlainIcon(Token.Symbol.qr, picked: qr, tap: toggleQR) }
            if let open { PlainIcon(Token.Symbol.open, tap: open) }
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
            Text(word).font(.system(size: Token.Space.text, weight: .medium)).foregroundStyle(ink).lineLimit(1)
            if busy {
                PendingDot()
            } else if let tap {
                Button(action: tap) { Switch(on: on) }.buttonStyle(.plain)
            }
        }
    }
}

// value: the thin line of the references' "Live" tree that ties a child
// row to its parent: down the parent icon's middle, an elbow into the
// child's, ending there on the last child.
public struct Branch: View {
    public enum Kind { case pass, elbow, last }
    let kind: Kind
    let x: CGFloat
    let reach: CGFloat
    @Environment(\.stationPalette) private var palette

    public init(_ kind: Kind, x: CGFloat, reach: CGFloat) {
        self.kind = kind
        self.x = x
        self.reach = reach
    }

    public var body: some View {
        GeometryReader { geo in
            let mid = geo.size.height / 2
            Path { p in
                p.move(to: CGPoint(x: x, y: 0))
                p.addLine(to: CGPoint(x: x, y: kind == .last ? mid - 5 : geo.size.height))
                if kind != .pass {
                    if kind == .last {
                        p.addQuadCurve(to: CGPoint(x: x + 5, y: mid), control: CGPoint(x: x, y: mid))
                    } else {
                        p.move(to: CGPoint(x: x, y: mid))
                    }
                    p.addLine(to: CGPoint(x: reach, y: mid))
                }
            }
            .stroke(palette.faint, lineWidth: 1)
        }
    }
}
