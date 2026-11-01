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

`create` asks the lifetime on a terminal, then the host when two or more are reachable. Defaults are 4 CPUs, 8 GB, 50 GB and image `linux`; memory is capped at 75 % of the host's. It reserves the ports of every service [profile](services.md#profiles) before the VM first runs, and a step that fails rolls everything back. `--network bridged` is for macOS VMs only (an iPhone on the Wi-Fi reaching the VM); a Linux VM publishes its [services](services.md) instead. A temporary VM cannot be restarted from the host (`Hint: ssh <vm> sudo reboot`).

`create`, `start` and `restart` need the [daemon](#the-daemon): with it down they refuse before any change (`Error: the station daemon is not running`, `Hint: station setup`). `stop` and `delete` act directly when it is down, so a broken daemon never holds a VM.

## On another machine

`--on <machine>` runs the same command there: `station status --on mac-mini`, `station tools install gh --on api`. Station runs `ssh <machine>` with the remote `station`, and the exit code is the remote's. An unreachable machine is `Error: cannot reach "<machine>"`, `Hint: station machines probe <machine>`, exit 1. It is refused on `shell`, `completion`, `help` and the hidden plumbing (`daemon`, `guest`, `machines pipe`, `machines supervise`); `man` runs there like any command. Bare `station --on api` opens that machine's TUI on a terminal. After a remote create, delete, add or forget, the local ssh blocks refresh.

## The tailnet

Each host runs the Tailscale app (the cask `tailscale-app`, `station tools install tailscale`); the owner logs in through the app. Station only reads it: `station tailnet` shows the node, its MagicDNS name and what `tailscale serve` exposes, and station drives nothing but `tailscale serve` for [published services](services.md#https). A VM has no node, no key and no tag; the owner's Tailscale policy needs no line for station.

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

One `station daemon` per host, under the launchd agent `com.jterrazz.station.daemon` that `station setup` installs once tart is there (`○ daemon  not needed until tart is installed` before). It runs every VM of the host detached (a daemon restart never stops a VM), reads how each run ended (the rules of [Lifetimes](#lifetimes)), adopts the running VMs again at its start, keeps one ssh connection per running VM to its [guest](services.md), and holds the leases. The CLI is its client over `~/.local/state/station/daemon.sock` (protocol `station.daemon/v1`, [Services](services.md#the-daemon-protocol)). With it down, `machines` and `services` still read the files and show one `◐ daemon  not running  station setup` row; `status` shows the same row in its Machine section. [Footprint](footprint.md) lists the plist, the socket and the run files. A host from the earlier rounds moves under it by the [Round 5 migration](#round-5-migration).

## Round 5 migration

`station setup` moves a host from the earlier rounds and says so, row by row:

- `layout`: the userspace tailscaled station ran is stopped (in the background when it carries the session) and its files removed; its node stays in the [admin console](https://login.tailscale.com/admin/machines) until the owner removes it. The three tailnet keychain accounts are deleted, and the row points at the [OAuth client](https://login.tailscale.com/admin/settings/oauth) to revoke. `settings.json` loses its `tailnet` key.
- `daemon`: every per-VM agent `com.jterrazz.station.vm.<name>` is booted out and its plist removed, a lasting VM keeping its stopped or running wish; with any of those VMs running the step refuses (`Hint: stop the lasting VMs and delete the temporary ones (station machines), then station setup`), never cutting a VM.

Left to the owner, by hand and whenever wanted: the old VM tag and its ACL lines, the OAuth client, each host's old userspace node in the admin console, and the Homebrew formula (`brew uninstall tailscale`: `status` shows `◐ tailscale  formula installed, app absent` until the app is installed).

## Gateways

On a host, `station status` and `machines probe` report the OpenClaw and Hermes gateways when detected, by their process or their config: the runtime, the config and the channels for OpenClaw, the runtime and the config for Hermes (`status.gateways` in [the contract](status-json.md)). Their ports come from `OPENCLAW_GATEWAY_PORT`, `HERMES_GATEWAY_PORT` and `API_SERVER_PORT` ([Configuration](configuration.md)). Designing them as VMs is on the [TODO](../../TODO.md).

## Related

- [Services](services.md): what a VM listens on, and its URL.
- [Images](images.md): what a VM is cloned from.
- [Secrets](secrets.md): what reaches a VM, and what delete revokes.
- [Commands](commands.md): every flag.
