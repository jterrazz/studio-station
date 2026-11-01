# Storage

What a tool keeps on a machine, by level, and the one family that sizes, cleans and resets it. Every path of every row is listed, generated from the table, in [Footprint](footprint.md).

| Section           | Answers                                                  |
| ----------------- | -------------------------------------------------------- |
| The levels        | What each level means and who removes it                 |
| The folders       | Where station's own files live                           |
| Relocation        | How a tool's home moves, and what never moves            |
| Tart              | Where the engine and its VMs live                        |
| `station storage` | The list, `clean` and `reset`                            |
| Strays            | The home-root folders a tool left, and how they are seen |
| Backups           | What Time Machine skips                                  |

## The levels

Every path a tool or station writes has one level, declared in `src/catalog/storage.go`.

| Level    | Means                                                                 | Who removes it                                      |
| -------- | --------------------------------------------------------------------- | --------------------------------------------------- |
| `cache`  | rebuilt or re-downloaded on demand: only time and bandwidth are lost  | `station storage clean`, with no question           |
| `data`   | state the owner may wipe to factory: histories, databases, simulators | `station storage reset <tool>`, named and confirmed |
| `config` | the tool's own configuration files, many written by a config item     | nobody: a config item's uninstall, where reversible |
| `kept`   | keys, credentials, the node identity, the VM records                  | nobody                                              |

The files an install put there (an app bundle, a Cellar, mise's installs, bun's and uv's globals, browser builds) are no level: they are sized as `installed` and removed only by `station tools uninstall`.

## The folders

station's own files follow the XDG rule: `~/.config/station` (settings), `~/.local/share/station` (Tart and its VMs), `~/.local/state/station` (records, logs, keys), `~/.cache/station` (downloads), `~/.local/bin` (the binary and the tart launcher). Each root honours its `XDG_*_HOME` when it is set and absolute. `~/.cache` stays apart from `~/.local`: everything under it may be deleted.

## Relocation

A tool's path moves only through the tool's own config file, the one it reads whoever launches it (an IDE started from the Dock included), never through a shell variable alone. `station setup` writes them, absolute, and never over a value the owner set by hand:

| Tool | File                                                                                             | Keys                                             | Old home |
| ---- | ------------------------------------------------------------------------------------------------ | ------------------------------------------------ | -------- |
| go   | Go's env file: `$GOENV`, else `~/Library/Application Support/go/env` (Linux: `~/.config/go/env`) | `GOPATH`, `GOMODCACHE`                           | `~/go`   |
| npm  | `~/.npmrc`                                                                                       | `cache`                                          | `~/.npm` |
| bun  | `~/.bunfig.toml`                                                                                 | `globalDir`, `globalBinDir`, `install.cache.dir` | `~/.bun` |

The bun row is pending: setup does not write it until the macOS machine tier proves that `bun add -g`, `bun install -g` and `bunx` honour the global bunfig and create no `~/.bun` (`TODO.md`, "Machine test tiers"). Until then `~/.bun` is a declared exception.

Nothing already under `~/Library/Caches` or another `~/Library` folder moves: go-build, Homebrew, mise's cache, the pnpm store and Playwright are in the macOS place for them. The folders a tool keeps at the home root on purpose (`.ssh`, `.gnupg`, `.cargo` and `.rustup`, which rust-analyzer started by a GUI IDE needs, `.claude`, `.codex`, the dotfiles) are the declared exceptions, listed with their reasons in [Footprint](footprint.md#home-root-exceptions).

## Tart

Tart's app and its `LICENSE` live in `~/.local/share/station/tart/`, its home (`TART_HOME`) is `~/.local/share/station/vms`: the VMs and images, the OCI and IPSW caches and `tmp`, so a clone from the cache stays an APFS clone. The launcher `~/.local/bin/tart`, written by the config item `tart-launcher`, sets `TART_HOME` itself: a launchd agent, `--on` over ssh and a GUI all get it, with no shell variable. `station setup` moves an existing `~/.tart` and `~/.local/opt/tart.app` in place, one rename each, never a copy and never a pull; it refuses while `tart` runs, onto a destination that holds anything, and across volumes.

## `station storage`

- `station storage` lists, per tool placed here, its cache size, then its data, kept and installed sizes; `station` first, then the catalogue's categories, then the strays. `--json` is `station.storage/v1`. A row that holds nothing is not shown. The installed size counts the system paths too (`/opt/homebrew`, an app in `/Applications`, `/Library/Developer/CoreSimulator`). Every row is sized in parallel under 10 s; a row past it shows `?` (`-1` in JSON).
- `station storage clean [<tool...>]` empties caches with each tool's official command (`go clean -cache -modcache`, `brew cleanup --prune=all`, `tart prune --entries=caches --older-than=0`, `npm cache clean --force`...), or deletes a cache folder its docs call deletable. Bare, it cleans every row here that is installed or holds bytes. It asks nothing; a failing action is its row's `✗` and the others still run.
- `station storage reset <tool...>` deletes a tool's caches and data, back to factory: always named, asked in a form whose default is Keep, `--yes` off a terminal. It refuses while the tool's app runs. Resetting xcode empties DerivedData and erases every simulator device; the archives stay.

A row with no reset names who owns its data instead: station's VMs and images are `station machines delete` and `station images delete`, Homebrew's formulae are `station tools uninstall`.

## Strays

A stray is a home-root folder a tool writes when its relocation is missing or was never applied: `~/go`, `~/.npm`, `~/.tart`. `station status` shows each as a `◐` row of Workspace with its size and its repair: `station setup` while something in it is still to move, else `station storage clean <tool>`, which empties it with the old location's own command and removes the emptied folders. A file station does not own stays where it is.

## Backups

The config item `backup-exclusions` keeps `~/.cache` and `~/.local/share/station/vms` out of Time Machine with `tmutil addexclusion` (a sticky exclusion, no root); uninstalling it gives them back.

## Related

- [Footprint](footprint.md): every path, generated from the table.
- [Configuration](configuration.md): the config items and the environment variables.
