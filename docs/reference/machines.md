# Machines

The fleet is every machine the owner runs: the physical Macs (hosts) and the VMs on each. One list shows them, any host can drive any other, and a VM is made, run and deleted from the host it lives on.

| Section           | Answers                                                      |
| ----------------- | ------------------------------------------------------------ |
| Hosts and VMs     | What a machine is, and how it is named and reached           |
| Lifetimes         | When a VM ends: poweroff, reboot, host reboot, `delete`      |
| The commands      | Create, start, stop, restart, delete, unlock                 |
| `--on`            | Running a command on another machine                         |
| The tailnet       | The Tailscale app on each host, read only                    |
| SSH and Herdr     | How one name reaches a host, a VM here, a VM of another host |
| The engine        | The one process per host that runs the VMs                   |
| Round 5 migration | What setup retires, and what the owner removes by hand       |
| Gateways          | The OpenClaw and Hermes gateways a host reports              |

## Hosts and VMs

A host registers itself with `station setup --name <name>`; another host is registered with `station machines add --name <name> [--user <login>] [--key <file>]` and forgotten with `forget`, neither touching it. A VM is cloned from an [image](images.md) on a chosen host. The kind of a machine is `host`, `linux` or `macos`, detected at run time (a Linux binary is `linux`; a macOS VM reports `kern.hv_vmm_present`). A VM refuses the commands that only hosts do, with `Hint: run it on a host`.

A machine has one name, used for `preferences.json`, the `~/.ssh/config` Host, Herdr and `--on`; a host's name is also its tailnet node. A VM is never on the tailnet: it is reached through its host. Every VM has the fleet's one login, `vm_user`; a host's login is its own `--user`.

`station machines` is All machines: a first line that counts the machines, the running, the stopped and what needs you; each Mac with its VMs under it, in the columns MACHINE, KIND, STATE, CPU and MEMORY; then Needs you. A Mac that does not answer in 5 seconds keeps its row, `✗ Unreachable`, and is a finding, never a failure; a Mac that runs another station says which and where to update it. Piped, each machine is one record (glyph, name, kind, its Mac, state, CPU, memory); `--json` is `station.machines.list/v1`.

```text
5 machines · 4 running · 1 stopped · nothing needs you
 MACHINE KIND STATE CPU MEMORY
 ● This Mac Mac Engine running
 ├─ ○ api Workspace Stopped · permanent 4 CPU 8 GB
 └─ ● linux-1 Workspace Running · throwaway 4 CPU 8 GB
 ● mac-mini Mac Engine running
 └─ ● infra Server Running · always on 8 CPU 24 GB
```

`station machines <machine>` is that machine's Overview, the one `station overview --on <machine>` prints.

## Lifetimes

A workspace is throwaway unless it is made `--permanent` ([Glossary](../concepts/glossary.md)); a server is always on (its record's lifetime is `always-on`, the one mark of its kind). No workspace restarts by itself (SP-4), and a stop never destroys a VM (SP-5). A server whose wanted state is Running starts with its Mac's engine; a stop sets its wanted state to Stopped, so it stays stopped across its Mac's reboots until `start` sets it back to Running. A Mac's servers are listed before its workspaces:

| Event                          | Throwaway workspace                                   | Permanent workspace                             |
| ------------------------------ | ----------------------------------------------------- | ----------------------------------------------- |
| `poweroff` inside the VM       | deleted: disk, leases, ssh blocks, record             | stays stopped until `start`                     |
| `reboot` inside the VM         | comes back; never deleted                             | comes back                                      |
| host reboot or power cut       | stopped, `ended with its Mac`                         | stopped, `ended with its Mac`                   |
| a crash, or an end not read    | kept, `ended unexpectedly · start or delete it`       | kept, `ended unexpectedly · start or delete it` |
| `station machines stop <vm>`   | stopped, kept with its disk until `start` or `delete` | stopped, kept with its disk until `start`       |
| `station machines delete <vm>` | deleted                                               | deleted                                         |

The VM tells its host when it powers off: a unit in the image writes `station-intent=poweroff` to its console, and [the engine](#the-engine) reads it when the VM's run ends. The host cannot otherwise tell a reboot from a poweroff, so any end it cannot read as a poweroff never deletes a throwaway workspace.

## The commands

```sh
station machines create api --permanent --memory 12G --disk 80G
station machines start api # a stopped workspace, its disk as it was left
station machines stop api # asks yes or no; kept with its disk
station machines stop infra --on mac-mini # a server: asks its name typed (--confirm=infra), --yes answers nothing
station machines edit api --memory 16G # applies when api next starts; a disk only grows
station machines ssh api # prints the ssh command: ssh admin@api
station machines restart mac-mini # a Mac: FileVault-aware, waits for ssh; a VM: power off, run
station machines delete api # destroys the VM, its leases and the secrets lent to it
station machines api # its Overview, as station overview --on api
station machines unlock mac-mini # the FileVault pre-boot ssh session
```

`create` makes a throwaway workspace unless `--permanent` is given, and asks the host when two or more are reachable. Defaults are 4 CPUs, 4 GB, 20 GB and image `linux`, each size with its unit. A size above what the Mac can give is refused before any effect (`not enough memory on This Mac: 60 GB asked, 4 GB left for VMs`): macOS keeps 4 GB, and the running VMs hold theirs. It reserves the ports of every service [profile](services.md#profiles) before the VM first runs; a step that fails before that rolls everything back. It then prints the health steps as they pass (Running, Has an address, SSH answers), its ports, and `linux-2 Created` with its size. From the first run on the VM is kept whatever fails: `linux-2 did not boot: …`, or `linux-2 is kept, failing Has an address: none after 60 s`, each with start or delete as the fix. A running workspace whose guest connection fails is a finding, `fails SSH answers`. A Mac that does not answer keeps its row in `station machines`, `Unreachable · last answer 03:12`, and the VMs it held at that answer as `? Unknown`; `station machines <vm>` of such a VM reads `infra is on mac-mini, which does not answer`, exit 0. `--network bridged` is for macOS VMs only (an iPhone on the Wi-Fi reaching the VM); a Linux VM publishes its [services](services.md) instead. With Tailscale HTTPS off, the profiles' variables hold the local URL and `create` adds a `◐ ports` row naming the admin step. `delete` asks the name typed (`--confirm=<name>` in a script, never `--yes`). With the engine stopped, start, stop, restart, edit and delete are refused (`Hint: station engine start`).

`create`, `start`, `stop`, `restart`, `edit` and `delete` need the [engine](#the-engine): with it down they refuse before any change and the VM is kept (`Error: the engine is stopped on This Mac`, `Hint: station engine start`; without the agent's plist, `is not installed on This Mac` and `station setup`).

## On another machine

`--on <machine>` runs the command's call there and renders its answer here: `station status --on mac-mini`, `station apps install gh --on api`. Another Mac is reached through its engine: one `ssh -o BatchMode=yes <mac> station engine attach`, which speaks `station.api/v2` frames over ssh's stdio; a VM of this Mac through this Mac's engine, which runs `station guest serve --stdio` over its one connection to the VM. A VM of another Mac rides that Mac's engine: its name is looked up in this Mac's own records first, then asked of every other host at once (`machines.list`, 4 at a time, 5 s each); station never runs `ssh <vm>`. A name no host holds is `Error: no machine named "<name>"`, `Hint: station machines`, exit 2, with nothing dialed under that name; while a host is silent the VM may be its own, so the answer is that host's `does not answer`. The far side returns data, so the output is this binary's, in tty, pipe and JSON alike. Each connection opens with `Hello`, the far binary's version and the calls it serves: a call it lacks is `Error: <mac> runs station <version>, which has no <call>`, `Hint: update station on <mac> (scripts/install.sh there)`, exit 1. A connection to a Mac serves every call while it answers, and nothing outlives it: a Mac asleep at one call, or whose link dropped, is dialed again at the next, and a call the open connection's `Hello` lacks is asked again of a fresh one, so a Mac upgraded meanwhile serves it. A far refusal is worded by this binary's table; a code this binary does not know keeps the far side's own words. An unreachable machine is `Error: <machine> does not answer`, `Hint: station machines`, exit 1. It is refused on `shell`, `completion`, `help`, `man` and the hidden plumbing (`engine`, `engine`, `guest`, `machines pipe`, `machines supervise`). Bare `station --on api` opens the TUI here, its calls on that machine. After a remote create, delete, add or forget, the local ssh blocks refresh.

The fleet list asks each other host's engine for its own view (`machines.list`), 4 at a time, 5 s each; a host that does not answer is a row of its own, never the others' delay. On a terminal the rows fill as each host answers; piped and in JSON the list prints once, whole.

## The tailnet

Each host runs the Tailscale app; the owner logs in through the app. Station only reads it: the first line of `station ports` shows the node, its MagicDNS name, the CLI it ran and what `tailscale serve` exposes, and station drives nothing but `tailscale serve` for [published services](services.md#serve-and-https) and `tailscale cert`, which [warms the certificate](services.md#serve-and-https). A VM has no node, no key and no tag; the owner's Tailscale policy needs no line for station.

The app comes in two variants, and station takes either:

| Variant      | Bundle id                 | How it got there                                                                                                             | Its CLI                                                            |
| ------------ | ------------------------- | ---------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| `standalone` | `io.tailscale.ipn.macsys` | the cask `tailscale-app`, `station apps install tailscale`                                                                   | `/usr/local/bin/tailscale`, which the cask's pkg installs          |
| `app-store`  | `io.tailscale.ipn.macos`  | the App Store (`Contents/_MASReceipt`); `tools` lists it `ok`, unmanaged, `app-store`, never upgraded nor removed by station | `/Applications/Tailscale.app/Contents/MacOS/Tailscale`, on no PATH |

Every `tailscale` call, the engine's included, runs the CLI resolved at that call by absolute path: `/usr/local/bin/tailscale`, then the App Store bundle's executable, then `tailscale` on PATH (launchd's PATH holds neither `/opt/homebrew/bin` nor `~/.local/bin`); none is the tailnet `absent`. Every call sets `SHLVL`: the App Store bundle's executable answers as a CLI only when `TERM`, `SHLVL` or `TERM_PROGRAM` is set, and launchd sets none of them. A `Warning: client version ...` line, a CLI older or newer than its engine, is dropped before the answer is read. `station tailnet`, `station status`'s tailnet row and `status --json`'s `tailnet` say the `variant` and the `cli` path used.

With the App Store app, `station setup`'s `tools` step makes `tailscale` answer in a terminal, an IDE and `studio doctor` by writing `~/.local/bin/tailscale`, two lines, `#!/bin/sh` and `exec /Applications/Tailscale.app/Contents/MacOS/Tailscale "$@"`: a symlink to that executable crashes (`bundleIdentifier is unknown to the registry`), so a symlink found there is replaced, the script's exact bytes are adopted, and a file station did not write, or another `tailscale` on PATH, is left alone. Until it is written `status` shows `◐ setup tools`. The same step uninstalls the Homebrew formula `tailscale` (an older CLI the app supersedes) once the app is installed: `● tools tailscale formula uninstalled: the app replaces it`; the formula alone stays, `◐ tailscale formula installed, app absent`.

## SSH and Herdr

Station writes one managed block per machine of the fleet in `~/.ssh/config` on every host, refreshed by a bare `machines` and after every change, so `ssh api` works anywhere in the fleet. Every block pins its host key in `~/.local/state/station/known_hosts/<name>` (`StrictHostKeyChecking accept-new`). The three shapes:

```text
Host mac-mini # a host: its sshd, reached by its MagicDNS name
 HostName mac-mini
 User jterrazz.agent
 IdentityFile ~/.ssh/id_ed25519

Host linux-1 # a VM of this host: no hop
 ProxyCommand /Users/jterrazz/.local/bin/station machines pipe linux-1

Host api # a VM of mac-mini, seen from macbook: one hop
 ProxyCommand ssh mac-mini -- sh -c 'PATH="$HOME/.local/bin:/usr/local/bin:$PATH" exec station "$@"' station machines pipe api
```

`station machines pipe <vm>` (hidden plumbing) carries ssh's bytes to the VM's current NAT ip on its own host; it never needs the engine. A VM trusts its host's key and every line of the host's own `~/.ssh/authorized_keys` through a managed block of its `~/.ssh/authorized_keys` (`# >>> station hosts >>>`), written by the guest, so whoever may log into a host may reach its VMs. Herdr drives that same ssh: `hc machine add api` (add `--temp` for a throwaway workspace). `station machines ssh <machine>` prints the command, never runs it, so `$(station machines ssh api)` does.

## Files and editors

Nothing of a host is ever shared into a VM: `tart run` gets no `--dir`, and no flag or setting adds one, because a mounted host folder would be a hole in the isolation. The ssh name is the one door, for files and editors alike:

```sh
scp report.zip api: # one file in
rsync -a project/ api:project/ # a folder, again and again
scp api:out.log . # and back
```

An editor on a host opens the VM by its Remote SSH to the same name: Zed, VS Code or Cursor on `ssh://api/~/code`. So there is no `cp` verb and no shared exchange folder.

## The engine

One engine per host, under the launchd agent `com.jterrazz.station.engine` that `station setup` installs once tart is there (`○ engine not needed until tart is installed` before). It runs every VM of the host detached (an engine restart never stops a VM), reads how each run ended (the rules of [Lifetimes](#lifetimes)), adopts the running VMs again at its start (its socket appears only once that and the serve reconciliation are done), keeps one ssh connection per running VM to its [guest](services.md), and holds the leases. `machines create` returns once that guest is connected and has written the VM's keys block and `services.env` (60 s, else the VM is kept, failing SSH answers). `machines start` and `restart` return once the new run's guest is connected too: a stop ends the guest's session with its run. That connection shares the VM's host key pin with `ssh`, whichever key type was pinned first. Why a guest does not connect is written to `~/.local/state/station/logs/engine.log`, one timestamped line per new reason. Its `tailscale` calls die with it: each leads its own process group, killed whole when the call ends or times out, and a call a killed engine left running is killed at the next engine start, from its record under `~/.local/state/station/children/`. The CLI is its client over `~/.local/state/station/engine.sock` (protocol `station.api/v2`, [Services](services.md#the-engine-protocol)). With it down, `machines` and `services` still read the files and show one engine row, and `status` the same row in its Machine section: `◐ engine not installed → station setup` when the agent's plist is absent, `◐ engine stopped → station engine start` when the plist is there but the job does not answer (another host's stopped engine is read and started through it, `station engine start --on <host>`).

`station engine stop [--yes]` is Stop everything on this Mac: every running VM stops with its disk kept, a throwaway one too (only `machines delete` destroys), all at once through the engine's `engine.stop-all`; the Awake session ends and the sharing of their ports with it; then the agent is booted out (`launchctl bootout gui/<uid>/com.jterrazz.station.engine`) and the stop is kept with who asked it, so `station engine` reads `stopped · since 13:40 by the menu bar`. A VM that will not stop keeps the engine running (exit 1). Off a terminal, a stop that would stop a VM needs `--yes`. `station engine start` bootstraps the agent again; the VMs it stopped stay stopped, and a login starts the engine again whatever stopped it.

## The v3 migration

`station setup` on a Mac station v2 set up moves, once, and says so: `~/.config/station/settings.json` to `preferences.json` (byte for byte, only while no `preferences.json` is there: `● preferences moved settings.json to preferences.json`), and the engine's job `com.jterrazz.station.daemon` to `com.jterrazz.station.engine` (`● engine running v3.1.0, its job moved to com.jterrazz.station.engine`). A second run says nothing of either. A `preferences.json` that does not read stops every command that needs it, at its line, and is never rewritten.

## Round 5 migration

`station setup` moves a host from the earlier rounds and says so, row by row:

- `layout`: the userspace tailscaled station ran is stopped (in the background when it carries the session) and its files removed; its node stays in the [admin console](https://login.tailscale.com/admin/machines) until the owner removes it. The three tailnet keychain accounts are deleted, and the row points at the [OAuth client](https://login.tailscale.com/admin/settings/oauth) to revoke. Every store is brought to its version: `preferences.json` loses any key no group names (the `tailnet` key among them); a record setup cannot read is left as it is, for the command that reads it to name, and a record a newer station wrote fails the step.
- `engine`: every per-VM agent `com.jterrazz.station.vm.<name>` is booted out and its plist removed, a VM keeping its stopped or running wish; with any of those VMs running the step refuses (`Hint: stop the permanent VMs and delete the throwaway ones (station machines), then station setup`), never cutting a VM.

Left to the owner, by hand and whenever wanted: the old VM tag and its ACL lines, the OAuth client and each host's old userspace node in the admin console. The Homebrew formula `tailscale` is `setup`'s to uninstall once the app is there ([The tailnet](#the-tailnet)).

## Gateways

On a host, the TUI's probe reports the OpenClaw and Hermes gateways when detected, by their process or their config. `station status` reports, on a host whose tailnet is connected, `Tailscale Serve`, the way every VM service leaves the host: `on`, or `◐ off` with the admin step ([Services](services.md#serve-and-https)) (`status.gateways` in [the contract](status-json.md)). The gateways' ports come from `OPENCLAW_GATEWAY_PORT`, `HERMES_GATEWAY_PORT` and `API_SERVER_PORT` ([Configuration](configuration.md)). Designing them as VMs is on the [TODO](../../TODO.md).

## Related

- [Services](services.md): what a VM listens on, and its URL.
- [Images](images.md): what a VM is cloned from.
- [Secrets](secrets.md): what reaches a VM, and what delete revokes.
- [Commands](commands.md): every flag.
