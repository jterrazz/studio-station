import SwiftUI

public struct LinkActions {
    public var open: (URL) -> Void
    public var qr: String?
    public var toggleQR: (String) -> Void

    public init(open: @escaping (URL) -> Void = { _ in }, qr: String? = nil, toggleQR: @escaping (String) -> Void = { _ in }) {
        self.open = open
        self.qr = qr
        self.toggleQR = toggleQR
    }
}

public struct LinkView: View {
    let link: Link
    let actions: LinkActions?
    @Environment(\.stationPalette) private var palette

    public init(_ link: Link, actions: LinkActions?) {
        self.link = link
        self.actions = actions
    }

    var key: String { link.label ?? Format.show(link.lead ?? Value()) }

    public var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack(spacing: Token.Space.gap) {
                if let lead = link.lead, !lead.isNone {
                    ValueText(lead)
                } else {
                    Text(link.label ?? "").font(.system(size: Token.Space.text)).foregroundStyle(palette.accent)
                }
                Spacer(minLength: Token.Space.gap)
                if let actions, let url = link.value.flatMap(Format.url) {
                    Button("open") { actions.open(url) }.buttonStyle(.plain).foregroundStyle(palette.accent).font(.system(size: Token.Space.small))
                    Text(Token.Glyph.sep.trimmingCharacters(in: .whitespaces)).foregroundStyle(palette.subtle)
                    Button("QR") { actions.toggleQR(url.absoluteString) }.buttonStyle(.plain).foregroundStyle(palette.accent).font(.system(size: Token.Space.small))
                } else if let value = link.value {
                    ValueText(value, muted: true)
                }
            }
            if let actions, let url = link.value.flatMap(Format.url), actions.qr == url.absoluteString {
                QRView(url.absoluteString)
            }
        }
    }
}

public struct ItemsView: View {
    let items: [Item]
    let columns: [Column]
    let width: CGFloat
    let links: LinkActions?
    let hintButtons: Bool

    public init(_ items: [Item], columns: [Column], width: CGFloat, links: LinkActions? = nil, hintButtons: Bool = false) {
        self.items = items
        self.columns = columns
        self.width = width
        self.links = links
        self.hintButtons = hintButtons
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            ForEach(Array(items.enumerated()), id: \.offset) { _, item in
                ItemView(item: item, columns: columns, width: width, links: links, hintButtons: hintButtons)
            }
        }
    }
}

struct ItemView: View {
    let item: Item
    let columns: [Column]
    let width: CGFloat
    let links: LinkActions?
    let hintButtons: Bool
    @Environment(\.stationPalette) private var palette

    var body: some View {
        if let row = item.row {
            RowView(row, columns: columns, width: width, hintAsButton: hintButtons)
        } else if let fact = item.fact {
            FactView(fact)
        } else if let group = item.group {
            GroupView(group: group, columns: columns, width: width, links: links, hintButtons: hintButtons)
        } else if let empty = item.empty {
            EmptyLine(empty)
        } else if let error = item.error {
            ErrorLine(error)
        } else if let note = item.note {
            Text(note.text ?? "").font(.system(size: Token.Space.small)).foregroundStyle(palette.muted)
        } else if let link = item.link {
            LinkView(link, actions: links)
        }
    }
}

struct GroupView: View {
    let group: Group
    let columns: [Column]
    let width: CGFloat
    let links: LinkActions?
    let hintButtons: Bool

    var children: some View {
        ItemsView(group.items ?? [], columns: columns, width: width - Token.Space.indent, links: links, hintButtons: hintButtons)
            .padding(.leading, Token.Space.indent)
    }

    var body: some View {
        let head = RowView(group.head ?? Row(), columns: columns, width: width, hintAsButton: hintButtons)
        if group.fold == nil || group.fold == "open" {
            VStack(alignment: .leading, spacing: 2) {
                head
                children
            }
        } else {
            DisclosureGroup { children } label: { head }
        }
    }
}

public struct SectionView: View {
    let section: Section
    let width: CGFloat
    let links: LinkActions?
    let hintButtons: Bool
    let title: String?
    @Environment(\.stationPalette) private var palette

    public init(_ section: Section, width: CGFloat, links: LinkActions? = nil, hintButtons: Bool = false, title: String? = nil) {
        self.section = section
        self.width = width
        self.links = links
        self.hintButtons = hintButtons
        self.title = title
    }

    var head: some View {
        HStack(spacing: Token.Space.gap) {
            Text(title ?? section.title ?? "").font(.system(size: Token.Space.text, weight: .semibold)).foregroundStyle(palette.text)
            ForEach(Array((section.caption ?? []).enumerated()), id: \.offset) { _, c in ValueText(c, muted: true) }
            Spacer(minLength: 0)
            CountsView(section.counts ?? [])
        }
    }

    var items: some View {
        ItemsView(section.items ?? [], columns: section.columns ?? [], width: width, links: links, hintButtons: hintButtons)
    }

    public var body: some View {
        if section.fold == "folded" {
            DisclosureGroup { items } label: { head }
        } else {
            VStack(alignment: .leading, spacing: 4) {
                head
                items
            }
        }
    }
}

public struct StripView: View {
    let strip: Strip
    @Environment(\.stationPalette) private var palette

    public init(_ strip: Strip) { self.strip = strip }

    public var body: some View {
        let size = Token.Space.small
        HStack(spacing: Token.Space.gap - 2) {
            if let spark = strip.spark, !spark.isEmpty { Spark(spark) }
            ValueText(strip.cpu, size: size).fixedSize()
            if let memory = strip.memory {
                if memory.kind == "ratio", let of = memory.of, of > 0 {
                    Meter(Double(memory.n ?? 0) / Double(of))
                }
                ValueText(memory, size: size).fixedSize()
            }
            if let battery = strip.battery {
                HStack(spacing: 2) {
                    Text(Token.Glyph.charging).font(.system(size: size)).foregroundStyle(palette.muted)
                    ValueText(battery, size: size)
                }
                .fixedSize()
            }
            if let power = strip.power, !power.isNone {
                Text("\(Token.Glyph.power) \(Format.show(power.items?.last ?? power))")
                    .font(.system(size: size, weight: .semibold)).foregroundStyle(palette.accent)
                    .lineLimit(1).fixedSize()
            }
            Spacer(minLength: 0)
        }
    }
}

public struct StatusBar: View {
    let line: String
    let failed: Bool
    @Environment(\.stationPalette) private var palette

    public init(_ line: String, failed: Bool) {
        self.line = line
        self.failed = failed
    }

    public var body: some View {
        HStack(spacing: Token.Space.gap / 2) {
            if failed { MarkView(.broken) }
            Text(line).font(.system(size: Token.Space.small)).foregroundStyle(failed ? palette.text : palette.muted).lineLimit(2)
            Spacer(minLength: 0)
        }
    }
}
