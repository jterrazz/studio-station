# Design system

Everything `station` shows, in a terminal, a pipe or the TUI, comes from one palette, one mark vocabulary and one document model.

| Section           | Answers                                                |
| ----------------- | ------------------------------------------------------ |
| The palette       | The colours, by meaning                                |
| The marks         | The four glyphs and what each says                     |
| Typography        | Margins, rows, columns and empty states                |
| Header and footer | What frames every document                             |
| Pipe and JSON     | The record format a script reads, written once         |
| The TUI           | Tabs, focus modes, the action rule and the loading law |

## The palette

Semantic tokens, each a light and a dark value chosen by the terminal's background (Catppuccin Latte and Macchiato, the flavour the dotfiles already use). `src/ui/token/palette.go` is the only place a colour is written; the layer test refuses a hex or a colour constructor anywhere else.

| Token       | Used for                                                      |
| ----------- | ------------------------------------------------------------- |
| `Text`      | body text                                                     |
| `Muted`     | hints, details, `○`, secondary text                           |
| `Subtle`    | rules and inactive tabs only; never text that carries meaning |
| `Surface`   | borders and the modal frame                                   |
| `Selection` | the cursor row's background                                   |
| `Accent`    | active tab, section titles, the header name                   |
| `Good`      | `●`                                                           |
| `Warn`      | `◐`                                                           |
| `Bad`       | `✗` and `Error:`                                              |

The same tokens colour Fang's help and the forms that ask a config item's values.

## The marks

| Mark | Means                                       |
| ---- | ------------------------------------------- |
| `●`  | ok                                          |
| `◐`  | degraded, or unknown                        |
| `○`  | absent                                      |
| `✗`  | broken                                      |
| `◌`  | pending                                     |
| `⊘`  | off here, your choice                       |
| `–`  | not for this machine                        |
| `?`  | not managed by station, or not readable now |

The glyph is the meaning and survives a pipe and `NO_COLOR`; colour only reinforces it. `◌` (Muted) marks a row whose probe has not answered yet, on a terminal and in the TUI only: a pipe, `NO_COLOR` and `--json` wait for every probe, so they never carry it. `⊘` and `?` are Muted, `–` Subtle, each always followed by its word (`page.OffHere`, `page.NotForThisMachine`, `page.NotKnown`); the families draw them as they take their v3 words. Every glyph station prints is a token of `src/ui/token/glyphs.go` (`token.Glyphs`), and the layer test refuses one in any other Go literal of a migrated package. Folding in the TUI uses `+` and `-` before a group title; the cursor is a background, not a glyph.

## Typography

- The left margin is one space. Headers, rules, section titles and hints start at the margin; rows start two columns in.
- A blank line precedes every section. A section title is `Accent` and bold, followed by its counts (`58 ●  2 ◐  14 ○`, each in its mark's tone).
- A row is a mark, a space, then cells separated by two spaces. Column widths are computed per document, so every section lines up, and the last cell is never padded.
- A cell is a name (strong), a value, a method (muted), then a detail (muted, cut with `…` to the width by its kind, a path in its middle, never cut in a pipe; beside the row's own cells when its last columns are blank and the aligned place would cut it; never cut below its kind's floor: a path or a URL then goes on its own line under the row, any other detail is lost on that width). A long path in a column is cut in the middle before it takes a detail's room. A row that is not `●` may end with the one command that repairs it.
- Sizes are humanised by one function; numbers are right-aligned.
- An empty state is one muted row (`none`, `nothing to clean`, `no machines registered`) plus a hint naming the command that fills it.

## Header and footer

```text
 station tools                                                macbook · host
 ────────────────────────────────────────────────────────────────────────────
```

Left, `station` and the command path; right, this machine's name and kind. A rule runs under it. The footer is a blank line, then the hints, one per line, each prefixed `→ `.

## Instant reads

`station overview`, `storage` and `tools` on a terminal draw their frame at once: every row the catalogue or a probe table declares, each `◌` with its name only (a size shows its last-known value, Muted, from `~/.cache/station/sizes.json`). Each row fills in place as its probe answers, the region redrawn at most every 50 ms and clipped to the terminal's height with a Muted `… <n> rows pending` last line; a probe past its budget answers `◐ ?` with `no answer in <budget>`. When the last probe answers the region is erased and the final document printed once, so the scrollback holds what a pipe would. The probes run on two bounded lanes (16 commands, one walk of 8 workers), and one reading per run is shared: the tools inventory feeds status, tools and storage; the storage walk feeds the Storage tab and the status `disk` row.

## Pipe and JSON

When stdout is not a terminal, or `NO_COLOR` is set, a document prints as records, one per line, with no header, rule, footer or padding and no colour sequence. The separator is a tab everywhere:

- a section title is `# <title>`; the tools and config categories, and the status Tools, Config and Background sections, follow it with their counts as `<n> <glyph>` cells (every other title is bare, as its records always were); Secrets' Lend list and Kept, one section on a terminal and in the TUI, pipe as the two titled sections `# Lend list` and `# Kept`;
- a column for the eye only (a peer's online state and tags, a host's role) is no field: the records keep the fields scripts already cut;
- a row is `<glyph>` then its cells; a row with no mark starts with an empty field;
- a key and its value are `<key><TAB><value>`;
- a blank cell stays (two tabs); trailing empty cells are trimmed.

So `cut -f2` is a row's name. `--json` prints the presenter's JSON value, two-space indented, with a trailing newline; the key conventions are `name`, `ip`, `*_at`, `*_bytes`, `lend_list` and `vms`, one key per concept across every schema.

## The TUI

`station` on a terminal opens one Bubble Tea app with the tabs Home, Tools, Config, Storage, then Fleet, Services, Accounts and Power. Each tab renders the same page as its command. The tab line never wraps: when it does not fit, the context shortens to the machine name (then `host`), the inactive tabs turn into their number key one at a time from the last, then the context goes. While a Power session holds, the context ends with `⏻ awake` (`⏻ awake · lid` with `--lid`), the last thing to go.

| Tab      | Holds                                                                                                                   | Keys                                                                                                                                                     |
| -------- | ----------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Home     | To do (each action and its fix), the Summary, then the status sections folded, with a live strip while the tab is shown | `enter` on an action runs its station fix; an External one (launchctl, sudo) only opens its detail                                                       |
| Tools    | the catalogue by category, agent skills last                                                                            | `i` install, `u` uninstall, `U` upgrade, `/`                                                                                                             |
| Config   | the config items placed here; an item with inputs opens its form                                                        | `i`, `u`                                                                                                                                                 |
| Storage  | each tool's storage by level, then the strays                                                                           | `c` clean the row (no question), `x` reset it (the reset form, Keep by default), `/`                                                                     |
| Fleet    | every host and its VMs, images, secrets and the tailnet                                                                 | `n` create, `s` start/stop, `d` delete, `f` forget, `p` probe, `o` unlock a host, `b` build images, `ctrl+r` restart, `e` opens `station --on <it>` here |
| Services | every host's range and its leases, a QR code of the selected lease's URL                                                | `x` releases the selected lease (another host's on that host)                                                                                            |
| Accounts | this host's and its VMs' identities and scopes, where each right comes from, a lend's time left; never a token          | `l` lends (the lend Form, then `secrets lend --for`), `r` recalls a lent row (an own row shows its logout), `R` reloads: `r` is revoke here              |
| Power    | the session, its guards and the reading, on the engine's `page` watch, so a guard's end redraws it at once              | `a` the awake Form (for, lid, battery), `s` sleep                                                                                                        |

Focus modes: the app holds exactly one. Normal owns the global keys (`tab`, `1` to `8`, `r` but on a tab that binds it, `?`, `q`); Filter gives the text input every key except `esc` and `enter`, so typing `q1i` filters by `q1i`; Modal gives a form every key except `ctrl+c`.

The action rule: every mutating key releases the screen, makes the one call with the real terminal, then restores the screen, shows the outcome in the status bar and reloads. A tab asks only when that call asks (`tab.Confirmed`): the call runs unanswered first, and the question is put once its use case refuses with a confirmation. Reads run as commands with a spinner. One action at a time per screen.

The loading law: a screen's `Init` returns the commands that call the domain, results arrive as messages, and `View` only renders the model. A view never calls a service, a probe or the system. Every tab starts its reading at once, so the first frame shows each tab's rows pending and every tab fills in the background; the shown tab's probes go first, and the header spinner turns while one of them is pending. `r` drops the shared reading and starts a new one.

## Related

- [Architecture](architecture.md): the output layer that decides the form.
- [Commands](../reference/commands.md): the commands these documents belong to.
