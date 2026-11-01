# Machines

The fleet is every machine the owner runs: the physical Macs (hosts) and the VMs on each. One list shows them, any host can drive any other, and a VM is made, run and deleted from the host it lives on.

| Section        | Answers                                                 |
| -------------- | ------------------------------------------------------- |
| Hosts and VMs  | What a machine is, and how it is named and reached      |
| Lifetimes      | When a VM ends: poweroff, reboot, host reboot, `delete` |
| The commands   | Create, start, stop, restart, delete, probe, unlock     |
| `--on`         | Running a command on another machine                    |
| The tailnet    | The host's own auth, the VM join and its ACL            |
| SSH and Herdr  | How one name reaches a machine three ways               |
| Launchd agents | What keeps a VM running                                 |
| Services       | The OpenClaw and Hermes gateways a host reports         |

## Hosts and VMs

A host registers itself with `station setup --name <name>`; another host is registered with `station machines add --name <name> [--user <login>] [--key <file>]` and forgotten with `forget`, neither touching it. A VM is cloned from an [image](images.md) on a chosen host. The kind of a machine is `host`, `linux` or `macos`, detected at run time (a Linux binary is `linux`; a macOS VM reports `kern.hv_vmm_present`). A VM refuses the commands that only hosts do, with `Hint: run it on a host`.

A machine has one name, used for `settings.json`, the tailnet node, the `~/.ssh/config` Host, Herdr and `--on`. Every VM has the fleet's one login, `vm_user`; a host's login is its own `--user`.

`station machines` lists the fleet, one column schema for every row, a host unreachable in 5 seconds being a `◐` row and not a failure:

```text
 ●  macbook   host                online   100.101.1.2   2 VMs
 ●  api       linux  macbook  lasting    running  100.101.1.9   4 cpus · 12 GB · 18 GB
 ●  linux-1   linux  macbook  temporary  running  100.101.1.7   4 cpus · 8 GB · 3 GB
 ◐  mac-mini  host                unreachable
```

## Lifetimes

Whether a VM is meant to last is asked at every create ([Glossary](../concepts/glossary.md)). The two lifetimes end differently:

| Event                          | Temporary VM                                          | Lasting VM                                 |
| ------------------------------ | ----------------------------------------------------- | ------------------------------------------ |
| `poweroff` inside the VM       | deleted: disk, node, ssh blocks, record               | stays stopped                              |
| `reboot` inside the VM         | comes back; never deleted                             | comes back                                 |
| host reboot or power cut       | stale (`✗`), reclaimed by `station machines delete`   | comes back up with the host                |
| `station machines delete <vm>` | deleted                                               | deleted                                    |
| `station machines stop <vm>`   | refused: it would delete it (`Hint: machines delete`) | powers off and stays stopped until `start` |

The VM tells its host when it powers off: a unit in the image writes `station-intent=poweroff` to its console, and the VM's supervisor, the hidden `station machines supervise <vm>` its launchd agent runs, reads it. The host cannot otherwise tell a reboot from a poweroff, so any stop it cannot read as a poweroff never deletes a temporary VM. A VM that stops again at once, three times in a minute, is reported as broken and kept.

## The commands

```sh
station machines create --lifetime lasting --name api --memory 12G --disk 80G
station machines start api          # a stopped lasting VM
station machines stop api           # a lasting VM
station machines restart mac-mini   # a host: FileVault-aware, waits for ssh; a lasting VM: power off, run
station machines delete api         # destroys the VM, its node and the secrets lent to it
station machines probe api          # reachability without logging in
station machines unlock mac-mini    # the FileVault pre-boot ssh session
```

`create` asks the lifetime on a terminal, then the host when two or more are reachable. Defaults are 4 CPUs, 8 GB, 50 GB and image `linux`; memory is capped at 75 % of the host's. A step that fails after the node joined rolls everything back. A temporary VM cannot be restarted from the host (`Hint: ssh <vm> sudo reboot`).

## On another machine

`--on <machine>` runs the same command there: `station status --on mac-mini`, `station tools install gh --on api`. Station runs `ssh <machine>` with the remote `station`, and the exit code is the remote's. An unreachable machine is `Error: cannot reach "<machine>"`, `Hint: station machines probe <machine>`, exit 1. It is refused on `shell`, `completion`, `help` and `supervise`. Bare `station --on api` opens that machine's TUI on a terminal. After a remote create, delete, add or forget, the local ssh blocks refresh.

## The tailnet

Every machine is a node. A host runs a userspace tailscaled and authenticates with its own login or an auth key (`station config install tailnet-auth`, asked by `station tailnet up`). A VM joins with a single-use auth key the host mints at create time; the key touches no disk and no argv. A VM-join client lets the host do that and remove the node on delete.

### VM join

The owner does this once in the Tailscale admin console; until a mint succeeds `station tailnet` shows `○ VM join  not set up`.

1. Declare the tag owner:

    ```json
    "tagOwners": { "tag:station-vm": ["autogroup:admin"] }
    ```

2. Create an OAuth client with the scopes `auth_keys` (write) and `devices:core` (write), both limited to `tag:station-vm`, and keep its id and secret with `station config install vm-join`.
3. Let members reach `tag:station-vm`, and let `tag:station-vm` reach nothing on the tailnet.
4. Allow ssh, never `check` mode, because `check` re-authenticates in a browser that Herdr's batch ssh cannot open:

    ```json
    "ssh": [{ "action": "accept", "src": ["autogroup:member"], "dst": ["tag:station-vm"], "users": ["autogroup:nonroot", "root"] }]
    ```

The ACL limits the tailnet only. On Tart's shared NAT a VM reaches the other VMs of its host and every service the host binds on `0.0.0.0`, whatever the ACL says; closing that is on the [TODO](../../TODO.md).

## SSH and Herdr

Hosts are reached through the Mac's own sshd and a key (the `sshd` and `ssh-key` items, `machines add --key`); VMs through Tailscale SSH, with no key. Station writes one managed block per machine of the fleet in `~/.ssh/config` on every host, refreshed by a bare `machines` and after every change, so `ssh api` works anywhere in the fleet. Herdr drives that same ssh: `hc machine add api` (add `--temp` for a temporary VM). There is no `ssh` verb in station: a wrapper would be a second way to do one thing.

## Launchd agents

Each VM has a launchd agent, `com.jterrazz.station.vm.<name>`, that runs its supervisor. A lasting VM's agent runs at load and restarts after a crash; a temporary VM's does not run at load, which is why it is stale after a host reboot. [Footprint](footprint.md) lists the plist.

## Services

On a host, `station status` and `machines probe` report the OpenClaw and Hermes gateways when detected, by their process or their config: the runtime, the config and the channels for OpenClaw, the runtime and the config for Hermes. Their ports come from `OPENCLAW_GATEWAY_PORT`, `HERMES_GATEWAY_PORT` and `API_SERVER_PORT` ([Configuration](configuration.md)). Designing them as VMs is on the [TODO](../../TODO.md).

## Related

- [Images](images.md): what a VM is cloned from.
- [Secrets](secrets.md): what reaches a VM, and what delete revokes.
- [Commands](commands.md): every flag.
