# TODO

Decided with the owner on 2026-10-31 and left for later, each with what starts it.

## Choose the secret manager
Bitwarden Secrets Manager or 1Password service accounts. It becomes the home
of LENT secrets beyond `session:*` (`operator:*` tokens scoped per VM), so
`station secrets allow` stops refusing non-session names. Starts when a VM
first needs a token that is not a browser session.

## The macOS VM and the iPhone (on the MacBook M5 Pro, from 2026-11-07)
Run the PoC of ENGINE.md "Risks" 1, 5, 6, 7 for a macOS guest: pull
`macos-tahoe-xcode:27`, `--provisioning-opts` instead of admin/admin, the
`kern.hv_vmm_present` = 1 for kind detection, `station guest` on macOS
(`lsof`), a build deployed to the iPhone over `station machines create
--network bridged` (`--net-bridged=<default interface>`; pairing: cable on the
host, or Accessory Access; N8, unproven). No tailscale inside (round 5). Then add the `macos` image (recipe, first-boot), move
lift `images build`'s linux-only list: xcode, cocoapods, xcodegen,
android-studio and lens are placed `macos` (and `host` as exploration since
2026-11-01) and install there from then on. If the iPhone cannot
be reached from the VM, the owner decides how the install step reaches it
(a host Xcode is not an option he has accepted).

## Machine test tiers: the nightly
Tiers 2 and 3 are built (`make e2e-machine`, [Testing](docs/guides/test.md#the-machine-specs)).
What waits is tier 3 as a NIGHTLY on the Mac mini, once a Tart VM may run
there.

## The Mac mini as a host
The owner finishes this first, so the new MacBook stays ultra clean (round 3).
When its 256 GB disk is upgraded: `station setup --name mac-mini`
there, `tart run` under launchd with autologin and no screen (ENGINE.md risk
4), Linux images built there. Ask the owner before any test on it (its disk
is nearly full).

## Long-running agents
Hermes and OpenClaw as lasting VMs on the always-on host. Not designed: the
owner still starts them by hand ("factory"). `machines probe` reads their
gateway ports from the environment until then (no flag).

## Network isolation
Tart Softnet for VMs that need no LAN (ENGINE.md risk 3; round 5: no
tailscaled inside a VM any more, so Softnet only has to carry the engine's
ssh). Until then VMs run on Tart's shared NAT: a VM reaches host
services bound on 0.0.0.0 AND the other VMs of its host (sshd, published
Docker ports), whatever the tailnet ACL says.

## Smaller follow-ups
- An image registry (private ghcr) once two hosts build the same image.
- Memory soak: a Linux VM 8 h under load, host RSS of the VM process (ENGINE.md risk 2).
- `machines keep <vm>`: turn a temporary VM lasting (rewrite its record; the engine reads it).
- Delete the `machines supervise` retirement shim one release after v3.0.0 (every old per-VM agent is migrated by then).
- A `watch` op on the engine's socket, for the menu-bar app (round 5, N3: "a menu-bar app later is another client").
- HTTPS mode: if phase B finds the app's `tailscale serve` refusing arbitrary https ports, the default of `catalog.ServicePolicy.HTTPS` becomes `path` (C350).
- Remove the old Tailscale leftovers by hand when wanted: the `tag:station-vm` tag owner, the VM-join OAuth client, the userspace node of each host (admin console), `brew uninstall tailscale` (the formula; the app replaces it).
- The poweroff intent over a FIFO instead of a console file, if the macOS guest's console differs.
- A Lima adapter for Linux guests if Tart's licence or owner turns bad.
- Herdr: `machines create` could run `hc machine add <name> [--temp]`.
- `station status --only <section>` if `studio doctor` finds 20 s too slow.
- The TUI's Here tabs re-scoped to a remote machine in place (today `enter` on a Fleet row opens that machine's own TUI over ssh).
