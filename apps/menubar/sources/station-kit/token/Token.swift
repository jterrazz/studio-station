import SwiftUI

public enum Token {
    public enum Glyph {
        public static let arrow = "→"
        public static let sep = " · "
        public static let ellipsis = "…"
        public static let power = "⏻"
        public static let charging = "⚡"
        public static let unknown = "?"
        public static let dash = "-"
        public static let slash = "/"
        public static let down = "!"
        public static let command = "⌘"
        public static let restart = "↺"
        public static let caret = "|"
        public static let received = "↓"
        public static let sent = "↑"
    }

    // value: the keys a menu item says and answers (vision a, b).
    public enum Key {
        public static func shown(_ key: String) -> String {
            guard key.hasPrefix("cmd+") else { return key.uppercased() }
            return Glyph.command + key.dropFirst(4).uppercased()
        }
    }

    // value: the tiles and pills of the popover (vision a-f): rounded
    // surfaces a step lighter than the panel.
    public enum Tile {
        public static let radius: CGFloat = 12
        public static let padding: CGFloat = 8
        public static let pill: CGFloat = 10
        public static let title: CGFloat = 17
        public static let sparkHeight: CGFloat = 18
        public static let fill = 0.07
        public static let sheet = 0.12
    }

    // value: the menu bar icon of the vision: the station symbol, 16 pt, its
    // badge on its top right corner, and the CPU line beside it.
    public enum Icon {
        public static let symbol: CGFloat = 16
        public static let line: CGFloat = 1.6
        public static let badge: CGFloat = 14
        public static let badgeText: CGFloat = 9.5
        public static let badgeShift = CGSize(width: 5, height: -3)
        public static let sparkWidth: CGFloat = 22
        public static let sparkHeight: CGFloat = 12
        public static let gap: CGFloat = 4
        public static let dimmed = 0.45

        public static func needs(_ scheme: ColorScheme) -> Color { Token.hex(scheme == .dark ? 0xFFAD1F : 0xF29D0B) }
        public static func down(_ scheme: ColorScheme) -> Color { Token.hex(scheme == .dark ? 0xFF4F45 : 0xEC3B30) }
        public static let badgeInk = Token.hex(0xFFFFFF)
        public static func ink(_ scheme: ColorScheme) -> Color { Token.hex(scheme == .dark ? 0xFFFFFF : 0x000000) }
    }

    public enum Space {
        public static let margin: CGFloat = 12
        public static let indent: CGFloat = 12
        public static let gap: CGFloat = 8
        public static let row: CGFloat = 22
        public static let width: CGFloat = 340
        public static let cap: CGFloat = 560
        public static let qr: CGFloat = 160
        public static let sparkWidth: CGFloat = 48
        public static let sparkHeight: CGFloat = 12
        public static let meterWidth: CGFloat = 36
        public static let meterHeight: CGFloat = 5
        public static let mark: CGFloat = 10
        public static let text: CGFloat = 12
        public static let small: CGFloat = 11
        public static let title: CGFloat = 15
        public static let graphHeight: CGFloat = 34
    }

    // value: the glass of the vision's panels: a pane's corner, its inner
    // margin, how much of the ink tints it, and its bright edge.
    public enum Glass {
        public static let radius: CGFloat = 14
        public static let padding: CGFloat = 10
        public static let tint = 0.14
        public static func fill(_ scheme: ColorScheme) -> Double { scheme == .dark ? 0.07 : 0.05 }
        public static func edge(_ scheme: ColorScheme) -> Color { Color.white.opacity(scheme == .dark ? 0.16 : 0.6) }
    }

    public enum Symbol {
        public static let ok = "circle.fill"
        public static let attention = "circle.lefthalf.filled"
        public static let absent = "circle"
        public static let broken = "xmark.circle.fill"
        public static let pending = "circle.dashed"
        public static let offHere = "nosign"
        public static let notForThisMachine = "minus"
        public static let notKnown = "questionmark"
        public static let error = "xmark.octagon"
        public static let stopped = "stop.circle"
        public static let awake = "power.circle.fill"
        public static let more = "ellipsis.circle"
        public static let copy = "doc.on.doc"
        public static let back = "chevron.left"
        public static let down = "chevron.down"
        public static let forward = "chevron.right"
        public static let mac = "desktopcomputer"
        public static let vm = "shippingbox"
        public static let plus = "plus"
        public static let terminal = "terminal"
        public static let stop = "stop.fill"
        public static let search = "magnifyingglass"
        public static let check = "checkmark"
        public static let sun = "sun.max"
        public static let person = "person.crop.circle"
        public static let warn = "exclamationmark.triangle"
        public static let open = "arrow.up.right.square"
        public static let qr = "qrcode"
        public static let globe = "globe"
        public static let tailnet = "point.3.connected.trianglepath.dotted"
        public static let lid = "rectangle.compress.vertical"
        public static let battery = "battery.75percent"
        public static let thermo = "thermometer.medium"
        public static let screen = "display"

        public static func of(_ mark: Mark) -> String {
            switch mark {
            case .ok: ok
            case .attention: attention
            case .absent: absent
            case .broken: broken
            case .pending: pending
            case .offHere: offHere
            case .notForThisMachine: notForThisMachine
            case .notKnown: notKnown
            }
        }
    }

    public struct Palette: Sendable {
        public let text, muted, subtle, surface, selection, accent, good, warn, bad, qrDark, qrLight: Color

        public func of(_ mark: Mark) -> Color {
            switch mark {
            case .ok: good
            case .attention: warn
            case .broken: bad
            case .absent, .pending, .offHere, .notKnown: muted
            case .notForThisMachine: subtle
            }
        }
    }

    private static func hex(_ value: UInt32) -> Color {
        Color(.sRGB, red: Double((value >> 16) & 0xFF) / 255, green: Double((value >> 8) & 0xFF) / 255, blue: Double(value & 0xFF) / 255, opacity: 1)
    }

    public static func palette(_ scheme: ColorScheme) -> Palette {
        let dark = scheme == .dark
        let pair = { (light: UInt32, night: UInt32) in hex(dark ? night : light) }
        return Palette(
            text: pair(0x4C4F69, 0xCAD3F5), muted: pair(0x6C6F85, 0xA5ADCB), subtle: pair(0x9CA0B0, 0x6E738D),
            surface: pair(0xEFF1F5, 0x24273A), selection: pair(0xBCC0CC, 0x494D64), accent: pair(0x1E66F5, 0x8AADF4),
            good: pair(0x40A02B, 0xA6DA95), warn: pair(0xDF8E1D, 0xEED49F), bad: pair(0xD20F39, 0xED8796),
            qrDark: hex(0x000000), qrLight: hex(0xFFFFFF))
    }
}

private struct PaletteKey: EnvironmentKey {
    static let defaultValue = Token.palette(.dark)
}

private struct HintsKey: EnvironmentKey {
    static let defaultValue = true
}

private struct StillKey: EnvironmentKey {
    static let defaultValue = false
}

private struct NowKey: EnvironmentKey {
    static let defaultValue: Date? = nil
}

extension EnvironmentValues {
    public var stationPalette: Token.Palette {
        get { self[PaletteKey.self] }
        set { self[PaletteKey.self] = newValue }
    }

    public var stationHints: Bool {
        get { self[HintsKey.self] }
        set { self[HintsKey.self] = newValue }
    }

    public var stationStill: Bool {
        get { self[StillKey.self] }
        set { self[StillKey.self] = newValue }
    }

    public var stationNow: Date? {
        get { self[NowKey.self] }
        set { self[NowKey.self] = newValue }
    }
}

extension View {
    public func stationTheme(_ scheme: ColorScheme) -> some View {
        environment(\.colorScheme, scheme).environment(\.stationPalette, Token.palette(scheme))
    }
}
