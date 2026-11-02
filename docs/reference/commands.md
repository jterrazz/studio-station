# Commands

Every command of `station`, grouped as `station --help` groups them. The words are the [Glossary](../concepts/glossary.md)'s.

| Section    | Answers                                                |
| ---------- | ------------------------------------------------------ |
| Rules      | What holds for every command                           |
| Here       | `status`, `tools`, `config`, `storage`                 |
| The fleet  | `machines`, `services`, `images`, `secrets`, `tailnet` |
| Shortcuts  | `run git` and `run docker`                             |
| The binary | `setup`, `--version`, `help`                           |
| Plumbing   | The hidden commands the machine runs for itself        |
| Exit codes | What 0, 1, 2 and 130 mean                              |

## Rules

- A bare noun lists (`tools`, `config`, `storage`, `machines`, `services`, `images`, `secrets`, `tailnet`). A bare group with nothing to list (`station`, `run`, `run git`) prints its menu on stderr and exits 2; on a terminal bare `station` opens the TUI.
- An existing thing is a positional (`machines stop api`); a new thing is described by options (`machines create --lifetime lasting --name api`).
- The same verbs everywhere: `install`, `uninstall`, `upgrade`. There are no aliases.
- Every read takes `--json`. Every destructive action asks on a terminal or takes `--yes`.
- Every refusal exits 2 before any question; every failure is one `Error:` and one `Hint:`.
- A word under `tools`, `config` or `storage` that names one of its entries, the verb forgotten, is still refused, but its hint is the command meant: `station config github-login` hints `station config install github-login` (`tools install <tool>`, `storage clean <tool>`).
- `--on <machine>` runs any command below on another machine of the fleet, over ssh; see [Machines](machines.md#on-another-machine).
- A value a command needs from you is never an argv flag when it could be a secret: a config item asks in a form on a terminal ([Configuration](configuration.md)).

## Here

These commands act on the machine they run on, and list only what its kind allows.

### `station status`

```sh
station status [--json]
```

Everything about this machine: system, environment, workspace, identity, daemons, gateways, tools, config and setup; on a host its Machine section shows station's daemon. `--json` prints the [`station.status/v1`](status-json.md) document `studio doctor` reads. On a terminal, `status`, `tools` and `storage` draw every row at once and fill each as its probe answers ([instant reads](../concepts/design-system.md#instant-reads)); piped or with `--json` they print one complete document.

### `station tools`

```sh
station tools [--json]                          # the catalogue tools placed here, agent skills included
station tools install <tool...>
station tools uninstall <tool...> [--yes]
station tools upgrade [<tool...> | --via <manager>...]
```

A bare `upgrade` upgrades every installed catalogue tool. `--via` takes a manager (`brew`, `npm`, `bun`, `uv`, `mise`, `skills`) and cannot be combined with names. A tool whose place excludes this machine is refused with a hint naming where it lives. An agent skill is a tool: `station tools install frontend-design`. The model is [Tools and skills](tools-and-skills.md).

### `station config`

```sh
station config [--json]                         # the config items placed here
station config install <item...>
station config uninstall <item...> [--yes]
```

An item that needs values asks them in a form on a terminal and is refused elsewhere. Only reversible items uninstall. See [Configuration](configuration.md).

### `station storage`

```sh
station storage [--json]                        # per tool: caches · data · kept · installed, with sizes; nothing runs
station storage clean [<tool...>]               # empty caches with each tool's own command; bare = every row here
station storage reset <tool...> [--yes]         # delete a tool's data, back to factory; asks first
```

`clean` asks nothing: caches only. `reset` is always named, and refuses while the tool's app runs. A stale VM goes with `station machines delete`, an old image version with `station images delete`. See [Storage](storage.md).

## The fleet

### `station machines`

```sh
station machines [--here] [--json]              # every host and the VMs on each
station machines add --name <name> [--user <login>] [--key <file>]
station machines forget <host> [--yes]
station machines create [--lifetime temporary|lasting] [--image linux] [--name <name>] [--cpus <n>] [--memory <size>] [--disk <size>] [--network nat|bridged]
station machines start <vm>
station machines stop <vm>
station machines restart <machine> [--yes]
station machines delete <vm> [--yes]
station machines probe <machine> [--json]
station machines unlock <host>
```

`--here` lists this host only, with no network. `create` asks the lifetime on a terminal and, with two reachable hosts, the host; piped it needs `--lifetime`. `delete` is the only verb that destroys a machine; `forget` only unregisters a host. VMs are made and run on hosts, so these commands except `probe` are refused on a VM. `create`, `start` and `restart` need the host's daemon; `stop` and `delete` act without it. `--network bridged` is for macOS VMs only. See [Machines](machines.md).

### `station services`

```sh
station services [--here] [--json]             # what the fleet's VMs listen on, and each URL
station services url <port> [--json]           # in a VM: reserve the port's public URL now, and print it
station services release <vm>:<port> [--yes]   # on a host; in a VM: station services release <port>
```

Every host is asked over ssh; `--here` asks none, and is refused in a VM (exit 2), which lists only its own services. `url` runs inside a VM only, its piped stdout the URL alone. A profile's lease asks before `release`. See [Services](services.md).

### `station images`

```sh
station images [--json]
station images build [<image...>] [--binary <linux station binary>]
station images delete <image>[@<version>] [--yes]
```

A bare `build` builds every image that is missing or whose recipe changed. See [Images](images.md).

### `station secrets`

```sh
station secrets [--json]                        # the lend list and what was lent where, names only
station secrets allow <name...>
station secrets disallow <name...> [--yes]
station secrets lend --to <vm> [<name...>]
station secrets recall --from <vm> [<name...>]
```

See [Secrets](secrets.md).

### `station tailnet`

```sh
station tailnet [--json]                        # the Tailscale app's node, the peers, what serve exposes
```

Read only: the owner logs in through the Tailscale app, and station never drives it but for [published services](services.md#serve-and-https).

## Shortcuts

```sh
station run git <branches|chore|feat|fix|log|push|status|sync|unwip|wip> [args]
station run docker <rm|rmi|reset> [--yes]
```

| Shortcut                              | Does                                                               |
| ------------------------------------- | ------------------------------------------------------------------ |
| `run git feat "message"`              | `git add .`, then commit `feat: message` (`fix`, `chore` alike)    |
| `run git wip` / `unwip`               | commit everything as `WIP` / undo that commit, keeping the changes |
| `run git push`                        | `git push -u origin HEAD`                                          |
| `run git sync`                        | fetch with prune, then pull                                        |
| `run git status` / `log` / `branches` | git's own, `log` the last 10 one-line                              |
| `run docker rm`                       | remove every container                                             |
| `run docker rmi`                      | remove every image                                                 |
| `run docker reset`                    | `rm`, then `rmi`                                                   |

`station storage clean docker` prunes Docker's unused data.

## The binary

```sh
station setup [--name <name>]
station --version
station help [command]
```

`setup` is the one bootstrap verb, safe to run again: it registers this host in the fleet (asking the name on a terminal, taking `--name` otherwise), writes the one managed block of `~/.zshrc`, and moves `~/.studio/` to the XDG folders. `--version` prints `station version <version> (<commit>)`. See [Getting started](../start/getting-started.md) and [Dotfiles](dotfiles.md).

## Plumbing

These commands exist for the machine's own use. They are hidden from help and the owner never types them.

| Command                           | What runs it and what it does                                                                                                                                                                                                                         |
| --------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `station shell zsh`               | the `~/.zshrc` block evaluates it at every shell start: PATH, the mise activation, station's completion, and in a VM the lent Claude token and the [profiles'](services.md#profiles) exports. Output on stdout only; another shell or none is refused |
| `station completion <shell>`      | cobra's completion script, for a shell the block does not serve                                                                                                                                                                                       |
| `station daemon`                  | the launchd agent `com.jterrazz.station.daemon` runs it on a host: the VMs and their services ([Machines](machines.md#the-daemon))                                                                                                                    |
| `station guest`                   | the host's daemon runs it inside each VM over ssh: what listens there ([Services](services.md))                                                                                                                                                       |
| `station machines pipe <vm>`      | a VM's `~/.ssh/config` ProxyCommand runs it: ssh's bytes to the VM's NAT ip ([Machines](machines.md#ssh-and-herdr))                                                                                                                                   |
| `station machines supervise <vm>` | retired: an old per-VM agent that still runs it exits at once; `station setup` removes those agents                                                                                                                                                   |

`--on` is refused on `shell`, `completion`, `help`, `daemon`, `guest`, `machines pipe` and `machines supervise`; `man --on <machine>` prints that machine's page.

## Exit codes

| Code | Means                                               |
| ---- | --------------------------------------------------- |
| 0    | Done                                                |
| 1    | An operation failed, or a confirmation was declined |
| 2    | A refusal, before any question or effect            |
| 130  | Interrupted                                         |

A command over several items prints every outcome, then exits 1 with `<n> of <m> <noun> failed` if any failed.

## Related

- [Design system](../concepts/design-system.md): the forms a document takes.
- [Status contract](status-json.md): the JSON a script reads.
