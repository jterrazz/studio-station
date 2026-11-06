import SwiftUI

// invariant: a field the owner types into: the text so far and a caret when
// drawn still (an offscreen snapshot), a live text field in the app.
public struct TypedField: View {
    let placeholder: String
    let symbol: String?
    let key: String?
    @Binding var text: String
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationStill) private var still
    @Environment(\.stationInCard) private var inCard

    public init(_ placeholder: String, text: Binding<String>, symbol: String? = nil, key: String? = nil) {
        self.placeholder = placeholder
        self.symbol = symbol
        self.key = key
        _text = text
    }

    public var body: some View {
        HStack(spacing: 7) {
            if let symbol { Image(systemName: symbol).font(.system(size: Token.Space.text, weight: .medium)).foregroundStyle(palette.muted) }
            field
            if let key {
                Text(key).font(.system(size: Token.Space.small - 1, weight: .semibold)).foregroundStyle(palette.muted)
                    .padding(.horizontal, 5).padding(.vertical, 1.5)
                    .overlay(RoundedRectangle(cornerRadius: 5, style: .continuous).strokeBorder(palette.subtle, lineWidth: 0.8))
            }
        }
        .font(.system(size: Token.Space.text))
        .padding(.horizontal, inCard ? 9 : Token.Glass.padding)
        .padding(.vertical, inCard ? 6 : 10)
        .modifier(FieldGround(inCard: inCard))
    }

    @ViewBuilder
    var field: some View {
        SwiftUI.Group {
            if still {
                HStack(spacing: 0) {
                    Text(text.isEmpty ? placeholder : text).foregroundStyle(text.isEmpty ? palette.subtle : palette.text)
                    Text(Token.Glyph.caret).foregroundStyle(palette.accent)
                    Spacer(minLength: 0)
                }
            } else {
                TextField(placeholder, text: $text).textFieldStyle(.plain)
            }
        }
    }
}

// value: where a field sits: on the panel, a card of its own, as the
// references' "Filter by name or port"; in a card, a flat inset.
struct FieldGround: ViewModifier {
    let inCard: Bool

    func body(content: Content) -> some View {
        if inCard { content.stationWell() } else { content.stationGlass(radius: Token.Glass.radius) }
    }
}

// value: where a sheet sits: on the panel, a card of its own; in a card,
// the rest of it under a hairline.
struct SheetGround: ViewModifier {
    @Environment(\.stationInCard) private var inCard
    @Environment(\.stationPalette) private var palette

    func body(content: Content) -> some View {
        if inCard {
            content.padding(.top, 10).overlay(alignment: .top) { Rectangle().fill(palette.faint).frame(height: 0.5) }
        } else {
            content.padding(Token.Glass.padding).stationGlass(radius: Token.Glass.radius)
        }
    }
}

// invariant: a select field drawn as its options side by side, the chosen
// one filled: every option is visible, and it draws offscreen.
public struct Choice: View {
    let field: Field
    @Binding var value: String
    @Environment(\.stationPalette) private var palette

    public init(_ field: Field, value: Binding<String>) {
        self.field = field
        _value = value
    }

    public var body: some View {
        Segments(options: (field.options ?? []).map { ($0.label ?? $0.value ?? "", ($0.value ?? "") == value, { [o = $0] in value = o.value ?? "" }) })
    }
}

// invariant: SPEC §9.8 in the menu bar: a sheet of the call's level: yes or
// no naming what happens, or a name to type whose confirm stays off until
// it matches exactly.
public struct ConfirmSheet: View {
    let confirm: Confirm
    @Binding var typed: String
    let onAnswer: (Bool) -> Void
    @Environment(\.stationPalette) private var palette

    public init(_ confirm: Confirm, typed: Binding<String>, onAnswer: @escaping (Bool) -> Void) {
        self.confirm = confirm
        _typed = typed
        self.onAnswer = onAnswer
    }

    public static func allows(_ confirm: Confirm, typed: String) -> Bool {
        guard let name = confirm.type else { return true }
        return typed == name
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(confirm.question ?? "").font(.system(size: Token.Space.text + 1, weight: .semibold)).foregroundStyle(palette.text)
            ForEach(Array((confirm.lines ?? []).enumerated()), id: \.offset) { _, line in
                ValueText(line, muted: true).fixedSize(horizontal: false, vertical: true)
            }
            if let name = confirm.type {
                TypedField(name, text: $typed)
            }
            HStack {
                Spacer()
                SheetButton(confirm.no ?? "Cancel", role: .plain) { onAnswer(false) }.keyboardShortcut(.cancelAction)
                SheetButton(confirm.yes ?? "Yes", role: confirm.danger == true ? .danger : .accent) { onAnswer(true) }
                    .disabled(!Self.allows(confirm, typed: typed))
                    .opacity(Self.allows(confirm, typed: typed) ? 1 : 0.4)
                    .keyboardShortcut(.defaultAction)
            }
        }
        .modifier(SheetGround())
    }
}

// invariant: a form as a sheet: each select as its options, each text as a
// field, then its action's own words to run it.
public struct FormSheet: View {
    let form: Form
    let run: String
    @Binding var answers: [String: String]
    let onAnswer: (Bool) -> Void
    @Environment(\.stationPalette) private var palette

    public init(_ form: Form, run: String, answers: Binding<[String: String]>, onAnswer: @escaping (Bool) -> Void) {
        self.form = form
        self.run = run
        _answers = answers
        self.onAnswer = onAnswer
    }

    func binding(_ f: Field) -> Binding<String> {
        Binding(get: { answers[f.key ?? ""] ?? f.default ?? "" }, set: { answers[f.key ?? ""] = $0 })
    }

    var complete: Bool {
        (form.fields ?? []).allSatisfy { f in f.required != true || !(binding(f).wrappedValue.isEmpty) }
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(form.title ?? "").font(.system(size: Token.Space.text + 1, weight: .semibold)).foregroundStyle(palette.text)
            ForEach(Array((form.fields ?? []).enumerated()), id: \.offset) { _, f in
                VStack(alignment: .leading, spacing: 3) {
                    Text(f.label ?? "").font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
                    if f.kind == "select" {
                        Choice(f, value: binding(f))
                        if let note = f.options?.first(where: { $0.value == binding(f).wrappedValue })?.note {
                            Text(note).font(.system(size: Token.Space.small)).foregroundStyle(palette.subtle)
                        }
                    } else {
                        TypedField(f.note ?? "", text: binding(f))
                    }
                }
            }
            HStack {
                Spacer()
                SheetButton("Cancel", role: .plain) { onAnswer(false) }.keyboardShortcut(.cancelAction)
                SheetButton(run, role: .accent) { onAnswer(true) }.disabled(!complete).opacity(complete ? 1 : 0.4).keyboardShortcut(.defaultAction)
            }
        }
        .modifier(SheetGround())
    }
}

public struct SheetButton: View {
    public enum Role { case plain, accent, danger }
    let title: String
    let role: Role
    let action: () -> Void
    @Environment(\.stationPalette) private var palette

    public init(_ title: String, role: Role, action: @escaping () -> Void) {
        self.title = title
        self.role = role
        self.action = action
    }

    public var body: some View {
        Pill(title, role: role == .accent ? .primary : role == .danger ? .danger : .plain, action: action)
    }
}

// value: a picker as the references draw one: its options as words side
// by side, the chosen one on a faint fill, no well around them.
public struct Segments: View {
    let options: [(label: String, on: Bool, tap: () -> Void)]
    @Environment(\.stationPalette) private var palette

    public init(options: [(label: String, on: Bool, tap: () -> Void)]) { self.options = options }

    public var body: some View {
        HStack(spacing: 4) {
            ForEach(Array(options.enumerated()), id: \.offset) { _, o in
                Button(action: o.tap) {
                    Text(o.label)
                        .font(.system(size: Token.Space.small + 1, weight: o.on ? .semibold : .medium))
                        .foregroundStyle(o.on ? palette.text : palette.muted)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 4)
                        .background(RoundedRectangle(cornerRadius: Token.Tile.hover - 2, style: .continuous).fill(o.on ? palette.hover : .clear))
                        .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
            }
        }
    }
}
