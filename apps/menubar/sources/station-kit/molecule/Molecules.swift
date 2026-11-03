import SwiftUI

public struct Layout {
    static let charWidth: CGFloat = 6.6
    static let markWidth: CGFloat = 12

    static func chars(_ v: Value?) -> Int { v.map { Format.show($0).count } ?? 0 }

    // invariant: a row that does not fit drops its cells by Keep, lowest
    // first, never its mark nor its label (G4's order).
    public static func kept(_ row: Row, columns: [Column], width: CGFloat) -> [Int] {
        let cells = row.cells ?? []
        var keep = Array(cells.indices)
        let room = Int((width - markWidth - Token.Space.gap) / charWidth)
        func need() -> Int {
            (row.short ?? row.label ?? "").count + keep.reduce(0) { $0 + chars(cells[$1]) + 2 } + chars(row.detail) + 2
        }
        let order = cells.indices.sorted { a, b in
            let ka = a + 1 < columns.count ? columns[a + 1].keep ?? 0 : 0
            let kb = b + 1 < columns.count ? columns[b + 1].keep ?? 0 : 0
            return ka == kb ? a > b : ka < kb
        }
        for i in order where need() > room {
            keep.removeAll { $0 == i }
        }
        return keep
    }
}

public struct RowView: View {
    let row: Row
    let columns: [Column]
    let width: CGFloat
    let hintAsButton: Bool
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationHints) private var hints

    public init(_ row: Row, columns: [Column], width: CGFloat, hintAsButton: Bool = false) {
        self.row = row
        self.columns = columns
        self.width = width
        self.hintAsButton = hintAsButton
    }

    public var body: some View {
        let cells = row.cells ?? []
        VStack(alignment: .leading, spacing: 2) {
            HStack(spacing: Token.Space.gap) {
                HStack(spacing: Token.Space.gap / 2) {
                    MarkView(row.mark)
                    Text(row.short ?? row.label ?? "")
                        .font(.system(size: Token.Space.text))
                        .foregroundStyle(palette.text)
                        .lineLimit(1)
                }
                .layoutPriority(1)
                ForEach(Layout.kept(row, columns: columns, width: width), id: \.self) { i in
                    ValueText(cells[i], muted: i > 0)
                }
                Spacer(minLength: Token.Space.gap)
                if let detail = row.detail, !detail.isNone {
                    ValueText(detail, muted: true).layoutPriority(-1)
                }
            }
            .frame(minHeight: Token.Space.row - 4)
            if hints, let hint = row.hint {
                HintView(hint, asButton: hintAsButton).padding(.leading, Layout.markWidth + Token.Space.gap / 2)
            }
        }
    }
}

public struct FactView: View {
    let fact: Fact
    @Environment(\.stationPalette) private var palette

    public init(_ fact: Fact) { self.fact = fact }

    public var body: some View {
        HStack(spacing: Token.Space.gap) {
            Text(fact.key ?? "").font(.system(size: Token.Space.text)).foregroundStyle(palette.muted)
            ValueText(fact.value)
            Spacer(minLength: 0)
        }
    }
}

public struct HintView: View {
    let hint: Hint
    let asButton: Bool
    @Environment(\.stationPalette) private var palette

    public init(_ hint: Hint, asButton: Bool = false) {
        self.hint = hint
        self.asButton = asButton
    }

    public var body: some View {
        HStack(spacing: Token.Space.gap / 2) {
            Text("\(Token.Glyph.arrow) \(hint.command ?? "")")
                .font(.system(size: Token.Space.small, design: .monospaced))
                .foregroundStyle(palette.muted)
                .lineLimit(2)
            if let text = hint.text {
                Text(text).font(.system(size: Token.Space.small)).foregroundStyle(palette.subtle)
            }
            if asButton {
                Button {
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(hint.command ?? "", forType: .string)
                } label: {
                    Image(systemName: Token.Symbol.copy).font(.system(size: Token.Space.small))
                }
                .buttonStyle(.plain)
                .foregroundStyle(palette.muted)
            }
        }
    }
}

public struct EmptyLine: View {
    let empty: Empty
    @Environment(\.stationPalette) private var palette
    @Environment(\.stationHints) private var hints

    public init(_ empty: Empty) { self.empty = empty }

    public var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            Text("no \(empty.noun ?? "")").font(.system(size: Token.Space.text)).foregroundStyle(palette.muted)
            if hints, let hint = empty.hint { HintView(hint) }
        }
    }
}

public struct ErrorLine: View {
    let error: PageError
    @Environment(\.stationPalette) private var palette

    public init(_ error: PageError) { self.error = error }

    public var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            HStack(spacing: Token.Space.gap / 2) {
                Image(systemName: Token.Symbol.error).font(.system(size: Token.Space.text)).foregroundStyle(palette.bad)
                Text(error.message ?? "").font(.system(size: Token.Space.text)).foregroundStyle(palette.text).fixedSize(horizontal: false, vertical: true)
            }
            if let hint = error.hint, !hint.isEmpty {
                Text("\(Token.Glyph.arrow) \(hint)").font(.system(size: Token.Space.small, design: .monospaced)).foregroundStyle(palette.muted)
            }
        }
    }
}

public struct CountsView: View {
    let counts: [Tally]
    @Environment(\.stationPalette) private var palette

    public init(_ counts: [Tally]) { self.counts = counts }

    public var body: some View {
        HStack(spacing: Token.Space.gap / 2) {
            ForEach(Array(counts.enumerated()), id: \.offset) { _, t in
                HStack(spacing: 2) {
                    Text("\(t.n ?? 0)").font(.system(size: Token.Space.small)).monospacedDigit().foregroundStyle(palette.muted)
                    MarkView(t.mark)
                }
            }
        }
    }
}

public struct ConfirmPanel: View {
    let confirm: Confirm
    let onAnswer: (Bool) -> Void
    @Environment(\.stationPalette) private var palette

    public init(_ confirm: Confirm, onAnswer: @escaping (Bool) -> Void) {
        self.confirm = confirm
        self.onAnswer = onAnswer
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(confirm.question ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
            ForEach(Array((confirm.lines ?? []).enumerated()), id: \.offset) { _, line in
                ValueText(line, muted: true)
            }
            HStack {
                Spacer()
                Button(confirm.no ?? "No") { onAnswer(false) }.keyboardShortcut(.defaultAction)
                Button(confirm.yes ?? "Yes") { onAnswer(true) }.foregroundStyle(confirm.danger == true ? palette.bad : palette.text)
            }
        }
    }
}

public struct FormView: View {
    let form: Form
    @Binding var answers: [String: String]
    @Environment(\.stationPalette) private var palette

    public init(_ form: Form, answers: Binding<[String: String]>) {
        self.form = form
        _answers = answers
    }

    func binding(_ f: Field) -> Binding<String> {
        Binding(get: { answers[f.key ?? ""] ?? f.default ?? "" }, set: { answers[f.key ?? ""] = $0 })
    }

    public var body: some View {
        HStack(spacing: Token.Space.gap) {
            ForEach(Array((form.fields ?? []).enumerated()), id: \.offset) { _, f in
                if f.kind == "confirm" {
                    Toggle(f.label ?? "", isOn: Binding(get: { binding(f).wrappedValue == "true" }, set: { binding(f).wrappedValue = $0 ? "true" : "false" }))
                        .toggleStyle(.checkbox)
                        .font(.system(size: Token.Space.small))
                } else {
                    Picker(f.label ?? "", selection: binding(f)) {
                        ForEach(Array((f.options ?? []).enumerated()), id: \.offset) { _, o in
                            Text(o.label ?? o.value ?? "").tag(o.value ?? "")
                        }
                    }
                    .pickerStyle(.menu)
                    .font(.system(size: Token.Space.small))
                    .fixedSize()
                }
            }
        }
    }
}
