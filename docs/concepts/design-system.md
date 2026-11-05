# Design system

Everything `station` shows, in a terminal, a pipe or the TUI, comes from one palette, one mark vocabulary and one document model.

| Section           | Answers                                              |
| ----------------- | ---------------------------------------------------- |
| The palette       | The colours, by meaning                              |
| The marks         | The four glyphs and what each says                   |
| Typography        | Margins, rows, columns and empty states              |
| Header and footer | What frames every document                           |
| Pipe and JSON     | The record format a script reads, written once       |
| The TUI           | The shell, its focus, its widths and the loading law |

## The palette

Semantic tokens, each a light and a dark value chosen by the terminal's background (Catppuccin Latte and Macchiato, the flavour the dotfiles already use). `src/ui/token/palette.go` is the only place a colour is written; the layer test refuses a hex or a colour constructor anywhere else.

| Token       | Used for                                                          |
| ----------- | ----------------------------------------------------------------- |
| `Text`      | body text                                                         |
| `Muted`     | hints, details, `○`, secondary text                               |
| `Subtle`    | rules and unfocused borders only; never text that carries meaning |
| `Surface`   | borders and the modal frame                                       |
| `Selection` | the cursor row's background                                       |
| `Accent`    | the focused border, the header name                               |
| `Good`      | `●`                                                               |
| `Warn`      | `◐`                                                               |
| `Bad`       | `✗` and `Error:`                                                  |

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

- On a terminal, a CLI page starts at the first column with the line that sums it up (`This Mac · 108 apps for this Mac · 2 updates`), in plain text, then one blank line; the line is never cut: a part it cannot hold goes on the next line, two columns in; its sections read on without one, rows two columns in, and a section with nothing to do is one line. The TUI keeps a one-space margin.
- A section title is `Text` and bold, bright with no hue (vision 07). A titled first line leads with its machine in bold (`This Mac`); its counts stay `Text`, as state colours are on marks only (K1). In a value, only a glyph takes its mark's colour, the words around it stay `Text` (`● 39 ok`). An open section's title counts nothing; a folded one is one line: its title, its count by mark (`○ 34`), then as many of its rows' names as fit (`yazi, dust, eza, …`). A brief section (`page.Brief`) is folded so on a terminal and open in the TUI; a calm one (`page.Calm`) is folded so on a terminal and, in the TUI, folded while every row is calm, as an `Auto` one is. A section that only says something is one line on a terminal too, its title then its words (`Not for this machine  7 apps live on other machines`), and so is a calm section of one row with nothing to do: its title, the row's mark and its words (`Always-on Mac   ○ 5 · for a Mac that hosts servers`), or the row alone when its label already says the title (`– 12 not for this machine · Mac only`); a table, whose columns are headed, keeps its heads.
- What needs you comes first. A page whose rows that need a look stay in their own groups lifts them first on a terminal, in a section of the role `page.Lifted` (Settings' `Needs a look`); the pipe, the TUI and the menu bar read those rows where they are, so a record is printed once.
- Another machine's output starts with `On <machine>:`, the machine `--on` named (tty and pipe).
- A row is a mark, a space, then cells separated by two spaces. Column widths are computed per document, so every section lines up, and the last cell is never padded.
- A cell is a name (strong), a value, a method (muted), then a detail (muted, cut with `…` to the width by its kind, a path in its middle, never cut in a pipe; beside the row's own cells when its last columns are blank and the aligned place would cut it; never cut below its kind's floor: a path or a URL then goes on its own line under the row, any other detail is lost on that width). A long path in a column is cut in the middle before it takes a detail's room. A row that is not `●` may end with the one command that repairs it. On a terminal a name is never cut, nor a command (a column of `page.KindCommand`, as Needs you's fixes): a row whose name is wider than the name column, or whose command does not fit beside the names, writes its name whole and its cells, detail and hint on the lines under it, at the name's column.
- Sizes are humanised by one function; numbers are right-aligned.
- An empty state is one muted row (`none`, `nothing to free`, `no machines registered`) plus a hint naming the command that fills it.

## Header and footer

A CLI page has no header: its first line says where it is (`This Mac · …`, `On mac-mini:`). The TUI draws its own header (UI-SPEC). The footer is a blank line, then the next steps, one per line, each prefixed `→ `, their words after the command at one column for the page.

## Instant reads

`station overview`, `disk` and `apps` on a terminal draw their frame at once: every row the catalogue or a probe table declares, each `◌` with its name only (a size shows its last-known value, Muted, from `~/.cache/station/sizes.json`). Each row fills in place as its probe answers, the region redrawn at most every 50 ms and clipped to the terminal's height with a Muted `… <n> rows pending` last line; a probe past its budget answers `◐ ?` with `no answer in <budget>`. When the last probe answers the region is erased and the final document printed once, so the scrollback holds what a pipe would. The probes run on two bounded lanes (16 commands, one walk of 8 workers), and the Overview owns no reading of its own: each of its lines is its section's read through the Router, the same read `apps` and `disk` draw.

## Pipe and JSON

When stdout is not a terminal, or `NO_COLOR` is set, a document prints as records, one per line, with no rule, footer or padding and no colour sequence. The separator is a tab everywhere:

- a section title is `# <title>`; the tools and config categories, and the status Tools, Config and Background sections, follow it with their counts as `<n> <glyph>` cells (every other title is bare, as its records always were); Secrets' Lend list and Kept, one section on a terminal and in the TUI, pipe as the two titled sections `# Lend list` and `# Kept`;
- a column for the eye only (a peer's online state and tags, a host's role) is no field: the records keep the fields scripts already cut;
- a row is `<glyph>` then its cells; a row with no mark starts with an empty field;
- a key and its value are `<key><TAB><value>`;
- a blank cell stays (two tabs); trailing empty cells are trimmed.

So `cut -f2` is a row's name. `--json` prints the presenter's JSON value, two-space indented, with a trailing newline; the key conventions are `name`, `ip`, `*_at`, `*_bytes`, `share_list` and `vms`, one key per concept across every schema.

## The TUI

`station` on a terminal opens one Bubble Tea app, the shell of SPEC §9: the header names the target machine with its mark, what it is, its engine and its Awake (a narrow terminal drops the hostname, then Awake, then the engine word); the breadcrumb under it (`All machines › mac-mini › infra › Overview`); the sidebar, MACHINES as a tree (All machines, each Mac with its Needs-you count, its servers then its workspaces) and then the selected machine's sections in the one order of `section.For`, those its own Overview has a line for (the machine gates each read by its capabilities, so a Linux workspace shows no Background; the kind's table stands in until that Overview answers), each with its badge (its worst mark, or Overview's count); the master pane, the section's page; the detail pane, which follows the cursor in a list section; the footer. Every screen is the page its command prints: the shell reads `station <words> --page` (`overview --all --on linux-1`, `ports --all-machines`), so a page has one function on every surface. All machines' Overview is the table of `pages/machines.Table`: the machines page joined, by each machine's name, with Activity and Ports across every machine and each Mac's Overview (KIND, CPU with a sparkline of the session's reads, MEMORY, DISK, PORTS public · private, AWAKE, NEED), then Needs you across every machine; `enter` opens the machine under the cursor.

The app holds exactly one focus: the sidebar, the master or the detail. `tab` and `shift+tab` move it pane to pane, `←` `→` between the sidebar and the list, `[` `]` the previous and next section, `enter` opens (a machine of the tree or of the All machines table, a section, a folded group, a row's detail), `esc` backs up one level (a filter, the detail, then the machine's parent), `/` filters a list and the title writes it (`Apps (/node) 3/41`), `ctrl+r` reads the section again, `?` lists every key of the focused pane then every key that works everywhere, `q` quits. The footer shows only the keys that work for the focused pane. Filter mode gives the text every key but `esc` and `enter`, so a typed `q` filters.

Widths: from 110 columns, three panes; under 110 the detail opens with `tab` over the list; a table too wide for its pane leaves out its least kept columns whole and says which (`85 columns: MEMORY, DISK and AWAKE left out`); under 80 the sidebar goes and the breadcrumb leads; under 60 × 16 one line says the size station needs. With `NO_COLOR` every glyph stays, the focused pane's border is heavy (`┏━┓`) where the others are rounded, and the cursor row is reverse video.

A section with keys of its own is a screen, under `surface/tui/screens/` (Activity, Awake, Ports, a VM's Overview and SSH): it adds to its page's words only what it keeps itself (Activity's graph window, `--graph 6m`, a flag no help shows), draws its lines or lets the list draw them, names its keys for the footer, keeps the list's cursor off the sections it draws no row of (a screen that draws its own lines draws the cursor on the row it is on, and the list's filter still keeps its rows), and says how often it is read again while shown (Activity every 2 s, Awake every second, so a guard's end redraws at once). A screen may read pages beside its own and draw fixed panes at the right of the master pane, under it on a terminal narrower than 110 columns: a server's Overview reads its Activity over the last hour, its Mac's Ports and its ssh command, and draws its state, what it was created with, its last hour, its door and what station leaves to its tooling, with its Health and its Ports beside; its SSH screen reads its Overview and its Mac's ssh command to say each hop that reaches it. All machines › Ports is one wide pane, as SPEC §9.2 draws it: a table per class (MACHINE, PORT, WHAT, then LISTENS ON and DECLARED, or ROUTE and VERIFIED, or the link), the fix of what needs you said once in full, the count of what listens on its machine only, which `a` unfolds (`ports --all`); `enter` opens a row's detail, its QR code with it, over the pane. All machines › Activity draws a row per machine, each VM one step in under its Mac, CPU with a sparkline of the session's reads, how its rates were read, what needs you from them, and each Mac's thermal state and battery, which its page carries only for the TUI (`activity --all-machines --graph`). While the TUI shows a machine's activity (its Activity or its Overview, or All machines for every machine of this Mac) it holds this Mac's engine's watch on it, as the menu bar open does, so the engine samples it every 2 s. A key's call is the CLI's command for its words, run as the same binary: a yes/no question first, or a typed name (`S` stops a server, `enter` off until `infra` is typed), or a form whose answers make the words; while it runs, and for 4 s after, the Progress line above the footer says it. A copy (`c`) goes to the terminal's clipboard (OSC 52) and is said there too. The graphs are `ui/atom`'s: a sparkline from its label whose columns are the mean of their points, the newest at its right end, and a bar whose full and light blocks carry the share without colour.

The loading law: the shown section is read first, then All machines and each other Mac's Overview. Every read runs in a command off the frame, its answer arrives as a message, and the view only draws what has answered: until then a section is `◌` with its name. A view never calls a read or the system; the architecture rule `tui-loading-law` refuses a call of the TUI's read outside a command.

## Related

- [Architecture](architecture.md): the output layer that decides the form.
- [Commands](../reference/commands.md): the commands these documents belong to.
