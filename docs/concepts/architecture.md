# Architecture

`station` is one Go binary in layers: an input layer parses argv, a command calls one domain operation, a pure presenter turns the result into a document, and one output layer prints it. The layers mirror the estate's TypeScript `studio` CLI, and a test refuses an import that crosses them.

| Section                   | Answers                                                         |
| ------------------------- | --------------------------------------------------------------- |
| The layers                | The packages under `src/` and what each one holds               |
| Who may import whom       | The direction dependencies flow, and the test that owns it      |
| The path of a request     | What happens between argv and the bytes on the terminal         |
| The error block           | The one error shape, and the exit codes                         |
| Fang behind the CLI       | Why help, errors and completion can be removed without a change |
| The catalogue and methods | Why a feature is a value, and the one method table              |
| The two sources of truth  | The catalogue in the binary, the state on the machine           |
| The compiler              | The one source layer this repository projects                   |

## The layers

```text
src/
  cmd/station/    process entry: reads argv, environment and the three streams once, calls wire
  cmd/generate/   the compiler: projects the catalogue into the toolbelt skill and the footprint page, and the registry into calls.json
  wire/           the composition root: builds adapters and services, resolves every environment
                  variable into a typed option, builds the cobra tree and Fang, maps an error to an exit code
  surface/cli/    the cobra tree, one folder per command word: input -> call -> page
    kit/          how a leaf and a group are declared; how a leaf answers the confirmation its call asks; the wording of every domain error
    words/        the wording of every fail.Code a call raises
  domain/         behaviour, never touching the machine except through ports
    settings/ tools/ config/ setup/ status/ storage/ machines/ images/ secrets/ remote/ tailnet/ shortcuts/ shell/
  engine/         the per-Mac process (the daemon): its runs, its socket, the guards it holds
    process/      the daemon's own calls over its live state, and its station.api/v2 server: those and the registry's
```

The ports and the data:

```text
  system/         the ports: interfaces and value types (Runner, FS, Env, Net, Clock, Host, Engine)
    systemtest/   scripted fakes of every port, for tests
  machines/adapter/
    tart/         the Engine adapter, over Runner
    ssh/          the connection to a VM, over x/crypto/ssh
  platform/
    local/        the real adapters: the only importer of os/exec, files, the network
    darwin/       the only cgo: IOKit power assertions, the lid, power sources, notify(3)
```

The data and its presentation:

```text
  catalog/        pure data: tools and their storage rows, config items, managers, shortcuts, images, the embedded dotfiles and recipes
  page/           the page language: one semantic page (sections, rows, groups, values, marks, reasons, actions), Seal and station.page/v1
  pages/          one page function per page: domain data to a page, pure (tools today)
  ui/             the design system: token, atom, molecule, organism, template, and the renderers tty, pipe, json and tui
  present/        the Document model of the pages not migrated yet, and one presenter per subject
  output/         the only place that knows a terminal: picks tty, pipe or json and renders a page through ui/render, and the one error block
  theme/          the styles of the pages not migrated yet, over ui/token's palette
  prompt/         the conversations on stderr: confirmations and the forms that ask a config item's values
  surface/tui/    the unified Bubble Tea app
```

The shared kernel, which every family will share and which does no I/O:

```text
  core/
    machine/      a machine's Ref, Kind and Profile, the capabilities it reports, the Verdict of every applicability rule
    section/      the sections and section.For(kind), the one table of which kind shows which
    finding/      one Needs you line, and finding.Rank, its one order
    confirm/      the graded confirmations: confirm.For(action, kind) and confirm.Require
    call/         a use case as a Call, Bind (the gate before every call), the Registry, the Router interface
    contract/     the contracts every family shares: Self, Clock, Preferences
    store/        what every store shares: its unreadable and unwritable errors, its version, the Migrator setup runs
    fail/         the error type: its type and code, or today's message, hint and exit
```

The plumbing between machines and processes, which does no I/O of its own:

```text
  remote/         the Router: a call of this machine runs in this process; given this Mac's engine, one that needs its state runs there
    api/          station.api/v2: the frames (call, watch, progress, out, error) and the client of a socket
    serve/        the server of those frames over any connection: the engine's unix socket, never TCP
```

Beside `src/`, `specs/` holds the proofs:

```text
specs/
  architecture/   the test of the layers
  cli/            the end-to-end spec documents
```

A leaf folder `surface/cli/<noun>/<verb>/` holds two files: `input.go` (argv to a typed input) and `command.go` (one call through the router, one page, one answer). A leaf never prints: it answers through `kit.IO`, and the `output` layer decides the form.

Page functions live in `pages/<page>/` (presenters, for the pages not migrated yet, in `present/<subject>/`), not in the leaf folder, because the TUI tab renders the same page: a `tui` to `cli` import would couple the TUI to cobra. How a page looks in each medium is the kit's alone; the design system's rules (the kit's levels, glyphs only in `ui/token`, only pages build pages, no layout outside `ui/`) are rules 13 to 21 of the architecture test, which lists the packages not migrated yet in an allowlist that only shrinks.

## Who may import whom

The import table of today's layers and the list of forbidden symbols are Go literals in [`specs/architecture/layers_test.go`](../../specs/architecture/layers_test.go); the families' import table is one in [`specs/architecture/imports_test.go`](../../specs/architecture/imports_test.go), and a package one table rules the other skips. This page draws the direction and does not restate the rules.

```text
cmd/station ─> wire ─> surface/cli ─> kit ─> domain ─> system <─ platform/local
               │       │                     │         ▲
               │       └─> present <─────────┘         └── machines/adapter/tart
               ├─> output ─> theme
               ├─> prompt ─> theme/forms
               └─> surface/tui ─> present, domain, output
catalog: imported by domain, present, surface, prompt; imports nothing of the module
```

What the test holds, in short: `domain` reaches the machine only through `system` ports; only `platform/local` imports `os/exec`, the network and the process environment; only `output` writes bytes and only `ui/token` holds a colour; only `wire` imports Fang; the catalogue holds values and no function; every domain error type has its wording in `surface/cli/kit`. Each rule has a mutation test, so a rule that stops biting fails the gate.

The tree is moving to families first (machines, apps, settings, ports, awake...), each with its model, `contract/`, `calls/` and `adapter/<tech>/`. Thirteen rules hold that tree: `tree-screams` (`tree_test.go`), `imports` (`imports_test.go`), `technology-words` (`words_test.go`), `no-model-names` (`models_test.go`), `one-way-into-a-vm` (`vm_test.go`: a VM is reached by its engine's one `Guest` connection alone, no `scp`, an `ssh` exec only to a Mac) `one-writer-per-store` (`stores_test.go`: only `wire` builds a store's `jsonfile` adapter, and a store or a preference group is written only by its family's calls) `no-outbound-literals` (`outbound_test.go`: no package a machine's read runs through, the Overview and the adapters of the contracts it reads, spells an internet host, so a read calls no third-party service), and three in `calls_test.go`: `calls-gated` (every exported Call or Watch is in its family's `All`, named `<family>.<verb>` once, today's words allowlisted, and sets its `Applies`), `confirm-in-usecase` (a call naming a confirmation requires it in its `Run`; no surface requires one, answers one before its call asks, or puts a question but kit's `Ask` and `tab.Confirmed`) and `every-code-worded` (every `fail.Code` has its words in `surface/cli/words`, and nothing worded is never raised); `contracts-are-whole` (`contracts_test.go`: every interface of a `contract` package has a real adapter and a fake in its family's `adapter/fake`, and a suite of its `contracttest` that both run), `coverage` (`coverage_test.go`: every call tells a story of each outcome it declares under `specs/usecases/`) and `spec-build-never-ships` (`release_test.go`: the release binary links no world and no fake, and the spec build links every family's fake). `imports` also keeps a surface off the engine model: it reaches the engine through `src/engine/calls`. All but `no-outbound-literals` list today's offenders (`imports` the packages moved into the new tree as they were) in an allowlist that only shrinks: a line that is no longer true fails the gate until it is removed.

`src/world` is the fake world: data only, one slice per family on each of its Macs and VMs, and the closed list of its events. The spec build (`-tags world`, never shipped) is the one place that wires it: under `$STATION_WORLD`, `wire/world_on.go` hands every family's `adapter/fake` over this Mac's slice where the release build's `wire/world_off.go` keeps the real adapter, and steps each Mac's engine in the command. How a use case is told over it is [Testing](../guides/test.md#a-use-case-over-a-world)'s.

`wire` builds this machine's `core/machine.Profile` once: its kind and system from `Self` (`core/adapter/macos` tells a Mac from a macOS VM, `core/adapter/linux`), its battery, lid and thermal from `awake/contract.Power`, and what else it can do from the adapters it wires. A rule reads those capabilities, never a model name: the Awake guards a Mac has are `awake.Active(caps)`, so a desktop Mac has no battery guard because it reports no battery. A machine's disk is `disk/contract.DiskUsage` (`apfs` on macOS, `posix` on Linux).

`station status` is the Overview's read: `overview/calls` composes every section's read through the `core/call/lanes` pool, each row filling as its read answers, and `pages/home` renders the result (`station.status/v1` is its JSON). The facts it shows are `settings/contract.Inspector`'s (the kernel, the network, the protections and updates, by `macos` or `linux`; the owner's git, gpg, GitHub and ssh key, by `cli`), the counters `activity/contract.Meter`'s.

## The path of a request

`station tools install node`:

1. `cmd/station` hands argv and the environment to `wire.Run`.
2. `wire` builds the services over the real adapters and the cobra tree, and runs it under Fang.
3. The leaf `surface/cli/tools/install` binds argv to an `Input`, then validates it against the catalogue and the config (refusals are exit 2, before any question, any read and any effect).
4. The leaf invokes one call, `tools.install` of `src/apps/calls`, through the router `wire` built from every family's `All`: this machine's calls run in this process, past the gate of the target's section table and the call's own `Applies`. The call orders dependencies, runs each method's installer from the `Installers` registry and reads the inventory back.
5. A call that confirms decides it in its own `Run` (`confirm.Require`); when it refuses with `NeedsConfirmation`, kit answers: `--yes`, or the leaf's question on a terminal (its `Ask`), or a refusal, then invokes it once more with the answer. A TUI tab does the same through `tab.Confirmed`: no surface decides whether to ask.
6. The leaf hands the result to its page function, `pages/tools` for `station tools`, which returns a `page.Page`, and to its contract value for `--json`.
7. `output` prints it in the form the stream allows, through `ui/render`: tty with colour, pipe as tab-separated records, json (the contract), or the page itself as `station.page/v1` under the hidden `--page`. The form is decided once per stream; `NO_COLOR` degrades a terminal to the pipe form.

Multi-step behaviour (install then configure, restart then wait) lives in the use case, so a leaf and a TUI action make the same one call.

## The error block

Every failure prints one block on stderr and nothing else writes an error:

```text
Error: unknown tool "nope"
Hint: station tools
```

The exit code comes from the returned error, never from a call to exit inside a command:

| Code | Means                                                                                               |
| ---- | --------------------------------------------------------------------------------------------------- |
| 0    | Done, or a read that found findings (an unreachable machine is a finding)                           |
| 1    | An operation failed, or a confirmation was declined                                                 |
| 2    | A refusal: argv, the catalogue, the config or the place said no, before any question and any effect |
| 130  | Interrupted; no block is printed                                                                    |

A bare group (`station run`) prints its menu on stderr and exits 2, with no block. The wording of every domain error (message and one hint) lives in `surface/cli/kit`'s `failures_<noun>.go` files, and the layer test requires one for every exported error type.

## Fang behind the CLI

Fang renders the help, the version and the completion script, and handles the interrupt. It sits behind the CLI's own error layer: its error handler only calls the CLI's writer, and `wire` is the only importer. Removing Fang is replacing one call in `wire` by cobra's own execute plus the same failure call; no command changes.

`station --version` prints `station version <version> (<commit>)`. The version is stamped by the linker from the tag, in the Makefile and the publish workflow ([ADR-005](../decisions/005-the-version-is-stamped-from-the-tag.md)).

## The catalogue and methods

The catalogue is values in `src/catalog/`: tools (agent skills included), config items, managers, shortcuts, images, and each tool's storage row. A catalogue value carries no function, and a test says so, because behaviour chosen by data belongs to the domain, chosen by the value's type from a closed union.

The rule this repository is built on: **add to the catalogue, not to command code**. One tool entry gives the same tool a row in `station tools`, a line in `station status`, an install, an uninstall and an upgrade path, because all of them read the same value. Which file a change opens is [Develop](../guides/develop.md)'s table.

How a tool installs is one registry, `Installers` in `src/apps/contract/`: one adapter per method under `src/apps/adapter/<method>/`, with its install, uninstall, update and inventory commands, and what it can do (its abilities, whether it keeps a list, whether its installs ride the bundle). No other code switches on a method; the machine kinds that accept a method are the catalogue's. `brew bundle` installs and upgrades every method it covers; uninstall is each method's native command ([ADR-002](../decisions/002-brew-bundle-installs-native-commands-uninstall.md)). The model is [Tools and skills](../reference/tools-and-skills.md)'s.

Every catalogue entry carries its place, the kinds of machine it may live on, so one binary on a host, a Linux VM and a macOS VM lists and installs only what fits there ([Glossary](glossary.md)).

## The two sources of truth

The binary holds the catalogue: what station knows how to install and configure. The machine holds its own state, in the XDG folders: `~/.config/station/settings.json` (this machine's name, the hosts, the tailnet choice, the lend list), the VM records and image records under `~/.local/state/station/`. Nothing else is consulted for that, and no path inside the clone matters: the dotfiles and the VM recipes are embedded in the binary, so a deleted or moved clone breaks nothing. [Footprint](../reference/footprint.md) lists every file.

## The compiler

This repository is an application in the sense of the [repo-structure doctrine](repo-structure.md): it authors a corpus, routes to it from `CLAUDE.md` and `skills/`, and generates no API reference. Its one source layer worth projecting is the tool catalogue, and `make generate` (`src/cmd/generate/`) renders it between the `GENERATED` markers of `skills/studio-toolbelt/SKILL.md`. The projection is committed and a test fails when it drifts ([Tools and skills](../reference/tools-and-skills.md)).

## Related

- [Design system](design-system.md): the palette, the marks and the documents the output layer prints.
- [Develop](../guides/develop.md): the gestures and the table of which file a change opens.
- [Test](../guides/test.md): the suites that prove the layers and the behaviour.
- [ADR-001](../decisions/001-station-mirrors-the-studio-cli-layers.md): why the layers mirror the `studio` CLI, and where they deliberately differ.
