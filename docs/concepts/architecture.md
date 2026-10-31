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
  cmd/station/    process entry: reads argv, environment and the three streams once, calls app
  cmd/skillsgen/  the compiler: projects the catalogue into the toolbelt skill
  app/            the composition root: builds adapters and services, resolves every environment
                  variable into a typed option, builds the cobra tree and Fang, maps an error to an exit code
  cli/            the cobra tree, one folder per command word: input -> command -> presenter
    kit/          how a leaf and a group are declared; the confirm gate; the wording of every domain error
  domain/         behaviour, never touching the machine except through ports
    settings/ tools/ config/ setup/ status/ clean/ machines/ images/ secrets/ remote/ tailnet/ shortcuts/ shell/
```

The ports and the data:

```text
  system/         the ports: interfaces and value types (Runner, FS, Env, Net, Clock, Host, Engine)
    tart/         the Engine adapter, over Runner
    host/         the real adapters: the only importer of os/exec, files, the network
    systemtest/   scripted fakes of every port, for tests
  catalog/        pure data: tools, config items, cleanables, managers, shortcuts, images, the embedded dotfiles and recipes
  present/        the Document model (blocks, tones, marks) and one presenter per subject
  output/         the only place that knows a terminal: tty, pipe or json, and the one error block
  theme/          the one palette
  prompt/         the conversations on stderr: confirmations and the forms that ask a config item's values
  fail/           the error type: a message, a hint and a code
  tui/            the unified Bubble Tea app
```

Beside `src/`, `specs/` holds the proofs:

```text
specs/
  architecture/   the test of the layers
  cli/            the end-to-end spec documents
```

A leaf folder `cli/<noun>/<verb>/` holds two files: `input.go` (argv to a typed input) and `command.go` (one domain call, one presenter, one answer). A leaf never prints: it answers through `kit.IO`, and the `output` layer decides the form.

Presenters live in `present/<subject>/`, not in the leaf folder, because the TUI renders the same ones: a `tui` to `cli` import would couple the TUI to cobra.

## Who may import whom

The import table and the list of forbidden symbols are Go literals in [`specs/architecture/layers_test.go`](../../specs/architecture/layers_test.go), and nowhere else; this page draws the direction and does not restate the rules.

```text
cmd/station ─> app ─> cli ─> cli/kit ─> domain ─> system <─ system/host
                │       │                 │          ▲
                │       └─> present <─────┘          └── system/tart
                ├─> output ─> theme
                ├─> prompt ─> theme/forms
                └─> tui ─> present, domain, output
catalog: imported by domain, present, cli, prompt; imports nothing of the module
```

What the test holds, in short: `domain` reaches the machine only through `system` ports; only `system/host` imports `os/exec`, the network and the process environment; only `output` writes bytes and only `theme` holds a colour; only `app` imports Fang; the catalogue holds values and no function; every domain error type has its wording in `cli/kit`. Each rule has a mutation test, so a rule that stops biting fails the gate.

## The path of a request

`station tools install node`:

1. `cmd/station` hands argv and the environment to `app.Run`.
2. `app` builds the services over the real adapters and the cobra tree, and runs it under Fang.
3. The leaf `cli/tools/install` binds argv to an `Input`, then validates it against the catalogue and the config (refusals are exit 2, before any question, any read and any effect).
4. The confirm gate runs for a destructive leaf: `--yes`, or a question on a terminal, or a refusal.
5. The leaf makes one call to the domain service `domain/tools`, which orders dependencies, runs the method table's commands through the `Runner` port and reads the inventory back.
6. The leaf hands the result to `present/tools`, which returns an answer: a `Document` for a person and a JSON value for `--json`.
7. `output` prints it in the form the stream allows: tty with colour, pipe as tab-separated records, or json. The form is decided once per stream; `NO_COLOR` degrades a terminal to the pipe form.

Multi-step behaviour (install then configure, restart then wait) lives in the domain operation, so a leaf and a TUI action make the same one call.

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

A bare group (`station run`) prints its menu on stderr and exits 2, with no block. The wording of every domain error (message and one hint) lives in `cli/kit`'s `failures_<noun>.go` files, and the layer test requires one for every exported error type.

## Fang behind the CLI

Fang renders the help, the version and the completion script, and handles the interrupt. It sits behind the CLI's own error layer: its error handler only calls the CLI's writer, and `app` is the only importer. Removing Fang is replacing one call in `app` by cobra's own execute plus the same failure call; no command changes.

`station --version` prints `station version <version> (<commit>)`. The version is stamped by the linker from the tag, in the Makefile and the publish workflow ([ADR-005](../decisions/005-the-version-is-stamped-from-the-tag.md)).

## The catalogue and methods

The catalogue is values in `src/catalog/`: tools (agent skills included), config items, cleanables, managers, shortcuts, images. A catalogue value carries no function, and a test says so, because behaviour chosen by data belongs to the domain, chosen by the value's type from a closed union.

The rule this repository is built on: **add to the catalogue, not to command code**. One tool entry gives the same tool a row in `station tools`, a line in `station status`, an install, an uninstall and an upgrade path, because all of them read the same value. Which file a change opens is [Develop](../guides/develop.md)'s table.

How a tool installs is one table, the method table in `src/domain/tools/methods.go`: one row per method with its install, uninstall, upgrade and inventory commands, and the machine kinds that accept it. No other file switches on a method. `brew bundle` installs and upgrades every method it covers; uninstall is each method's native command ([ADR-002](../decisions/002-brew-bundle-installs-native-commands-uninstall.md)). The model is [Tools and skills](../reference/tools-and-skills.md)'s.

Every catalogue entry carries its place, the kinds of machine it may live on, so one binary on a host, a Linux VM and a macOS VM lists and installs only what fits there ([Glossary](glossary.md)).

## The two sources of truth

The binary holds the catalogue: what station knows how to install and configure. The machine holds its own state, in the XDG folders: `~/.config/station/settings.json` (this machine's name, the hosts, the tailnet choice, the lend list), the VM records and image records under `~/.local/state/station/`. Nothing else is consulted for that, and no path inside the clone matters: the dotfiles and the VM recipes are embedded in the binary, so a deleted or moved clone breaks nothing. [Footprint](../reference/footprint.md) lists every file.

## The compiler

This repository is an application in the sense of the [repo-structure doctrine](repo-structure.md): it authors a corpus, routes to it from `CLAUDE.md` and `skills/`, and generates no API reference. Its one source layer worth projecting is the tool catalogue, and `make skills` (`src/cmd/skillsgen/`) renders it between the `GENERATED` markers of `skills/studio-toolbelt/SKILL.md`. The projection is committed and a test fails when it drifts ([Tools and skills](../reference/tools-and-skills.md)).

## Related

- [Design system](design-system.md): the palette, the marks and the documents the output layer prints.
- [Develop](../guides/develop.md): the gestures and the table of which file a change opens.
- [Test](../guides/test.md): the suites that prove the layers and the behaviour.
- [ADR-001](../decisions/001-station-mirrors-the-studio-cli-layers.md): why the layers mirror the `studio` CLI, and where they deliberately differ.
