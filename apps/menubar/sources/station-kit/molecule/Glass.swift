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

// invariant: a text answer is drawn as its field; offscreen (still) it is
// the field's shape and its text, since an AppKit field renders no image.
public struct TextAnswer: View {
    let placeholder: String
    @Binding var text: String
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationStill) private var still

    public init(_ placeholder: String, text: Binding<String>) {
        self.placeholder = placeholder
        _text = text
    }

    public var body: some View {
        let shape = RoundedRectangle(cornerRadius: 6, style: .continuous)
        SwiftUI.Group {
            if still {
                Text(text.isEmpty ? placeholder : text).foregroundStyle(text.isEmpty ? palette.subtle : palette.text)
                    .frame(maxWidth: .infinity, alignment: .leading)
            } else {
                TextField(placeholder, text: $text).textFieldStyle(.plain)
            }
        }
        .font(.system(size: Token.Space.text))
        .padding(.horizontal, 8).padding(.vertical, 4)
        .background(shape.fill(palette.text.opacity(0.08)))
        .overlay(shape.strokeBorder(palette.text.opacity(0.15), lineWidth: 0.5))
    }
}

public struct Choices: View {
    let field: Field
    @Binding var answer: String
    @Environment(\.stationPalette) private var palette

    public init(_ field: Field, answer: Binding<String>) {
        self.field = field
        _answer = answer
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(field.label ?? "").font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
            FlowRow(spacing: 4) {
                ForEach(Array((field.options ?? []).enumerated()), id: \.offset) { _, o in
                    Pill(o.label ?? o.value ?? "", picked: (answer.isEmpty ? field.default ?? "" : answer) == o.value) { answer = o.value ?? "" }
                }
            }
        }
    }
}

// invariant: a sheet asks its form's fields, every select a row of
// choices and every text a field, then its two answers.
public struct FormSheet: View {
    let action: Action
    let answers: [String: String]
    let answer: (String, String) -> Void
    let done: (Bool) -> Void
    @Environment(\.stationPalette) private var palette

    public init(_ action: Action, answers: [String: String], answer: @escaping (String, String) -> Void, done: @escaping (Bool) -> Void) {
        self.action = action
        self.answers = answers
        self.answer = answer
        self.done = done
    }

    func binding(_ f: Field) -> Binding<String> {
        Binding(get: { answers[f.key ?? ""] ?? "" }, set: { answer(f.key ?? "", $0) })
    }

    public var body: some View {
        GlassCard {
            Text(action.form?.title ?? action.label ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
            ForEach(Array((action.form?.fields ?? []).enumerated()), id: \.offset) { _, f in
                if f.kind == "text" {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(f.label ?? "").font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
                        TextAnswer(f.note ?? "", text: binding(f))
                    }
                } else {
                    Choices(f, answer: binding(f))
                }
            }
            HStack {
                Spacer()
                Pill("Cancel") { done(false) }
                Pill(action.label ?? "", role: .primary) { done(true) }
            }
        }
    }
}

// invariant: a y/n sheet says its question and every line whole, then its
// two answers, the yes in red when it takes something away.
public struct ConfirmSheet: View {
    let confirm: Confirm
    let answer: (Bool) -> Void
    @Environment(\.stationPalette) private var palette

    public init(_ confirm: Confirm, answer: @escaping (Bool) -> Void) {
        self.confirm = confirm
        self.answer = answer
    }

    public var body: some View {
        GlassCard {
            Text(confirm.question ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
            ForEach(Array((confirm.lines ?? []).enumerated()), id: \.offset) { _, line in
                Text(Format.show(line)).font(.system(size: Token.Space.small)).foregroundStyle(palette.muted).fixedSize(horizontal: false, vertical: true)
            }
            HStack {
                Spacer()
                Pill(confirm.no ?? "No") { answer(false) }
                Pill(confirm.yes ?? "Yes", role: confirm.danger == true ? .danger : .primary) { answer(true) }
            }
        }
    }
}

public struct FlowRow: SwiftUI.Layout {
    let spacing: CGFloat

    public init(spacing: CGFloat) { self.spacing = spacing }

    public func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let width = proposal.width ?? .infinity
        var x: CGFloat = 0, y: CGFloat = 0, line: CGFloat = 0, widest: CGFloat = 0
        for s in subviews {
            let size = s.sizeThatFits(.unspecified)
            if x > 0 && x + size.width > width {
                y += line + spacing
                x = 0
                line = 0
            }
            x += size.width + spacing
            line = max(line, size.height)
            widest = max(widest, x - spacing)
        }
        return CGSize(width: min(widest, width), height: y + line)
    }

    public func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        var x = bounds.minX, y = bounds.minY, line: CGFloat = 0
        for s in subviews {
            let size = s.sizeThatFits(.unspecified)
            if x > bounds.minX && x + size.width > bounds.maxX {
                y += line + spacing
                x = bounds.minX
                line = 0
            }
            s.place(at: CGPoint(x: x, y: y), proposal: ProposedViewSize(size))
            x += size.width + spacing
            line = max(line, size.height)
        }
    }
}
