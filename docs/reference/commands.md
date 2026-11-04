# Commands

Every command of `station`, grouped as `station --help` groups them. The words are the [Glossary](../concepts/glossary.md)'s.

| Section        | Answers                                                |
| -------------- | ------------------------------------------------------ |
| Rules          | What holds for every command                           |
| Here           | `overview`, `apps`, `settings`, `disk`                 |
| The fleet      | `machines`, `services`, `images`, `secrets`, `tailnet` |
| Shortcuts      | `run git` and `run docker`                             |
| The binary     | `setup`, `--version`, `help`                           |
| Hidden aliases | The v2 words that still run, for one release           |
| Plumbing       | The hidden commands the machine runs for itself        |
| Exit codes     | What 0, 1, 2 and 130 mean                              |

`station help` groups the commands as SPEC v3 §4.1 does: MACHINES (`machines`), ON A MACHINE (`overview`, `apps`, `settings`, `disk`, `services`, `tailnet`, `secrets`, `accounts`, `power`, `run`), A MAC'S IMAGES AND ENGINE (`images`, `engine`), THIS TERMINAL (shown once `use` exists) and STATION ITSELF (`setup`, `help`). The sections below keep their v2 headings until each family takes its v3 words.

## Rules

- Bare `station` opens the TUI on a terminal; piped, or with `--all` or `--json`, it prints `station overview`, exit 0.
- A bare noun lists (`apps`, `settings`, `disk`, `machines`, `services`, `images`, `secrets`, `tailnet`). A bare group with nothing to list (`run`, `run git`) prints its menu on stderr and exits 2. A verb whose argument is missing is refused, its hint the command with it: `station apps install` hints `station apps install <app>`.
- An existing thing is a positional (`machines stop api`); a new thing is described by options (`machines create api --permanent`).
- The same verbs everywhere: `install`, `uninstall`, `upgrade`, except where a family has taken its v3 words (`apps`: `install`, `update`, `remove`). The only other words are the [hidden aliases](#hidden-aliases).
- Every read takes `--json`. Every destructive action asks on a terminal or takes `--yes`.
- Every refusal exits 2 before any question; every failure is one `Error:` and one `Hint:`. With `--json`, the same refusal or failure is also the `station.error/v1` document on stdout (`schema`, `type`, `code`, `subject`, `message`, `hint`, `fix`), the exit code unchanged. A question answered no is `Error: <what> kept: you said no`, exit 1.
- A word station does not know is `Error: unknown command "<word>"` / `Hint: station help`, exit 2; so is `station help <word>`.
- A word under `disk` that names one of its entries, the verb forgotten, is still refused, but its hint is the command meant: `station disk go` is `"go" is a disk row: say what to do with it`, hint `station disk free go`. Under `apps` the entry is a read: `station apps jq` is jq's detail; under `settings`, a setting's name is its read: `station settings dns`.
- `--on <machine>` runs any command below on another machine of the fleet, through its engine, and renders the answer here; see [Machines](machines.md#on-another-machine).
- A value a command needs from you is never an argv flag when it could be a secret: a config item asks in a form on a terminal ([Configuration](configuration.md)).

## Here

These commands act on the machine they run on, and list only what its kind allows.

### `station overview`

```sh
station overview [--all] [--json]
```

Everything about this machine: system, environment, disk, identity, background jobs, Tailscale Serve, apps, config and setup; on a terminal it opens with To do (every row needing action and its fix, a failing or orphan launchd job with the exact removal command, never run by station) and a two-column Summary, the sections folded unless `--all`; on a host its Machine section shows station's engine. `--json` prints `station.overview.read/v1`, named after its call; the hidden alias `station status --json` keeps printing the [`station.status/v1`](status-json.md) document `studio doctor` reads. On a terminal, `overview`, `apps` and `disk` draw every row at once and fill each as its probe answers ([instant reads](../concepts/design-system.md#instant-reads)); piped or with `--json` they print one complete document.

### `station apps`

```sh
station apps [<app>] [--all] [--json] # the apps for this machine, by attention; one app's detail
station apps install <app...>
station apps update [<app...> | --method <method>... | --all-machines]
station apps remove <app...> [--yes]
```

The list sums This Mac first (`108 apps for this Mac · 3 updates · 100 not installed`), then groups the apps by attention: Updates, Failed, Not installed, Not managed (`?`: present, but not installed by its catalogue method, the detail saying how it got there), and OK folded; the apps that live on other machines are a count, every row of every group unfolded with `--all`. `station apps <app>` is one app's detail: its method, its version and the newest that waits (`2.97.0 → 2.98.0`), where it lives, its category and what it brings. `--json` is `station.apps.list/v1`, each app with `name`, `category`, `method`, `state`, `version`, `newest` (with an update only), `lives_on` (`every-machine`, `mac-only`, `vms-only`), `provides` and `detail`.

`install` installs in dependency order, then the settings each app brings; on a terminal its spinner names what runs, and ctrl-c stops it with nothing printed after, exit 130, what finished staying installed. Its outcome opens with the count, the machine and the time (`● Installed 2 apps on This Mac 9s`), then each app with the version and the method that hold it; the pipe keeps the app rows alone. An app already installed says so with its version, `● jq 1.8.1 brew already installed`, exit 0. A bare `update` updates only the apps with an update and says so the same way (`● Updated 3 apps on This Mac`; `● every app is up to date on This Mac` when none); `--method` takes `brew` (brew and cask), `npm`, `bun`, `uv`, `mise` or `skills`, never with names; `--all-machines` updates each Mac and running workspace in turn, says each server and stopped VM it skips, and exits 1 when a Mac does not answer. `remove` asks on a terminal and needs `--yes` off one; an app not installed is refused before anything runs.

The refusals exit 2: `no app named "jqq"`, `dia lives on Macs only` (hint `--on <mac>`), `lazygit is not installed`, `infra is a server: station does not manage its apps`; an app whose method's manager is missing (`keynote needs mas, which is not installed`, hint `station apps install mas`) exits 1. An agent skill is an app: `station apps install frontend-design`. The model is [Apps and skills](tools-and-skills.md).

### `station settings`

```sh
station settings [--all] [--json] # Set by station: applied, changed since, not applied, off here
station settings <setting> [--json] # one setting's detail
station settings diff <setting> # what station would write against what is here
station settings apply [<setting...>] [--yes] # bare = every setting not applied or changed since that asks nothing
station settings off <setting...> [--yes] # off here: station neither applies nor watches it on this machine
```

A setting that needs values asks them in a form on a terminal and is refused elsewhere (exit 2); a value is never an argument. An apply over a setting changed since asks first, as yours is lost; off removes only what is still exactly station's, a change of yours stays. The Always-on Mac group is applied by name only. `station config`, `config install` and `config uninstall` are hidden aliases of `settings`, `settings apply` and `settings off` for one release. See [Configuration](configuration.md).

### `station disk`

```bash
station disk [--json] # the volume, then Misplaced, Can rebuild, Your data, Never touched, with sizes; nothing runs
station disk free [<app...>] # empty what an app can rebuild with its own command; bare = every Can rebuild row
station disk erase <app> [--confirm=<app>] # erase an app's data, back to factory; its name typed
station disk move <folder> # move a misplaced folder where its app's own config says
```

`free` and `move` ask nothing: nothing is lost. `erase` is always one app, its name typed, and refuses while the app runs or on a Never touched row. A stale VM goes with `station machines delete`, an old image version with `station images delete`. The v2 `station storage`, `storage clean` and `storage reset` are hidden aliases for one release. See [Disk](disk.md).

### `station power`

```sh
station power [--json] # the session, its guards, and this Mac's reading (lid, source, battery, thermal)
station power awake [--lid] [--battery] [--for <d>] # keep this host awake: 1m..24h, default 4h; held by the engine
station power sleep # end the session now
```

The engine that already runs holds the session, in memory only: a `PreventUserIdleSystemSleep` assertion, and with `--lid` the clamshell sleep disabled (no root, no `pmset -a`, no file). Every end sets the clamshell back to normal, and so does every engine start and quit. A guard ends the session first: thermal pressure at trapping (or heavy for 5 minutes lid closed on battery), the battery at 20 % on battery, Low Power Mode, or the lid closed on battery without `--battery`; each end is logged in `engine.log` and kept on the page for 24 hours. With `--lid`, a lid close sleeps the display with `pmset displaysleepnow`, never a display assertion. A guard already past its threshold refuses the start (exit 1). Piped, one record per row: the session (`on`, its flags, until in RFC 3339, who asked), the guards, then the reading as `key<TAB>value`. Unrelated to the `power` config item, the always-on pmset policy of a headless host.

## The fleet

### `station machines`

```sh
station machines [--json] # All machines: every Mac, its VMs under it, then Needs you
station machines <machine> [--json] # that machine's Overview, as station overview --on <machine>
station machines add --name <name> [--user <login>] [--key <file>]
station machines forget <host> [--yes]
station machines create <name> [--permanent] [--image linux] [--cpus <n>] [--memory <size>] [--disk <size>] [--network nat|bridged]
station machines start <vm>
station machines stop <vm> [--yes]
station machines restart <machine> [--yes]
station machines edit <vm> [--cpus <n>] [--memory <size>] [--disk <size>]
station machines ssh <machine>
station machines delete <vm> [--confirm=<name>]
station machines unlock <host>
```

`--here` lists This Mac only, with no network. `create` makes a throwaway workspace unless `--permanent` is given, and asks the Mac on a terminal when two answer; piped it needs no question. `delete` is the only verb that destroys a machine; `forget` only unregisters a Mac. VMs are made and run on Macs, so these commands are refused on a VM. `create`, `start`, `stop`, `restart`, `edit` and `delete` need the Mac's engine (`Hint: station engine start`). `station machines <machine>` is that machine's Overview (`probe` is its hidden v2 alias). `--network bridged` is for macOS VMs only. See [Machines](machines.md).

### `station services`

```sh
station services [--here] [--json] # what the fleet's VMs listen on, and each URL
station services url <port> [--json] # in a VM: reserve the port's public URL now, and print it
station services release <vm>:<port> [--yes] # on a host; in a VM: station services release <port>
```

Every host is asked over ssh; `--here` asks none, and is refused in a VM (exit 2), which lists only its own services. `url` runs inside a VM only, its piped stdout the URL alone. A profile's lease asks before `release`. See [Services](services.md).

### `station images`

```sh
station images [--json] # every Mac's, one row per version per Mac
station images build [<image...>] [--binary <linux station binary>]
station images update [<image...>] # the newest published version
station images delete <image>[@<version>] [--yes]
```

A bare `build` builds every image that is missing or whose recipe changed; a bare `update` updates every image this Mac holds. An image the catalogue does not hold is `no image "debian"` (exit 2). See [Images](images.md).

### `station secrets`

```sh
station secrets [--json] # the lend list and what was lent where, names only
station secrets allow <name...>
station secrets disallow <name...> [--yes]
station secrets lend --to <vm> [<name...>] [--for <duration>] # 12h by default, 1m to 7d
station secrets recall --from <vm> [<name...>]
```

See [Secrets](secrets.md).

### `station accounts`

```sh
station accounts [--json] # who has access to what, here and in this host's VMs
```

One group per machine: this host's GitHub, npm, Claude, Tailscale, Cloudflare, git, ssh and Docker rights, then each VM's, read inside it by its guest, with the lends its record holds (`lent`, until when). Identities and scopes only: no token, cookie or key body is ever read into a value, and anything shaped like one reads `?`. Piped, one record per account: mark, machine, provider, identity, origin, scope, until (RFC 3339), detail, hint. See [Secrets](secrets.md#accounts).

### `station engine`

```sh
station engine [--json] # running, its version, up, the VMs it runs, Awake; or stopped, since when and by what
station engine stop [--yes] # Stop everything on this Mac: every running VM stopped, its disk kept; Awake and the sharing end; then the engine
station engine start # the engine bootstrapped again; the VMs it stopped stay stopped
```

`--json` is `station.engine.read/v1`: `{schema, engine: {state, version, up_s, vms, stopped_at, stopped_by}}`, `state` `running`, `stopped` or `not-installed`. An engine running another version than station names both and `station setup`. A Mac only; `--on <mac>` runs it on another Mac. `station stop` and `station start` are hidden aliases of `engine stop` and `engine start` for one release, saying `→ now: station engine stop` on stderr (see [Hidden aliases](#hidden-aliases)). A start the job manager refuses ends in `the engine did not start: <its last word>` (exit 1). See [Machines](machines.md#the-engine).

### `station tailnet`

```sh
station tailnet [--json] # the Tailscale app's node, the peers, what serve exposes
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

`station disk free docker` prunes Docker's unused data.

## The binary

```sh
station setup [--name <name>]
station --version
station help [command]
```

`setup` is the one bootstrap verb, safe to run again: it names this Mac (asking the name on a terminal, taking `--name` otherwise; off a terminal, a Mac with no name yet is refused, exit 2, `Hint: station setup --name <name>`), writes the one managed block of `~/.zshrc`, moves `~/.studio/` to the XDG folders, starts the engine, installs this Mac's missing system apps (on a terminal; piped it names them), then applies the Set by station settings that need no value and no admin. The others are listed as next steps, one row each (`next: station settings apply ssh-key`); the Always-on Mac group is opt-in, so setup neither applies nor lists it. Inside a workspace it writes the block and the layout only; inside a server it sets nothing up, the server's own tooling keeps it. `--version` prints `station <version> (<commit>)`. See [Getting started](../start/getting-started.md) and [Dotfiles](dotfiles.md).

## Hidden aliases

A v2 word runs its v3 command with the same arguments, for one release, and prints one muted line on stderr naming it (nothing there with `--json` or `--page`); help and completion never list it.

| Old              | Runs                                                                                                                    |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `station status` | `station overview`; `status --json` keeps `station.status/v1`                                                           |
| `station stop`   | `station engine stop`                                                                                                   |
| `station start`  | `station engine start`                                                                                                  |
| `station tools`  | `station apps`; `install`, `uninstall` and `upgrade --via` run `apps install`, `apps remove` and `apps update --method` |

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
