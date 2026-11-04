# Commands

Every command of `station`, grouped as `station --help` groups them. The words are the [Glossary](../concepts/glossary.md)'s.

| Section        | Answers                                                |
| -------------- | ------------------------------------------------------ |
| Rules          | What holds for every command                           |
| Here           | `overview`, `tools`, `config`, `storage`               |
| The fleet      | `machines`, `services`, `images`, `secrets`, `tailnet` |
| Shortcuts      | `run git` and `run docker`                             |
| The binary     | `setup`, `--version`, `help`                           |
| Hidden aliases | The v2 words that still run, for one release           |
| Plumbing       | The hidden commands the machine runs for itself        |
| Exit codes     | What 0, 1, 2 and 130 mean                              |

`station help` groups the commands as SPEC v3 §4.1 does: MACHINES (`machines`), ON A MACHINE (`overview`, `tools`, `config`, `storage`, `services`, `tailnet`, `secrets`, `accounts`, `power`, `run`), A MAC'S IMAGES AND ENGINE (`images`, `engine`), THIS TERMINAL (shown once `use` exists) and STATION ITSELF (`setup`, `help`). The sections below keep their v2 headings until each family takes its v3 words.

## Rules

- Bare `station` opens the TUI on a terminal; piped, or with `--all` or `--json`, it prints `station overview`, exit 0.
- A bare noun lists (`tools`, `config`, `storage`, `machines`, `services`, `images`, `secrets`, `tailnet`). A bare group with nothing to list (`run`, `run git`) prints its menu on stderr and exits 2. A verb whose argument is missing is refused, its hint the command with it: `station tools install` hints `station tools install <tool>`.
- An existing thing is a positional (`machines stop api`); a new thing is described by options (`machines create --lifetime lasting --name api`).
- The same verbs everywhere: `install`, `uninstall`, `upgrade`. The only other words are the [hidden aliases](#hidden-aliases).
- Every read takes `--json`. Every destructive action asks on a terminal or takes `--yes`.
- Every refusal exits 2 before any question; every failure is one `Error:` and one `Hint:`. With `--json`, the same refusal or failure is also the `station.error/v1` document on stdout (`schema`, `type`, `code`, `subject`, `message`, `hint`, `fix`), the exit code unchanged. A question answered no is `Error: <what> kept: you said no`, exit 1.
- A word station does not know is `Error: unknown command "<word>"` / `Hint: station help`, exit 2; so is `station help <word>`.
- A word under `tools` or `storage` that names one of its entries, the verb forgotten, is still refused, but its hint is the command meant: `station storage go` is `"go" is a storage row: say what to do with it`, hint `station storage clean go` (`tools install <tool>`). Under `settings`, a setting's name is its read: `station settings dns`.
- `--on <machine>` runs any command below on another machine of the fleet, through its engine, and renders the answer here; see [Machines](machines.md#on-another-machine).
- A value a command needs from you is never an argv flag when it could be a secret: a config item asks in a form on a terminal ([Configuration](configuration.md)).

## Here

These commands act on the machine they run on, and list only what its kind allows.

### `station overview`

```sh
station overview [--all] [--json]
```

Everything about this machine: system, environment, disk, identity, background jobs, Tailscale Serve, tools, config and setup; on a terminal it opens with To do (every row needing action and its fix, a failing or orphan launchd job with the exact removal command, never run by station) and a two-column Summary, the sections folded unless `--all`; on a host its Machine section shows station's engine. `--json` prints `station.overview.read/v1`, named after its call; the hidden alias `station status --json` keeps printing the [`station.status/v1`](status-json.md) document `studio doctor` reads. On a terminal, `overview`, `tools` and `storage` draw every row at once and fill each as its probe answers ([instant reads](../concepts/design-system.md#instant-reads)); piped or with `--json` they print one complete document.

### `station tools`

```sh
station tools [--json]                          # the catalogue tools placed here, agent skills included
station tools install <tool...>
station tools uninstall <tool...> [--yes]
station tools upgrade [<tool...> | --via <manager>...]
```

A bare `upgrade` upgrades every installed catalogue tool. `--via` takes a manager (`brew`, `npm`, `bun`, `uv`, `mise`, `skills`) and cannot be combined with names. A tool whose place excludes this machine is refused with a hint naming where it lives. An agent skill is a tool: `station tools install frontend-design`. The model is [Tools and skills](tools-and-skills.md).

### `station settings`

```sh
station settings [--all] [--json]               # Set by station: applied, changed since, not applied, off here
station settings <setting> [--json]             # one setting's detail
station settings diff <setting>                 # what station would write against what is here
station settings apply [<setting...>] [--yes]   # bare = every setting not applied or changed since that asks nothing
station settings off <setting...> [--yes]       # off here: station neither applies nor watches it on this machine
```

A setting that needs values asks them in a form on a terminal and is refused elsewhere (exit 2); a value is never an argument. An apply over a setting changed since asks first, as yours is lost; off removes only what is still exactly station's, a change of yours stays. The Always-on Mac group is applied by name only. `station config`, `config install` and `config uninstall` are hidden aliases of `settings`, `settings apply` and `settings off` for one release. See [Configuration](configuration.md).

### `station storage`

```sh
station storage [--json]                        # per tool: caches · data · kept · installed, with sizes; nothing runs
station storage clean [<tool...>]               # empty caches with each tool's own command; bare = every row here
station storage reset <tool...> [--yes]         # delete a tool's data, back to factory; asks first
```

`clean` asks nothing: caches only. `reset` is always named, and refuses while the tool's app runs. A stale VM goes with `station machines delete`, an old image version with `station images delete`. See [Storage](storage.md).

### `station power`

```sh
station power [--json]                          # the session, its guards, and this Mac's reading (lid, source, battery, thermal)
station power awake [--lid] [--battery] [--for <d>]   # keep this host awake: 1m..24h, default 4h; held by the engine
station power sleep                             # end the session now
```

The engine that already runs holds the session, in memory only: a `PreventUserIdleSystemSleep` assertion, and with `--lid` the clamshell sleep disabled (no root, no `pmset -a`, no file). Every end sets the clamshell back to normal, and so does every engine start and quit. A guard ends the session first: thermal pressure at trapping (or heavy for 5 minutes lid closed on battery), the battery at 20 % on battery, Low Power Mode, or the lid closed on battery without `--battery`; each end is logged in `engine.log` and kept on the page for 24 hours. With `--lid`, a lid close sleeps the display with `pmset displaysleepnow`, never a display assertion. A guard already past its threshold refuses the start (exit 1). Piped, one record per row: the session (`on`, its flags, until in RFC 3339, who asked), the guards, then the reading as `key<TAB>value`. Unrelated to the `power` config item, the always-on pmset policy of a headless host.

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

`--here` lists this host only, with no network. `create` asks the lifetime on a terminal and, with two reachable hosts, the host; piped it needs `--lifetime`. `delete` is the only verb that destroys a machine; `forget` only unregisters a host. VMs are made and run on hosts, so these commands except `probe` are refused on a VM. `create`, `start` and `restart` need the host's engine; `stop` and `delete` act without it. `--network bridged` is for macOS VMs only. See [Machines](machines.md).

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
station secrets lend --to <vm> [<name...>] [--for <duration>]   # 12h by default, 1m to 7d
station secrets recall --from <vm> [<name...>]
```

See [Secrets](secrets.md).

### `station accounts`

```sh
station accounts [--json]                       # who has access to what, here and in this host's VMs
```

One group per machine: this host's GitHub, npm, Claude, Tailscale, Cloudflare, git, ssh and Docker rights, then each VM's, read inside it by its guest, with the lends its record holds (`lent`, until when). Identities and scopes only: no token, cookie or key body is ever read into a value, and anything shaped like one reads `?`. Piped, one record per account: mark, machine, provider, identity, origin, scope, until (RFC 3339), detail, hint. See [Secrets](secrets.md#accounts).

### `station engine`

```sh
station engine [--json]                         # running, its version, up, the VMs it runs, Awake; or stopped, since when and by what
station engine stop [--yes]                     # Stop everything on this Mac: every running VM stopped, its disk kept; Awake and the sharing end; then the engine
station engine start                            # the engine bootstrapped again; the VMs it stopped stay stopped
```

`--json` is `station.engine.read/v1`: `{schema, engine: {state, version, up_s, vms, stopped_at, stopped_by}}`, `state` `running`, `stopped` or `not-installed`. An engine running another version than station names both and `station setup`. A Mac only; `--on <mac>` runs it on another Mac. `station stop` and `station start` are hidden aliases of `engine stop` and `engine start` for one release, saying `→ now: station engine stop` on stderr (see [Hidden aliases](#hidden-aliases)). A start the job manager refuses ends in `the engine did not start: <its last word>` (exit 1). See [Machines](machines.md#the-engine).

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

`setup` is the one bootstrap verb, safe to run again: it registers this host in the fleet (asking the name on a terminal, taking `--name` otherwise), writes the one managed block of `~/.zshrc`, and moves `~/.studio/` to the XDG folders. `--version` prints `station <version> (<commit>)`. See [Getting started](../start/getting-started.md) and [Dotfiles](dotfiles.md).

## Hidden aliases

A v2 word runs its v3 command with the same arguments, for one release, and prints one muted line on stderr naming it (nothing there with `--json` or `--page`); help and completion never list it.

| Old              | Runs                                                          |
| ---------------- | ------------------------------------------------------------- |
| `station status` | `station overview`; `status --json` keeps `station.status/v1` |
| `station stop`   | `station engine stop`                                         |
| `station start`  | `station engine start`                                        |

## Plumbing

These commands exist for the machine's own use. They are hidden from help and the owner never types them.

| Command                           | What runs it and what it does                                                                                                                                                                                                                         |
| --------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `station shell zsh`               | the `~/.zshrc` block evaluates it at every shell start: PATH, the mise activation, station's completion, and in a VM the lent Claude token and the [profiles'](services.md#profiles) exports. Output on stdout only; another shell or none is refused |
| `station completion <shell>`      | cobra's completion script, for a shell the block does not serve                                                                                                                                                                                       |
| `station engine run`              | the launchd agent `com.jterrazz.station.engine` runs it on a host: the VMs and their services ([Machines](machines.md#the-engine))                                                                                                                    |
| `station guest`                   | the host's engine runs it inside each VM over ssh: what listens there ([Services](services.md))                                                                                                                                                       |
| `station machines pipe <vm>`      | a VM's `~/.ssh/config` ProxyCommand runs it: ssh's bytes to the VM's NAT ip ([Machines](machines.md#ssh-and-herdr))                                                                                                                                   |
| `station machines supervise <vm>` | retired: an old per-VM agent that still runs it exits at once; `station setup` removes those agents                                                                                                                                                   |

`--on` is refused on `shell`, `completion`, `help`, `man`, `daemon`, `engine run`, `engine attach`, `guest`, `machines pipe` and `machines supervise`: none of them sends a call.

## Exit codes

| Code | Means                                               |
| ---- | --------------------------------------------------- |
| 0    | Done                                                |
| 1    | An operation failed, or a confirmation was declined |
| 2    | A refusal, before any question or effect            |

The exit code is computed once, from the error's type (SPEC v3 §12.2): `refused`, `not-found`, `not-for-this-machine`, `needs-confirmation`, `needs-terminal` exit 2; `failed`, `unreachable`, `engine-stopped`, `far-too-old`, `declined` exit 1; `interrupted` exits 130 and prints nothing.
| 130 | Interrupted |

A command over several items prints every outcome, then exits 1 with `<n> of <m> <noun> failed` if any failed.

## Related

- [Design system](../concepts/design-system.md): the forms a document takes.
- [Status contract](status-json.md): the JSON a script reads.
