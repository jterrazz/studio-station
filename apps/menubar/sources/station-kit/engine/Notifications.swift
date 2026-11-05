import Foundation

public protocol Poster: Sendable {
    func post(_ text: String)
}

public enum Notifications {
    public static let watch = "engine.events"

    // invariant: SPEC §10.4: a notification is an event of the engine's
    // events watch, which streams the closed list alone, posted in the
    // engine's words, once per event; a notice of the page watch draws the
    // status line and is never posted.
    public static func posted(_ event: StreamEvent, from watch: String) -> String? {
        guard watch == Self.watch, case .notice(let kind, let text) = event, !kind.isEmpty, !text.isEmpty else { return nil }
        return text
    }
}
