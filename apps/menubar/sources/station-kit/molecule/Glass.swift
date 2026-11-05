import SwiftUI

// invariant: the menu bar's pages are Liquid Glass (VISION-v3 panels d, e,
// g): each group a rounded pane of translucent glass with a bright top
// edge, drawn from shapes alone so an offscreen snapshot renders it.
public struct GlassCard<Content: View>: View {
    let tint: Color?
    let content: Content
    @Environment(\.stationPalette) private var palette
    @Environment(\.colorScheme) private var scheme

    public init(tint: Color? = nil, @ViewBuilder content: () -> Content) {
        self.tint = tint
        self.content = content()
    }

    public var body: some View {
        let shape = RoundedRectangle(cornerRadius: Token.Glass.radius, style: .continuous)
        VStack(alignment: .leading, spacing: Token.Space.gap) { content }
            .padding(Token.Glass.padding)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(shape.fill((tint ?? palette.text).opacity(tint == nil ? Token.Glass.fill(scheme) : Token.Glass.tint)))
            .overlay(shape.strokeBorder(Token.Glass.edge(scheme), lineWidth: 0.5))
    }
}

public struct PushedHeader: View {
    let back: String
    let title: String
    let subtitle: String?
    let action: () -> Void
    @Environment(\.stationPalette) private var palette

    public init(back: String, title: String, subtitle: String? = nil, action: @escaping () -> Void = {}) {
        self.back = back
        self.title = title
        self.subtitle = subtitle
        self.action = action
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Button(action: action) {
                HStack(spacing: 4) {
                    Image(systemName: Token.Symbol.back).font(.system(size: Token.Space.small, weight: .semibold))
                    Text(back).font(.system(size: Token.Space.text, weight: .semibold))
                }
                .foregroundStyle(palette.accent)
            }
            .buttonStyle(.plain)
            HStack(alignment: .firstTextBaseline, spacing: Token.Space.gap) {
                Text(title).font(.system(size: Token.Space.title, weight: .bold)).foregroundStyle(palette.text)
                if let subtitle {
                    Text(subtitle).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).lineLimit(1)
                }
            }
        }
    }
}

public struct SectionHead: View {
    let text: String
    @Environment(\.stationPalette) private var palette

    public init(_ text: String) { self.text = text }

    public var body: some View {
        Text(text).font(.system(size: Token.Space.small, weight: .semibold)).foregroundStyle(palette.muted).padding(.horizontal, 4)
    }
}

public enum PillRole { case plain, primary, danger }

public struct Pill: View {
    let label: String
    let role: PillRole
    let picked: Bool
    let action: () -> Void
    @Environment(\.stationPalette) private var palette

    public init(_ label: String, role: PillRole = .plain, picked: Bool = false, action: @escaping () -> Void = {}) {
        self.label = label
        self.role = role
        self.picked = picked
        self.action = action
    }

    var ink: Color {
        switch role {
        case .primary: palette.accent
        case .danger: palette.bad
        case .plain: picked ? palette.surface : palette.text
        }
    }

    public var body: some View {
        Button(action: action) {
            Text(label).font(.system(size: Token.Space.small, weight: .medium)).foregroundStyle(ink)
                .padding(.horizontal, 10).padding(.vertical, 4)
                .background(Capsule().fill(picked ? palette.text : palette.text.opacity(0.1)))
        }
        .buttonStyle(.plain)
    }
}
