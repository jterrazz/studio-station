import SwiftUI

// invariant: a field the owner types into: the text so far and a caret when
// drawn still (an offscreen snapshot), a live text field in the app.
public struct TypedField: View {
    let placeholder: String
    @Binding var text: String
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationStill) private var still

    public init(_ placeholder: String, text: Binding<String>) {
        self.placeholder = placeholder
        _text = text
    }

    public var body: some View {
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
        .font(.system(size: Token.Space.text))
        .padding(.horizontal, Token.Tile.padding)
        .padding(.vertical, 5)
        .background(RoundedRectangle(cornerRadius: Token.Tile.pill / 1.5).fill(palette.text.opacity(Token.Tile.fill)))
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
        HStack(spacing: 4) {
            ForEach(Array((field.options ?? []).enumerated()), id: \.offset) { _, o in
                let on = (o.value ?? "") == value
                Button { value = o.value ?? "" } label: {
                    Text(o.label ?? o.value ?? "")
                        .font(.system(size: Token.Space.small, weight: on ? .semibold : .regular))
                        .foregroundStyle(on ? palette.surface : palette.text)
                        .padding(.horizontal, Token.Tile.padding)
                        .padding(.vertical, 4)
                        .background(Capsule().fill(on ? palette.accent : palette.text.opacity(Token.Tile.fill)))
                }
                .buttonStyle(.plain)
            }
        }
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
                ValueText(line, muted: true)
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
        .padding(Token.Tile.padding + 2)
        .background(RoundedRectangle(cornerRadius: Token.Tile.radius).fill(palette.text.opacity(Token.Tile.sheet)))
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
        .padding(Token.Tile.padding + 2)
        .background(RoundedRectangle(cornerRadius: Token.Tile.radius).fill(palette.text.opacity(Token.Tile.sheet)))
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

    var fill: Color {
        switch role {
        case .plain: palette.text.opacity(Token.Tile.fill)
        case .accent: palette.accent
        case .danger: palette.bad
        }
    }

    public var body: some View {
        Button(action: action) {
            Text(title)
                .font(.system(size: Token.Space.small, weight: .semibold))
                .foregroundStyle(role == .plain ? palette.text : palette.surface)
                .padding(.horizontal, Token.Tile.padding + 2)
                .padding(.vertical, 5)
                .background(Capsule().fill(fill))
        }
        .buttonStyle(.plain)
    }
}
