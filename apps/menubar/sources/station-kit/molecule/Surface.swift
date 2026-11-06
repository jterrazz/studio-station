import SwiftUI

// invariant: the MenuBarExtra's window is the only glass. Live the panel
// paints nothing; drawn still (an offscreen snapshot, which cannot sample
// what lies behind) it stands in for that window. Its corner is the
// container shape every grouping inside is concentric with.
struct PanelGlass: ViewModifier {
    @Environment(\.stationStill) private var still
    @Environment(\.colorScheme) private var scheme

    func body(content: Content) -> some View {
        let shape = RoundedRectangle(cornerRadius: Token.Glass.window, style: .continuous)
        if still {
            content
                .containerShape(shape)
                .background {
                    ZStack {
                        shape.fill(Token.Glass.pane(scheme))
                        shape.fill(LinearGradient(colors: Token.Glass.sheen(scheme), startPoint: .top, endPoint: .center))
                        shape.strokeBorder(Token.Glass.edge(scheme), lineWidth: 0.6)
                    }
                    .shadow(color: Token.Glass.shade(scheme), radius: 18, y: 10)
                }
        } else {
            content.containerShape(shape)
        }
    }
}

// value: the one subtle grouping a page may hold (two at most): a quinary
// fill concentric with the panel, never glass; with Reduce Transparency
// or Increase Contrast, a separator stroke in its place.
struct Grouping: ViewModifier {
    let padding: CGFloat
    @Environment(\.accessibilityReduceTransparency) private var reduce
    @Environment(\.colorSchemeContrast) private var contrast

    func body(content: Content) -> some View {
        let shape = ConcentricRectangle(corners: .concentric(minimum: .fixed(Token.Tile.group)), isUniform: true)
        content
            .padding(padding)
            .frame(maxWidth: .infinity, alignment: .topLeading)
            .background {
                if reduce || contrast == .increased {
                    shape.stroke(.separator, lineWidth: 1)
                } else {
                    shape.fill(.quinary)
                }
            }
    }
}

// value: a row the pointer is over lights up as a menu's: a quaternary
// fill, 6 pt round, across the whole row.
struct Hover: ViewModifier {
    @State private var over = false

    func body(content: Content) -> some View {
        content
            .contentShape(Rectangle())
            .background {
                if over {
                    RoundedRectangle(cornerRadius: Token.Tile.hover, style: .continuous).fill(.fill.quaternary).padding(.horizontal, -6)
                }
            }
            .onHover { over = $0 }
    }
}

extension View {
    // value: the panel's own window (see PanelGlass).
    public func stationPanel() -> some View { modifier(PanelGlass()) }

    // value: a page's subtle grouping (see Grouping).
    public func stationGroup(padding: CGFloat = Token.Tile.groupPadding) -> some View { modifier(Grouping(padding: padding)) }

    public func stationHover() -> some View { modifier(Hover()) }
}

// value: a section's header as Apple's menus set one: semibold
// subheadline, secondary.
public struct SectionHeader: View {
    let title: String

    public init(_ title: String) { self.title = title }

    public var body: some View {
        Text(title).font(.subheadline.weight(.semibold)).foregroundStyle(.secondary).lineLimit(2)
            .fixedSize(horizontal: false, vertical: true)
    }
}

// value: a row's icon as Control Center and the Wi-Fi menu draw one: a
// 24 pt circle on the tertiary fill, its symbol in the label colour; the
// accent (or the state colour) with a white symbol only while active.
public struct IconCircle: View {
    let symbol: String
    let active: Bool
    let tint: Color?
    let size: CGFloat
    let pulse: Bool
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationStill) private var still
    @Environment(\.colorSchemeContrast) private var contrast

    public init(_ symbol: String, active: Bool = false, tint: Color? = nil, size: CGFloat = Token.Tile.circle, pulse: Bool = false) {
        self.symbol = symbol
        self.active = active
        self.tint = tint
        self.size = size
        self.pulse = pulse
    }

    public var body: some View {
        Image(systemName: symbol)
            .font(.system(size: size * 0.44, weight: .semibold))
            .foregroundStyle(active ? AnyShapeStyle(palette.onTint) : AnyShapeStyle(.primary))
            .contentTransition(.symbolEffect(.replace))
            .symbolEffect(.pulse, isActive: pulse && !still)
            .frame(width: size, height: size)
            .background {
                if active {
                    Circle().fill(tint ?? palette.accent)
                } else if contrast == .increased {
                    Circle().stroke(.separator, lineWidth: 1)
                } else {
                    Circle().fill(.fill.tertiary)
                }
            }
    }
}

// invariant: a row as Apple's Wi-Fi and Battery menus: at least 28 pt,
// what leads it, its title in the body font, its words under it in
// secondary, what ends it; the whole row lights up under the pointer and
// is one button when it opens or does something.
public struct MenuRow<Leading: View, Trailing: View>: View {
    let title: String
    let subtitle: String?
    let link: Bool
    let tap: (() -> Void)?
    let leading: Leading
    let trailing: Trailing

    public init(_ title: String, subtitle: String? = nil, link: Bool = false, tap: (() -> Void)? = nil, @ViewBuilder leading: () -> Leading, @ViewBuilder trailing: () -> Trailing) {
        self.title = title
        self.subtitle = subtitle
        self.link = link
        self.tap = tap
        self.leading = leading()
        self.trailing = trailing()
    }

    var line: some View {
        HStack(spacing: Token.Space.gap) {
            leading
            VStack(alignment: .leading, spacing: 1) {
                Text(title).font(.body).foregroundStyle(.primary).lineLimit(1)
                if let subtitle, !subtitle.isEmpty {
                    if link {
                        LinkText(subtitle)
                    } else {
                        Text(subtitle).font(.subheadline).foregroundStyle(.secondary).lineLimit(2).fixedSize(horizontal: false, vertical: true)
                    }
                }
            }
            .layoutPriority(1)
            Spacer(minLength: Token.Space.gap)
            trailing
        }
        .frame(minHeight: Token.Tile.row)
        .padding(.vertical, 2)
        .stationHover()
    }

    public var body: some View {
        if let tap {
            Button(action: tap) { line }.buttonStyle(.plain)
        } else {
            line
        }
    }
}

extension MenuRow where Trailing == EmptyView {
    public init(_ title: String, subtitle: String? = nil, link: Bool = false, tap: (() -> Void)? = nil, @ViewBuilder leading: () -> Leading) {
        self.init(title, subtitle: subtitle, link: link, tap: tap, leading: leading) { EmptyView() }
    }
}

// value: a row's value on its trailing edge: secondary, its digits
// monospaced.
public struct TrailingValue: View {
    let text: String

    public init(_ text: String) { self.text = text }

    public var body: some View {
        Text(text).font(.body).foregroundStyle(.secondary).monospacedDigit().lineLimit(1).truncationMode(.middle)
            .contentTransition(.numericText())
    }
}

// value: an address as Apple sets one under a title: monospaced, in the
// accent, cut in its middle.
public struct LinkText: View {
    let text: String
    let center: Bool
    @Environment(\.stationPalette) private var palette

    public init(_ text: String, center: Bool = false) {
        self.text = text
        self.center = center
    }

    public var body: some View {
        Text(text).font(.system(.caption, design: .monospaced)).foregroundStyle(palette.accent)
            .lineLimit(center ? 2 : 1).truncationMode(.middle).multilineTextAlignment(center ? .center : .leading)
            .textSelection(.enabled)
    }
}

// value: the chevron a row that opens a page ends with.
public struct Chevron: View {
    let down: Bool

    public init(down: Bool = false) { self.down = down }

    public var body: some View {
        Image(systemName: down ? Token.Symbol.down : Token.Symbol.forward).font(.system(size: 11, weight: .semibold)).foregroundStyle(.tertiary)
    }
}

// value: a machine's state as a dot: filled in its mark's colour, hollow
// when it is not running; the only colour a machine's line wears.
public struct StateDot: View {
    let mark: Mark?
    @Environment(\.stationPalette) private var palette

    public init(mark: Mark?) { self.mark = mark }

    public var body: some View {
        if let mark {
            if mark == .pending {
                PendingDot()
            } else if mark == .absent {
                Circle().strokeBorder(.tertiary, lineWidth: 1.2).frame(width: Token.Tile.dot, height: Token.Tile.dot)
            } else {
                Circle().fill(palette.of(mark)).frame(width: Token.Tile.dot, height: Token.Tile.dot)
            }
        }
    }
}

public enum StationRole { case plain, primary, danger }

// invariant: a button in the content as Apple's: small and bordered, what
// destroys in red, the page's one primary action prominent glass. Drawn
// still, where the renderer has no AppKit control, the same shape
// stands in.
public struct StationButton: View {
    let title: String
    let role: StationRole
    let wide: Bool
    let action: () -> Void
    @Environment(\.stationStill) private var still
    @Environment(\.isEnabled) private var enabled
    @Environment(\.stationPalette) private var palette

    public init(_ title: String, role: StationRole = .plain, wide: Bool = false, action: @escaping () -> Void = {}) {
        self.title = title
        self.role = role
        self.wide = wide
        self.action = action
    }

    var label: some View {
        Text(title).font(.body.weight(role == .primary ? .semibold : .regular)).lineLimit(1).fixedSize(horizontal: !wide, vertical: false)
            .frame(maxWidth: wide ? .infinity : nil)
    }

    public var body: some View {
        if still {
            drawn
        } else {
            switch role {
            case .primary: Button(action: action) { label }.buttonStyle(.glassProminent).controlSize(.regular)
            case .danger: Button(role: .destructive, action: action) { label }.buttonStyle(.bordered).controlSize(.regular)
            case .plain: Button(action: action) { label }.buttonStyle(.bordered).controlSize(.regular)
            }
        }
    }

    var drawn: some View {
        let ink: AnyShapeStyle = role == .primary ? AnyShapeStyle(palette.onTint) : role == .danger ? AnyShapeStyle(palette.bad) : AnyShapeStyle(.primary)
        return Button(action: action) {
            label
                .foregroundStyle(ink)
                .padding(.horizontal, 12)
                .frame(height: 24)
                .background {
                    if role == .primary {
                        Capsule().fill(palette.accent)
                    } else {
                        Capsule().fill(.fill.secondary)
                    }
                }
                .opacity(enabled ? 1 : 0.45)
                .contentShape(Capsule())
        }
        .buttonStyle(.plain)
    }
}

// invariant: a switch as macOS draws one, small, in the accent when on;
// drawn still, the same switch stands in for the AppKit control.
public struct StationSwitch: View {
    let on: Bool
    let tap: () -> Void
    @Environment(\.stationStill) private var still
    @Environment(\.stationPalette) private var palette

    public init(on: Bool, tap: @escaping () -> Void = {}) {
        self.on = on
        self.tap = tap
    }

    public var body: some View {
        if still {
            ZStack(alignment: on ? .trailing : .leading) {
                Capsule().fill(on ? AnyShapeStyle(palette.accent) : AnyShapeStyle(.fill.secondary)).frame(width: Token.Tile.switchWidth, height: Token.Tile.switchHeight)
                Circle().fill(palette.onTint).frame(width: Token.Tile.switchHeight - 2, height: Token.Tile.switchHeight - 2)
                    .shadow(color: .black.opacity(0.25), radius: 0.8, y: 0.5).padding(.horizontal, 1)
            }
        } else {
            Toggle("", isOn: Binding(get: { on }, set: { _ in tap() })).toggleStyle(.switch).controlSize(.small).labelsHidden()
        }
    }
}

// value: a text button, the accent's or, for what destroys, red.
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
                if let symbol { Image(systemName: symbol).font(.subheadline.weight(.medium)) }
                Text(label).font(.body).lineLimit(1).fixedSize()
            }
            .foregroundStyle(danger ? palette.bad : palette.accent)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

// value: a bare symbol that is a button (the header's gear, a link's copy
// and QR): secondary, a quaternary fill under the pointer, never glass.
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
            Image(systemName: symbol).font(.system(size: 12, weight: .medium))
                .foregroundStyle(picked ? AnyShapeStyle(palette.accent) : AnyShapeStyle(.secondary))
                .frame(width: 22, height: 22)
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

// value: the way back of a pushed page: the chevron and the page it goes
// back to, in the accent.
public struct BackButton: View {
    let title: String
    let action: () -> Void
    @Environment(\.stationPalette) private var palette

    public init(_ title: String, action: @escaping () -> Void) {
        self.title = title
        self.action = action
    }

    public var body: some View {
        Button(action: action) {
            HStack(spacing: 3) {
                Image(systemName: Token.Symbol.back).font(.subheadline.weight(.semibold))
                Text(title).font(.body)
            }
            .foregroundStyle(palette.accent)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

// value: a pushed page's head as the vision draws it: the way back, then
// its circle, its title and its words, what it ends with (a state).
public struct PageHeader<Trailing: View>: View {
    let back: String
    let title: String
    let subtitle: String?
    let symbol: String?
    let active: Bool
    let badge: String?
    let action: () -> Void
    let trailing: Trailing

    public init(back: String, title: String, subtitle: String? = nil, symbol: String? = nil, active: Bool = false, badge: String? = nil, action: @escaping () -> Void = {}, @ViewBuilder trailing: () -> Trailing) {
        self.back = back
        self.title = title
        self.subtitle = subtitle
        self.symbol = symbol
        self.active = active
        self.badge = badge
        self.action = action
        self.trailing = trailing()
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            BackButton(back, action: action)
            HStack(alignment: .center, spacing: 10) {
                if let symbol { IconCircle(symbol, active: active, size: Token.Tile.headCircle) }
                VStack(alignment: .leading, spacing: 2) {
                    HStack(alignment: .firstTextBaseline, spacing: 6) {
                        Text(title).font(.title3.weight(.semibold)).foregroundStyle(.primary)
                        if let badge { Text(badge).font(.subheadline).foregroundStyle(.secondary) }
                    }
                    if let subtitle, !subtitle.isEmpty {
                        Text(subtitle).font(.subheadline).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                    }
                }
                .layoutPriority(1)
                Spacer(minLength: 0)
                trailing
            }
        }
    }
}

extension PageHeader where Trailing == EmptyView {
    public init(back: String, title: String, subtitle: String? = nil, symbol: String? = nil, active: Bool = false, badge: String? = nil, action: @escaping () -> Void = {}) {
        self.init(back: back, title: title, subtitle: subtitle, symbol: symbol, active: active, badge: badge, action: action) { EmptyView() }
    }
}

// value: a machine's state word in its colour, as the vision's head says
// "Running".
public struct StateWord: View {
    let word: String
    let ink: Color

    public init(_ word: String, ink: Color) {
        self.word = word
        self.ink = ink
    }

    public var body: some View {
        Text(word).font(.subheadline.weight(.semibold)).foregroundStyle(ink).lineLimit(1).fixedSize()
    }
}

// value: the inset divider between two sections, never between rows.
public struct SectionDivider: View {
    public init() {}

    public var body: some View { Divider().padding(.horizontal, 2) }
}

// value: a short note under a section: footnote, secondary.
public struct FootNote: View {
    let text: String

    public init(_ text: String) { self.text = text }

    public var body: some View {
        Text(text).font(.subheadline).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
    }
}
