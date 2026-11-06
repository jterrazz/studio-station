import SwiftUI

// invariant: a field the owner types into: the text so far and a caret when
// drawn still (an offscreen snapshot), a live text field in the app.
public struct TypedField: View {
    let placeholder: String
    let symbol: String?
    let key: String?
    let autofocus: Bool
    let move: ((Int) -> Void)?
    let submit: (() -> Void)?
    @Binding var text: String
    @FocusState private var focused: Bool
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationStill) private var still
    @Environment(\.colorScheme) private var scheme

    // value: a field that takes the keys as it opens (the switcher's
    // search), its Up and Down moving through what it finds, its Return
    // opening it.
    public init(_ placeholder: String, text: Binding<String>, symbol: String? = nil, key: String? = nil, autofocus: Bool = false,
                move: ((Int) -> Void)? = nil, submit: (() -> Void)? = nil) {
        self.placeholder = placeholder
        self.symbol = symbol
        self.key = key
        self.autofocus = autofocus
        self.move = move
        self.submit = submit
        _text = text
    }

    public var body: some View {
        HStack(spacing: 8) {
            if let symbol { Image(systemName: symbol).font(.system(size: Token.Size.text, weight: .medium)).foregroundStyle(palette.glyph) }
            field
            if let key {
                Text(key).font(.system(size: Token.Size.key)).foregroundStyle(.secondary)
            }
        }
        .font(.system(size: Token.Size.text))
        .padding(.horizontal, 10).padding(.vertical, 7)
        .stationField()
    }

    @ViewBuilder
    var field: some View {
        SwiftUI.Group {
            if still {
                HStack(spacing: 2) {
                    if text.isEmpty { Rectangle().fill(palette.accent).frame(width: 1.5, height: 14) }
                    Text(text.isEmpty ? placeholder : text).foregroundStyle(text.isEmpty ? AnyShapeStyle(.secondary) : AnyShapeStyle(.primary))
                    if !text.isEmpty { Rectangle().fill(palette.accent).frame(width: 1.5, height: 14) }
                    Spacer(minLength: 0)
                }
            } else {
                TextField(placeholder, text: $text).textFieldStyle(.plain)
                    .focused($focused)
                    .onSubmit { submit?() }
                    .onKeyPress(.downArrow) {
                        guard let move else { return .ignored }
                        move(1)
                        return .handled
                    }
                    .onKeyPress(.upArrow) {
                        guard let move else { return .ignored }
                        move(-1)
                        return .handled
                    }
                    .onAppear {
                        guard autofocus else { return }
                        DispatchQueue.main.async { focused = true }
                    }
            }
        }
    }
}

// value: where a sheet sits: under a divider, in the flow of the page,
// never a card of its own.
struct SheetGround: ViewModifier {
    func body(content: Content) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            SectionDivider()
            content
        }
        .id(PanelScroll.sheetID)
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
            Text(confirm.question ?? "").font(.body.weight(.semibold)).foregroundStyle(.primary)
            ForEach(Array((confirm.lines ?? []).enumerated()), id: \.offset) { _, line in
                SubText(Format.show(line))
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
            Text(form.title ?? "").font(.body.weight(.semibold)).foregroundStyle(.primary)
            ForEach(Array((form.fields ?? []).enumerated()), id: \.offset) { _, f in
                VStack(alignment: .leading, spacing: 3) {
                    Text(f.label ?? "").font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
                    if f.kind == "select" {
                        Choice(f, value: binding(f))
                        if let note = f.options?.first(where: { $0.value == binding(f).wrappedValue })?.note {
                            Text(note).font(.system(size: Token.Space.small)).foregroundStyle(.tertiary)
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
        StationButton(title, role: role == .accent ? .primary : role == .danger ? .danger : .plain, action: action)
    }
}

// value: a picker as macOS's segmented control: its options side by side
// on a tertiary track, the chosen one raised.
public struct Segments: View {
    let options: [(label: String, on: Bool, tap: () -> Void)]
    @Environment(\.colorScheme) private var scheme

    public init(options: [(label: String, on: Bool, tap: () -> Void)]) { self.options = options }

    public var body: some View {
        HStack(spacing: 2) {
            ForEach(Array(options.enumerated()), id: \.offset) { _, o in
                Button(action: o.tap) {
                    Text(o.label)
                        .font(.system(size: Token.Size.segment, weight: o.on ? .semibold : .medium))
                        .foregroundStyle(.primary)
                        .lineLimit(1)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 5)
                        .background {
                            if o.on {
                                RoundedRectangle(cornerRadius: 8, style: .continuous).fill(Token.Fill.raised(scheme)).shadow(color: .black.opacity(0.12), radius: 1.5, y: 1)
                            }
                        }
                        .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
            }
        }
        .padding(2)
        .background(Token.Fill.faint(scheme), in: RoundedRectangle(cornerRadius: Token.Tile.fieldCorner, style: .continuous))
    }
}
