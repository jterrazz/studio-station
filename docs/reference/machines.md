# Machines

The fleet is every machine the owner runs: the physical Macs (hosts) and the VMs on each. One list shows them, any host can drive any other, and a VM is made, run and deleted from the host it lives on.

| Section           | Answers                                                      |
| ----------------- | ------------------------------------------------------------ |
| Hosts and VMs     | What a machine is, and how it is named and reached           |
| Lifetimes         | When a VM ends: poweroff, reboot, host reboot, `delete`      |
| The commands      | Create, start, stop, restart, delete, probe, unlock          |
| `--on`            | Running a command on another machine                         |
| The tailnet       | The Tailscale app on each host, read only                    |
| SSH and Herdr     | How one name reaches a host, a VM here, a VM of another host |
| The daemon        | The one process per host that runs the VMs                   |
| Round 5 migration | What setup retires, and what the owner removes by hand       |
| Gateways          | The OpenClaw and Hermes gateways a host reports              |

## Hosts and VMs

A host registers itself with `station setup --name <name>`; another host is registered with `station machines add --name <name> [--user <login>] [--key <file>]` and forgotten with `forget`, neither touching it. A VM is cloned from an [image](images.md) on a chosen host. The kind of a machine is `host`, `linux` or `macos`, detected at run time (a Linux binary is `linux`; a macOS VM reports `kern.hv_vmm_present`). A VM refuses the commands that only hosts do, with `Hint: run it on a host`.

A machine has one name, used for `settings.json`, the `~/.ssh/config` Host, Herdr and `--on`; a host's name is also its tailnet node. A VM is never on the tailnet: it is reached through its host. Every VM has the fleet's one login, `vm_user`; a host's login is its own `--user`.

`station machines` lists the fleet, one column schema for every row, a host unreachable in 5 seconds being a `◐` row and not a failure. A VM's ip is its NAT address on its host:

```text
 ●  macbook   host                online   100.101.1.2   2 VMs
 ●  api       linux  macbook  lasting    running  192.168.64.9   4 cpus · 12 GB · 18 GB
 ●  linux-1   linux  macbook  temporary  running  192.168.64.7   4 cpus · 8 GB · 3 GB
 ◐  mac-mini  host                unreachable
```

## Lifetimes

Whether a VM is meant to last is asked at every create ([Glossary](../concepts/glossary.md)). The two lifetimes end differently:

| Event                          | Temporary VM                                          | Lasting VM                                 |
| ------------------------------ | ----------------------------------------------------- | ------------------------------------------ |
| `poweroff` inside the VM       | deleted: disk, leases, ssh blocks, record             | stays stopped until `start`                |
| `reboot` inside the VM         | comes back; never deleted                             | comes back                                 |
| host reboot or power cut       | stale (`✗`), reclaimed by `station machines delete`   | comes back up with the host's daemon       |
| a crash of its run             | stale (`✗`), kept                                     | runs again after 10 s                      |
| `station machines delete <vm>` | deleted                                               | deleted                                    |
| `station machines stop <vm>`   | refused: it would delete it (`Hint: machines delete`) | powers off and stays stopped until `start` |

The VM tells its host when it powers off: a unit in the image writes `station-intent=poweroff` to its console, and [the daemon](#the-daemon) reads it when the VM's run ends. The host cannot otherwise tell a reboot from a poweroff, so any stop it cannot read as a poweroff never deletes a temporary VM. A VM that stops again at once, four times in a minute, is reported as broken and kept.

## The commands

```sh
station machines create --lifetime lasting --name api --memory 12G --disk 80G
station machines start api          # a stopped lasting VM
station machines stop api           # a lasting VM
station machines restart mac-mini   # a host: FileVault-aware, waits for ssh; a lasting VM: power off, run
station machines delete api         # destroys the VM, its leases and the secrets lent to it
station machines probe api          # reachability without logging in
station machines unlock mac-mini    # the FileVault pre-boot ssh session
```

`create` asks the lifetime on a terminal, then the host when two or more are reachable. Defaults are 4 CPUs, 8 GB, 50 GB and image `linux`; memory is capped at 75 % of the host's. It reserves the ports of every service [profile](services.md#profiles) before the VM first runs, and a step that fails rolls everything back. `--network bridged` is for macOS VMs only (an iPhone on the Wi-Fi reaching the VM); a Linux VM publishes its [services](services.md) instead. With Serve off on the tailnet, the profiles' variables hold the local URL and `create` adds a `◐ services` row naming the admin step. A temporary VM cannot be restarted from the host (`Hint: ssh <vm> sudo reboot`).

`create`, `start` and `restart` need the [daemon](#the-daemon): with it down they refuse before any change (`Error: the station daemon is stopped on <host>`, `Hint: station start`; without the agent's plist, `is not installed on <host>` and `station setup`). `stop` and `delete` act directly when it is down, so a broken daemon never holds a VM.

## On another machine

`--on <machine>` runs the same command there: `station status --on mac-mini`, `station tools install gh --on api`. Station runs `ssh <machine>` with the remote `station`, and the exit code is the remote's. An unreachable machine is `Error: cannot reach "<machine>"`, `Hint: station machines probe <machine>`, exit 1. It is refused on `shell`, `completion`, `help` and the hidden plumbing (`daemon`, `guest`, `machines pipe`, `machines supervise`); `man` runs there like any command. Bare `station --on api` opens that machine's TUI on a terminal. After a remote create, delete, add or forget, the local ssh blocks refresh.

## The tailnet

Each host runs the Tailscale app; the owner logs in through the app. Station only reads it: `station tailnet` shows the node, its MagicDNS name, the CLI it ran and what `tailscale serve` exposes, and station drives nothing but `tailscale serve` for [published services](services.md#serve-and-https) and `tailscale cert`, which [warms the certificate](services.md#serve-and-https). A VM has no node, no key and no tag; the owner's Tailscale policy needs no line for station.

The app comes in two variants, and station takes either:

| Variant      | Bundle id                 | How it got there                                                                                                             | Its CLI                                                            |
| ------------ | ------------------------- | ---------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| `standalone` | `io.tailscale.ipn.macsys` | the cask `tailscale-app`, `station tools install tailscale`                                                                  | `/usr/local/bin/tailscale`, which the cask's pkg installs          |
| `app-store`  | `io.tailscale.ipn.macos`  | the App Store (`Contents/_MASReceipt`); `tools` lists it `ok`, unmanaged, `app-store`, never upgraded nor removed by station | `/Applications/Tailscale.app/Contents/MacOS/Tailscale`, on no PATH |

Every `tailscale` call, the daemon's included, runs the CLI resolved at that call by absolute path: `/usr/local/bin/tailscale`, then the App Store bundle's executable, then `tailscale` on PATH (launchd's PATH holds neither `/opt/homebrew/bin` nor `~/.local/bin`); none is the tailnet `absent`. Every call sets `SHLVL`: the App Store bundle's executable answers as a CLI only when `TERM`, `SHLVL` or `TERM_PROGRAM` is set, and launchd sets none of them. A `Warning: client version ...` line, a CLI older or newer than its daemon, is dropped before the answer is read. `station tailnet`, `station status`'s tailnet row and `status --json`'s `tailnet` say the `variant` and the `cli` path used.

With the App Store app, `station setup`'s `tools` step makes `tailscale` answer in a terminal, an IDE and `studio doctor` by writing `~/.local/bin/tailscale`, two lines, `#!/bin/sh` and `exec /Applications/Tailscale.app/Contents/MacOS/Tailscale "$@"`: a symlink to that executable crashes (`bundleIdentifier is unknown to the registry`), so a symlink found there is replaced, the script's exact bytes are adopted, and a file station did not write, or another `tailscale` on PATH, is left alone. Until it is written `status` shows `◐ setup  tools`. The same step uninstalls the Homebrew formula `tailscale` (an older CLI the app supersedes) once the app is installed: `● tools  tailscale formula uninstalled: the app replaces it`; the formula alone stays, `◐ tailscale  formula installed, app absent`.

## SSH and Herdr

Station writes one managed block per machine of the fleet in `~/.ssh/config` on every host, refreshed by a bare `machines` and after every change, so `ssh api` works anywhere in the fleet. Every block pins its host key in `~/.local/state/station/known_hosts/<name>` (`StrictHostKeyChecking accept-new`). The three shapes:

```text
Host mac-mini                                  # a host: its sshd, reached by its MagicDNS name
  HostName mac-mini
  User jterrazz.agent
  IdentityFile ~/.ssh/id_ed25519

Host linux-1                                   # a VM of this host: no hop
  ProxyCommand /Users/jterrazz/.local/bin/station machines pipe linux-1

Host api                                       # a VM of mac-mini, seen from macbook: one hop
  ProxyCommand ssh mac-mini -- sh -c 'PATH="$HOME/.local/bin:/usr/local/bin:$PATH" exec station "$@"' station machines pipe api
```

`station machines pipe <vm>` (hidden plumbing) carries ssh's bytes to the VM's current NAT ip on its own host; it never needs the daemon. A VM trusts its host's key and every line of the host's own `~/.ssh/authorized_keys` through a managed block of its `~/.ssh/authorized_keys` (`# >>> station hosts >>>`), written by the guest, so whoever may log into a host may reach its VMs. Herdr drives that same ssh: `hc machine add api` (add `--temp` for a temporary VM). There is no `ssh` verb in station: a wrapper would be a second way to do one thing.

## Files and editors

Nothing of a host is ever shared into a VM: `tart run` gets no `--dir`, and no flag or setting adds one, because a mounted host folder would be a hole in the isolation. The ssh name is the one door, for files and editors alike:

```sh
scp report.zip api:                 # one file in
rsync -a project/ api:project/      # a folder, again and again
scp api:out.log .                   # and back
```

An editor on a host opens the VM by its Remote SSH to the same name: Zed, VS Code or Cursor on `ssh://api/~/code`. So there is no `cp` verb and no shared exchange folder.

## The daemon

One `station daemon` per host, under the launchd agent `com.jterrazz.station.daemon` that `station setup` installs once tart is there (`○ daemon  not needed until tart is installed` before). It runs every VM of the host detached (a daemon restart never stops a VM), reads how each run ended (the rules of [Lifetimes](#lifetimes)), adopts the running VMs again at its start (its socket appears only once that and the serve reconciliation are done), keeps one ssh connection per running VM to its [guest](services.md), and holds the leases. `machines create` returns once that guest is connected and has written the VM's keys block and `services.env` (60 s, else it rolls back). `machines start` and `restart` return once the new run's guest is connected too: a stop ends the guest's session with its run. That connection shares the VM's host key pin with `ssh`, whichever key type was pinned first. Why a guest does not connect is written to `~/.local/state/station/logs/daemon.log`, one timestamped line per new reason. Its `tailscale` calls die with it: each leads its own process group, killed whole when the call ends or times out, and a call a killed daemon left running is killed at the next daemon start, from its record under `~/.local/state/station/children/`. The CLI is its client over `~/.local/state/station/daemon.sock` (protocol `station.api/v2`, [Services](services.md#the-daemon-protocol)). With it down, `machines` and `services` still read the files and show one daemon row, and `status` the same row in its Machine section: `◐ daemon  not installed  → station setup` when the agent's plist is absent, `◐ daemon  stopped  → station start` when the plist is there but the job does not answer (another host's stopped daemon reads `→ ssh <host> station start`).

`station stop [--yes]` stops everything station runs on this host: every temporary VM is deleted and every running lasting VM stopped with its wish set to stopped (its disk kept), all at once through the daemon's `engine.stop-all`, then a [Power](commands.md#station-power) session ends (`● power  ended · the clamshell reset to normal`) and the agent is booted out (`launchctl bootout gui/<uid>/com.jterrazz.station.daemon`) and the command waits for the socket to stop answering. With a VM running it asks first on a terminal (the Confirm lists what each VM becomes) and refuses piped without `--yes` (`Error: station stop deletes linux-1 and stops api`, exit 2). A VM that does not stop keeps the daemon running (`✗ api did not stop`, exit 1): a VM never runs unsupervised. With the daemon already down, the VMs whose run outlived it are ended by the daemon-less stop and delete, the clamshell a dead daemon may have left disabled is set back to normal by the CLI itself (`● power  reset`), then a loaded agent is booted out; nothing loaded is `● daemon already stopped`. `station start` bootstraps the agent again (`kickstart -k` when it is loaded but silent), waits 15 s for the daemon to answer, and lists the lasting VMs it brought back; a running daemon is `● daemon running · pid <n>`, nothing done. A login bootstraps it too, since the plist stays with `RunAtLoad`. Both act on this host only: `--on` and a VM refuse them. [Footprint](footprint.md) lists the plist, the socket and the run files. A host from the earlier rounds moves under it by the [Round 5 migration](#round-5-migration).

## Round 5 migration

`station setup` moves a host from the earlier rounds and says so, row by row:

- `layout`: the userspace tailscaled station ran is stopped (in the background when it carries the session) and its files removed; its node stays in the [admin console](https://login.tailscale.com/admin/machines) until the owner removes it. The three tailnet keychain accounts are deleted, and the row points at the [OAuth client](https://login.tailscale.com/admin/settings/oauth) to revoke. Every store is brought to its version: `settings.json` loses any key no group names (the `tailnet` key among them); a record setup cannot read is left as it is, for the command that reads it to name, and a record a newer station wrote fails the step.
- `daemon`: every per-VM agent `com.jterrazz.station.vm.<name>` is booted out and its plist removed, a lasting VM keeping its stopped or running wish; with any of those VMs running the step refuses (`Hint: stop the lasting VMs and delete the temporary ones (station machines), then station setup`), never cutting a VM.

Left to the owner, by hand and whenever wanted: the old VM tag and its ACL lines, the OAuth client and each host's old userspace node in the admin console. The Homebrew formula `tailscale` is `setup`'s to uninstall once the app is there ([The tailnet](#the-tailnet)).

## Gateways

On a host, `machines probe` reports the OpenClaw and Hermes gateways when detected, by their process or their config. `station status` reports, on a host whose tailnet is connected, `Tailscale Serve`, the way every VM service leaves the host: `on`, or `◐ off` with the admin step ([Services](services.md#serve-and-https)) (`status.gateways` in [the contract](status-json.md)). The gateways' ports come from `OPENCLAW_GATEWAY_PORT`, `HERMES_GATEWAY_PORT` and `API_SERVER_PORT` ([Configuration](configuration.md)). Designing them as VMs is on the [TODO](../../TODO.md).

## Related

- [Services](services.md): what a VM listens on, and its URL.
- [Images](images.md): what a VM is cloned from.
- [Secrets](secrets.md): what reaches a VM, and what delete revokes.
- [Commands](commands.md): every flag.
