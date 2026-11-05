import AppKit
import StationKit
import SwiftUI
import XCTest

@MainActor
final class PortsTests: XCTestCase {
    func texts(_ values: [Value]?) -> [String] { (values ?? []).compactMap(\.text) }

    // invariant: PO-33: a shared port's row unfolds its link, its QR, Copy
    // link, which copies the link, Open, which opens it, and Stop sharing, a
    // y/n whose yes is the ports.stop-sharing call of that port.
    func testPortRowQR() async throws {
        let page = try Tree.page("menubar.vm.linux-1")
        let row = try XCTUnwrap(page.section("private")?.rows.first)
        let url = "https://jterrazz-macbook-pro.tail1234.ts.net:40001"
        XCTAssertEqual(page.section("private")?.cell(row, "url")?.text, url)
        let stop = try XCTUnwrap(page.actions(for: row.id ?? "").first)
        XCTAssertEqual(stop.label, "Stop sharing")
        XCTAssertEqual(stop.confirm?.question, "Stop sharing linux-1's port 5173?")
        XCTAssertEqual(stop.run?.call, "ports.stop-sharing")
        XCTAssertEqual(Client.input([:], fixed: stop.run?.in) as NSDictionary, ["VM": "linux-1", "Port": "5173", "Host": "macbook", "Confirm": "yes"] as NSDictionary)
        var copied: [String] = []
        var opened: [URL] = []
        var asked: [String?] = []
        let handlers = PopoverHandlers(confirm: { asked.append($0?.id) }, open: { opened.append($0) }, copy: { copied.append($0) })
        var state = PopoverState(page: page, connection: .live, qr: url)
        XCTAssertNil(try Golden.check("menubar.vm.linux-1.qr", try XCTUnwrap(MenuPage.view(state, handlers: handlers)), scheme: .dark))
        state.confirming = stop.id
        XCTAssertNil(try Golden.check("sheet.stop-sharing", try XCTUnwrap(MenuPage.view(state, handlers: handlers)), scheme: .dark))
        XCTAssertEqual(copied + opened.map(\.absoluteString) + asked.compactMap { $0 }, [])
        let link = PortsPage.link(try XCTUnwrap(URL(string: url)), handlers: handlers)
        XCTAssertEqual(link.map(\.label), ["Copy link", "Open"])
        link[0].tap()
        XCTAssertEqual(copied, [url])
        XCTAssertEqual(opened, [])
        link[1].tap()
        XCTAssertEqual(opened.map(\.absoluteString), [url])
    }

    // invariant: PO-34: mac-mini's Ports: red on top, then Public, then
    // Private, each row's class in words, never in colour alone.
    func testPortsTwoClasses() throws {
        let page = try Tree.page("menubar.ports.mini")
        XCTAssertEqual(page.sections?.compactMap(\.id).filter { ["needs-you", "public", "private"].contains($0) }, ["needs-you", "public", "private"])
        for (id, word) in [("needs-you", "needs you"), ("public", "public"), ("private", "private")] {
            let s = try XCTUnwrap(page.section(id))
            XCTAssertEqual(Set(s.rows.compactMap { s.cell($0, "class")?.text }), [word], id)
        }
        XCTAssertEqual(page.section("needs-you")?.rows.map(\.mark), [.broken, .broken])
        XCTAssertEqual(page.summary()?.label, "mac-mini")
        XCTAssertEqual(page.summary()?.cells?.first.map { Format.show($0) }, "5 ports · 2 need you")
    }

    // invariant: PO-45: Declare public… on mac-mini's Ports asks the port
    // and its route, red ones offered first, and sends ports.declare with
    // the port's machine and number as text.
    func testPortsDeclarePublic() throws {
        let page = try Tree.page("menubar.ports.mini")
        let declare = try XCTUnwrap(page.action("declare-public"))
        XCTAssertEqual(declare.run?.call, "ports.declare")
        let fields = declare.form?.fields ?? []
        XCTAssertEqual(fields.map(\.key), ["Port", "Via"])
        XCTAssertEqual(fields[0].options?.map(\.label), ["mac-mini:5000", "infra:10250", "mac-mini:5900", "infra:6443"])
        XCTAssertEqual(fields[1].options?.map(\.label), ["Cloudflare Tunnel", "Tailscale Funnel", "your LAN"])
        XCTAssertEqual(PortsPage.routes(declare), "To open a port on purpose, declare it public by Cloudflare Tunnel, Tailscale Funnel or your LAN.")
        let input = Client.input(ActionRunner.filled(declare, answers: [:]), fixed: declare.run?.in)
        XCTAssertEqual(input as NSDictionary, ["Machine": "mac-mini", "Port": "5000", "Via": "cloudflare-tunnel"] as NSDictionary)
        var state = Snapshot.state("menubar.ports.mini", page)
        state.confirming = declare.id
        XCTAssertNil(try Golden.check("sheet.declare-public", try XCTUnwrap(MenuPage.view(state)), scheme: .dark))
    }

    // invariant: PO-45: linux-1's row on Home opens linux-1's page, the
    // engine's watch of menubar.vm:linux-1; Share a port… there asks the port
    // as text and sends ports.share for linux-1, and a shared port's row
    // holds its link.
    func testWorkspaceSharePort() async throws {
        let home = try Tree.page("menubar.home.two-macs")
        let row = try XCTUnwrap(Self.row("linux-1", in: home.section("fleet")?.items ?? []))
        var pushed: [String] = []
        try XCTUnwrap(LinkActions(push: { pushed.append($0) }).tap(row))()
        XCTAssertEqual(pushed, ["menubar.vm:linux-1"])
        XCTAssertNil(LinkActions().tap(try XCTUnwrap(Self.row("infra", in: home.section("fleet")?.items ?? []))))
        let engine = try FakeEngine("mb-vm")
        defer { engine.stop() }
        engine.serveOnce(try Streamed.answer(pushed[0], "menubar.vm.linux-1"), thenClose: false)
        let shown = Shown()
        let nav = Navigation(socket: engine.path, clock: TestClock()) { shown.add($0, $1) }
        nav.wire(PopoverHandlers()).push(pushed[0])
        for _ in 0..<500 where shown.all.isEmpty { try await Task.sleep(nanoseconds: 10_000_000) }
        XCTAssertEqual(engine.received, [Streamed.watch("menubar.vm:linux-1")])
        XCTAssertEqual(shown.all, ["menubar.vm:linux-1 menubar.vm"])
        nav.back()

        let page = try Tree.page("menubar.vm.linux-1")
        XCTAssertNil(page.action("declare-public"))
        let share = try XCTUnwrap(page.action("share-port"))
        XCTAssertEqual(share.run?.call, "ports.share")
        XCTAssertEqual(share.form?.fields?.map(\.kind), ["text"])
        let text = Set(share.form?.fields?.compactMap(\.key) ?? [])
        let input = Client.input(ActionRunner.filled(share, answers: ["Port": "3000"]), fixed: share.run?.in, text: text)
        XCTAssertEqual(input as NSDictionary, ["VM": "linux-1", "Port": "3000"] as NSDictionary)
        XCTAssertTrue(input["Port"] is String)
        let private_ = try XCTUnwrap(page.section("private"))
        let vite = try XCTUnwrap(private_.rows.first { private_.cell($0, "port")?.n == 5173 })
        XCTAssertEqual(private_.cell(vite, "url")?.text, "https://jterrazz-macbook-pro.tail1234.ts.net:40001")
        var state = Snapshot.state("menubar.vm.linux-1", page)
        state.confirming = share.id
        state.answers = ["share-port": ["Port": "3000"]]
        XCTAssertNil(try Golden.check("sheet.share-port", try XCTUnwrap(MenuPage.view(state)), scheme: .dark))
    }

    static func row(_ id: String, in items: [Item]) -> Row? {
        for item in items {
            if item.row?.id == id { return item.row }
            if item.group?.head?.id == id { return item.group?.head }
            if let found = row(id, in: item.group?.items ?? []) { return found }
        }
        return nil
    }

    // invariant: MB-17: Review on a red row of mac-mini's Ports opens a new
    // terminal on station ports --on mac-mini, and starts nothing else.
    func testPortsReviewOpensTerminal() async throws {
        let page = try Tree.page("menubar.ports.mini")
        let red = try XCTUnwrap(page.section("needs-you")?.rows.first)
        let review = try XCTUnwrap(page.actions(for: red.id ?? "").first)
        XCTAssertEqual(review.label, "Review")
        let calls = Recorded()
        let status = await ActionRunner(executable: "/u/bin/station", socket: "/nonexistent", launcher: RecordingLauncher(calls: calls)).run(review)
        XCTAssertNil(status)
        XCTAssertEqual(calls.all, [
            ["/usr/bin/osascript", "-e", #"tell application "Terminal" to do script "'/u/bin/station' 'ports' '--on' 'mac-mini'""#, "-e", #"tell application "Terminal" to activate"#],
        ])
    }
}
