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
        public static let missing = "—"
        public static let slash = "/"
        public static let down = "!"
        public static let command = "⌘"
        public static let restart = "↺"
        public static let caret = "|"
        public static let received = "↓"
        public static let secret = "••••••"
        public static let add = "+ "
        public static let sent = "↑"
        public static let unbroken = "\u{2011}"
    }

    // value: the keys a menu item says and answers (vision a, b).
    public enum Key {
        public static func shown(_ key: String) -> String {
            guard key.hasPrefix("cmd+") else { return key.uppercased() }
            return Glyph.command + key.dropFirst(4).uppercased()
        }
    }

    // value: the measures of a panel drawn by Apple's menus (Wi-Fi,
    // Battery): 28 pt rows, a 24 pt icon tile with a 13 pt glyph whose
    // corner is concentric with the 6 pt hover, 16 pt between sections, a
    // 10 pt grouping concentric with the panel.
    public enum Tile {
        public static let row: CGFloat = 28
        public static let icon: CGFloat = 24
        public static let glyph: CGFloat = 13
        public static let corner: CGFloat = 6.5
        public static let headIcon: CGFloat = 32
        public static let hover: CGFloat = 6
        public static let group: CGFloat = 10
        public static let groupPadding: CGFloat = 10
        public static let section: CGFloat = 16
        public static let indent: CGFloat = 32
        public static let figure: CGFloat = 17
        public static let spark: CGFloat = 22
        public static let chart: CGFloat = 54
        public static let measure: CGFloat = 22
        public static let meter: CGFloat = 14
        public static let dot: CGFloat = 7
        public static let switchWidth: CGFloat = 26
        public static let switchHeight: CGFloat = 15
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
        public static let text: CGFloat = 13
        public static let small: CGFloat = 11
        public static let title: CGFloat = 19
        public static let head: CGFloat = 15
        public static let graphHeight: CGFloat = 40
    }

    // value: the panel an offscreen snapshot draws in place of the
    // MenuBarExtra's system glass, which the renderer cannot sample: a
    // translucent pane, its rim and its shadow over the vision's desk.
    public enum Glass {
        public static let window: CGFloat = 22

        public static func pane(_ scheme: ColorScheme) -> Color { scheme == .dark ? Token.hex(0x1E2024).opacity(0.78) : Token.hex(0xF4F5F7).opacity(0.7) }
        public static func sheen(_ scheme: ColorScheme) -> [Color] {
            [Color.white.opacity(scheme == .dark ? 0.03 : 0.12), Color.white.opacity(0)]
        }
        public static func edge(_ scheme: ColorScheme) -> Color { scheme == .dark ? Color.white.opacity(0.14) : Color.black.opacity(0.1) }
        // value: the chosen segment of a segmented control, raised off its
        // track as macOS draws it.
        public static func knob(_ scheme: ColorScheme) -> Color { scheme == .dark ? Color.white.opacity(0.26) : Color.white }
        public static func shade(_ scheme: ColorScheme) -> Color { Color.black.opacity(scheme == .dark ? 0.5 : 0.22) }

        // value: the fixed wallpaper an offscreen snapshot draws the panel
        // over, so its glass reads in the PNG (the vision's desk).
        public static func wallpaper(_ scheme: ColorScheme) -> [(Color, UnitPoint, CGFloat)] {
            let dark = scheme == .dark
            return [
                (Token.hex(dark ? 0x9A3F2A : 0xFF9D6B), UnitPoint(x: 0.12, y: 0.18), 0.65),
                (Token.hex(dark ? 0x1E4FA8 : 0x5AA9FF), UnitPoint(x: 0.88, y: 0.12), 0.6),
                (Token.hex(dark ? 0x0F6E62 : 0x2FC6A8), UnitPoint(x: 0.7, y: 0.95), 0.7),
                (Token.hex(dark ? 0x7C5A12 : 0xF7C948), UnitPoint(x: 0.2, y: 0.9), 0.6),
            ]
        }
        public static func wallBase(_ scheme: ColorScheme) -> [Color] {
            scheme == .dark ? [Token.hex(0x142A5C), Token.hex(0x1B2F55), Token.hex(0x4A2A26)] : [Token.hex(0x3B6FE0), Token.hex(0x7FB6F2), Token.hex(0xF2A477)]
        }
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
        public static let awake = "sun.max"
        public static let needs = "exclamationmark"
        public static let gear = "ellipsis"
        public static let server = "server.rack"
        public static let accounts = "person.2"
        public static let more = "ellipsis.circle"
        public static let copy = "doc.on.doc"
        public static let back = "chevron.left"
        public static let down = "chevron.down"
        public static let forward = "chevron.right"
        public static let mac = "desktopcomputer"
        public static let workspace = "shippingbox"
        public static let plus = "plus"
        public static let terminal = "terminal"
        public static let stop = "stop.fill"
        public static let search = "magnifyingglass"
        public static let check = "checkmark"
        public static let cpu = "cpu"
        public static let memory = "memorychip"
        public static let network = "arrow.up.arrow.down"
        public static let warn = "exclamationmark.triangle.fill"
        public static let share = "square.and.arrow.up"
        public static let up = "chevron.up"
        public static let health = "checkmark"
        public static let open = "arrow.up.right.square"
        public static let qr = "qrcode"
        public static let globe = "globe"
        public static let tailnet = "point.3.connected.trianglepath.dotted"
        public static let lid = "rectangle.compress.vertical"
        public static let battery = "battery.75percent"
        public static let thermo = "thermometer.medium"
        public static let screen = "display"
        public static let ports = "network"
        public static let trash = "trash"
        public static let copyLink = "doc.on.doc"
        public static let key = "key.fill"
        public static let settings = "gearshape"
        public static let activity = "chart.bar"
        public static let restart = "arrow.clockwise"
        public static let unlock = "lock.open"

        // value: a machine's symbol by its kind: a Mac, a server, a
        // workspace.
        public static func machine(_ kind: String?) -> String {
            switch kind {
            case "Mac", "mac": mac
            case "Server", "server": server
            default: workspace
            }
        }

        // value: the tile a known line of a machine's page wears.
        public static func line(_ id: String) -> String? {
            switch id {
            case "apps": "square.grid.2x2"
            case "settings": "gearshape"
            case "disk": "internaldrive"
            case "ports": ports
            case "ssh": terminal
            case "github", "claude", "npm": accounts
            default: nil
            }
        }

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

    // value: the system's own colours: the label styles for ink, one
    // accent, and a state colour only where a state means something
    // (healthy, needs you, failed).
    public struct Palette: Sendable {
        public let text, muted, accent, good, warn, bad, onTint, qrDark, qrLight: Color

        public func of(_ mark: Mark) -> Color {
            switch mark {
            case .ok: good
            case .attention: warn
            case .broken: bad
            case .absent, .pending, .offHere, .notKnown, .notForThisMachine: muted
            }
        }
    }

    static func hex(_ value: UInt32) -> Color {
        Color(.sRGB, red: Double((value >> 16) & 0xFF) / 255, green: Double((value >> 8) & 0xFF) / 255, blue: Double(value & 0xFF) / 255, opacity: 1)
    }

    public static func palette(_ scheme: ColorScheme) -> Palette {
        Palette(
            text: Color.primary, muted: Color.secondary,
            accent: Color.accentColor, good: Color.green, warn: Color.orange, bad: Color.red,
            onTint: Color.white, qrDark: hex(0x000000), qrLight: hex(0xFFFFFF))
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

    // value: the palette of the appearance macOS shows, light or dark.
    public func stationSystemTheme() -> some View { modifier(SystemTheme()) }
}

struct SystemTheme: ViewModifier {
    @Environment(\.colorScheme) private var scheme

    func body(content: Content) -> some View {
        content.environment(\.stationPalette, Token.palette(scheme))
    }
}
