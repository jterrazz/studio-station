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

The files an install put there (an app bundle, a Cellar, mise's installs, bun's and uv's globals, browser builds) are no level: they are sized as `installed` and removed only by `station apps remove`.

## The folders

station's own files follow the XDG rule: `~/.config/station` (settings), `~/.local/share/station` (Tart and its VMs), `~/.local/state/station` (records, logs, keys), `~/.cache/station` (downloads), `~/.local/bin` (the binary and the tart launcher). Each root honours its `XDG_*_HOME` when it is set and absolute. `~/.cache` stays apart from `~/.local`: everything under it may be deleted.

## Relocation

A tool's path moves only through the tool's own config file, the one it reads whoever launches it (an IDE started from the Dock included), never through a shell variable alone. `station setup` writes them, absolute, and never over a value the owner set by hand:

| Tool | File                                                                                                                     | Keys                                             | Old home |
| ---- | ------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------ | -------- |
| go   | Go's env file: `$GOENV`, else `~/Library/Application Support/go/env` (Linux: `~/.config/go/env`)                         | `GOPATH`, `GOMODCACHE`                           | `~/go`   |
| npm  | `~/.npmrc`                                                                                                               | `cache`                                          | `~/.npm` |
| bun  | `~/.bunfig.toml`                                                                                                         | `globalDir`, `globalBinDir`, `install.cache.dir` | `~/.bun` |
| go   | mise's global config: `$MISE_GLOBAL_CONFIG_FILE`, else `$MISE_CONFIG_DIR/config.toml`, else `~/.config/mise/config.toml` | `settings.go_set_gobin = false`                  |          |

mise's go backend sets `GOBIN` to the running version's own install unless `go_set_gobin` is false, so `go install` would put its binaries where an upgrade or a prune loses them and `storage reset go` never sees them; with it off, they land in `~/.local/share/go/bin`. The key goes under the owner's own `[settings]` table when there is one.

bun's values sit in a managed block between `# >>> station >>>` and `# <<< station <<<`; a bunfig with its own `[install]` table is left alone, and setup says which keys to set by hand. The macOS machine tier proved that `bun add -g`, `bun install -g`, `bunx` and `bun pm cache rm` honour the global bunfig and create no `~/.bun`. `station setup` moves an existing `~/.bun`'s globals and bins into `~/.local/share/bun`, one rename each, so their relative bin links still resolve; `station storage clean bun` deletes its old cache.

Nothing already under `~/Library/Caches` or another `~/Library` folder moves: go-build, Homebrew, mise's cache, the pnpm store and Playwright are in the macOS place for them. The folders a tool keeps at the home root on purpose (`.ssh`, `.gnupg`, `.cargo` and `.rustup`, which rust-analyzer started by a GUI IDE needs, `.claude`, `.codex`, the dotfiles) are the declared exceptions, listed with their reasons in [Footprint](footprint.md#home-root-exceptions).

## Tart

Tart's app and its `LICENSE` live in `~/.local/share/station/tart/`, its home (`TART_HOME`) is `~/.local/share/station/vms`: the VMs and images, the OCI and IPSW caches and `tmp`, so a clone from the cache stays an APFS clone. The launcher `~/.local/bin/tart`, written by the config item `tart-launcher`, sets `TART_HOME` itself: a launchd agent, `--on` over ssh and a GUI all get it, with no shell variable. `station setup` moves an existing `~/.tart` and `~/.local/opt/tart.app` in place, one rename each, never a copy and never a pull; it refuses while `tart` runs, onto a destination that holds anything, and across volumes.

## `station storage`

- `station storage` lists, per tool placed here, its cache size, then its data, kept and installed sizes; `station` first, then the catalogue's categories, then the strays. `--json` is `station.storage/v1`. A row that holds nothing is not shown. The installed size counts the system paths too (`/opt/homebrew`, an app in `/Applications`, `/Library/Developer/CoreSimulator`). Every row is sized in parallel under 10 s; a row past it shows `?` (`-1` in JSON) with `not sized within 10 s`. Docker's size is `docker system df` under 5 s: an installed Docker that does not answer in time shows `?` with `not sized within 5 s`, even with no `~/.docker`.
- `station storage clean [<tool...>]` empties caches with each tool's official command (`go clean -cache -modcache`, `brew cleanup --prune=all`, `tart prune --entries=caches --older-than=0`, `npm cache clean --force`...), or deletes a cache folder its docs call deletable; what the command leaves behind (Homebrew's `api` and `bootsnap`, go's `README`) is deleted after it, and a command that would create the cache it cleans (`pnpm store prune` on no store) is not run. Bare, it cleans every row here that is installed or holds bytes. It asks nothing; a failing action is its row's `✗` and the others still run.
- A tool that is not installed has no command to run: `clean` and `reset` skip its commands and still delete by path, its cache folders, the cache inside its strays, and for `reset` the data it names. With nothing left to delete, `clean` says `○ <tool> not installed`.
- `station storage reset <tool...>` deletes a tool's caches and data, back to factory: always named, asked in a form whose default is Keep, `--yes` off a terminal. It refuses while the tool's app runs. Resetting xcode empties DerivedData and erases every simulator device; the archives stay.

A row with no reset names who owns its data instead: station's VMs and images are `station machines delete` and `station images delete`, Homebrew's formulae are `station apps remove`.

## Strays

A stray is a home-root folder a tool writes when its relocation is missing or was never applied: `~/go`, `~/.npm`, `~/.tart`. `station status` shows each as a `◐` row of Workspace with its size and its repair: `station setup` while something in it is still to move, else `station storage clean <tool>`, which empties it with the old location's own command (or deletes the cache inside it, the tool gone) and removes the emptied folders. A file station does not own stays where it is.

## Backups

The config item `backup-exclusions` keeps `~/.cache` and `~/.local/share/station/vms` out of Time Machine with `tmutil addexclusion` (a sticky exclusion, no root); uninstalling it gives them back.

## Related

- [Footprint](footprint.md): every path, generated from the table.
- [Configuration](configuration.md): the config items and the environment variables.
