# Developing

How to work on `station` itself: build it, run it from the checkout, and know which file a change belongs in.

| Section             | Answers                                     |
| ------------------- | ------------------------------------------- |
| Working from source | The clone, Go 1.26+, and the loop           |
| The make targets    | Every gesture the Makefile offers           |
| Where a change goes | Which file to open for which kind of change |
| What a change owes  | What must land in the same commit           |

## Working from source

```sh
git clone https://github.com/jterrazz/studio-station.git
cd studio-station
make install              # Go modules and the npm half
make build                # .artifacts/go/station
./.artifacts/go/station overview  # run the build without installing it
```

The binary reads nothing from the clone: the dotfiles and the VM recipes are embedded, so any checkout is a checkout to build. Go 1.27+ and npm are the two requirements; `make install` touches nothing outside the checkout. `make provision` puts the build on the machine as the real `station` ([Release](release.md)).

## The make targets

| Target               | Does                                                                                                                                                                                              |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `make install`       | Fetch the Go modules and install the npm half (`npm ci`)                                                                                                                                          |
| `make build`         | Build `.artifacts/go/station`, its version stamped from `git describe`                                                                                                                            |
| `make build-linux`   | Build `.artifacts/go/station-linux-arm64`, the binary a VM image carries                                                                                                                          |
| `make lint`          | Both halves: `gofmt -l`, `go tool golangci-lint` and `go tool deadcode` over Go, then `typescript check`                                                                                          |
| `make fix`           | Every fixer: `gofmt -w ./src ./specs`, then `typescript fix`                                                                                                                                      |
| `make test`          | `go test ./src/... ./specs/...`, then `typescript test`: the `specs/cli` suite against a fresh build                                                                                              |
| `make check`         | `lint`, `test`, `build-linux`, then `menubar-check`: the gate a change passes                                                                                                                     |
| `make menubar-check` | `swift test` over `apps/menubar` (the page fixtures drawn offscreen against PNG goldens), then its bundle; one line of skip without a macOS SDK, a failure then under `STATION_REQUIRE_MENUBAR=1` |
| `make menubar`       | Build `.artifacts/menubar/Station.app`, signed ad hoc; `make menubar-install` copies it to `~/Applications`, `make menubar-uninstall` unregisters its login item and removes it                   |
| `make generate`      | Regenerate the catalogue's projections (the toolbelt skill's rosters, `docs/reference/footprint.md`) and the page fixtures                                                                        |
| `make provision`     | Build, put `station` in `~/.local/bin`, then run `station setup`                                                                                                                                  |
| `make deprovision`   | Remove the provisioned binary                                                                                                                                                                     |
| `make clean`         | Remove `.artifacts/`                                                                                                                                                                              |

`install`, `build`, `lint`, `fix`, `test` and `check` are the gestures every repository of the estate answers, so a session that never opened this one installs, proves and repairs it the same way; `make install` touches nothing outside the checkout, and putting `station` on the machine is `make provision`'s alone. `make check` is the whole gate, and it fails on the first finding of either half. The Go half runs `golangci-lint` and `deadcode` with `go tool`, at the versions the `tool` directive of `go.mod` declares, so the Go that builds the binary builds its linters and a green run is never a fact about the day it ran. Which linters judge the Go sources, and the four exclusions that carry a reason, are `.golangci.yml`. The TypeScript half is `typescript check` from `@jterrazz/typescript`, whose passes are the toolchain's own [Quality checks](https://github.com/jterrazz/package-typescript/blob/main/docs/reference/quality-checks.md).

## Where a change goes

Most changes are a catalogue value, not new code ([Architecture](../concepts/architecture.md)). The folder to open:

| Change                                          | Folder                                                                                                                                                                                                                                                  |
| ----------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A tool or an agent skill `station tools` offers | `src/catalog/tools.go`, `src/catalog/skills.go`                                                                                                                                                                                                         |
| A config item                                   | `src/catalog/config.go`, its procedure in `src/domain/config/`                                                                                                                                                                                          |
| What a tool keeps on disk, how it is cleaned    | its row in `src/catalog/storage.go`                                                                                                                                                                                                                     |
| A manager `--via` knows                         | `src/catalog/managers.go`                                                                                                                                                                                                                               |
| A `station run` shortcut                        | `src/catalog/shortcuts.go`                                                                                                                                                                                                                              |
| How a method installs                           | its adapter `src/apps/adapter/<method>/`, in the `Installers` registry                                                                                                                                                                                  |
| A VM image's recipe                             | `src/catalog/images/`                                                                                                                                                                                                                                   |
| An application's dotfile                        | `src/catalog/dotfiles/`                                                                                                                                                                                                                                 |
| A new verb                                      | a leaf folder `src/surface/cli/<noun>/<verb>/`, its call in `src/<family>/calls/`, its page in `src/pages/`, its file in `src/wire/`                                                                                                                    |
| What a document shows                           | `src/present/<subject>/`                                                                                                                                                                                                                                |
| What the TUI shows                              | `src/surface/tui/screens/`                                                                                                                                                                                                                              |
| What the menu bar shows                         | the `menubar` page, `src/pages/menubar/`; how it is drawn, the Swift kit in `apps/menubar/sources/station-kit/`; its goldens are rewritten by `STATION_UPDATE_GOLDENS=1 swift test --package-path apps/menubar --scratch-path .artifacts/menubar/build` |
| The wording of a domain error                   | `src/surface/cli/kit/failures_<noun>.go`                                                                                                                                                                                                                |

Dependencies flow one way; `specs/architecture/layers_test.go` refuses a change that crosses a layer. The linters run with `go tool`, so `make lint` needs no installed tool.

## What a change owes

Four things land in the same commit as the change that makes them true:

- **A catalogue change regenerates the projection.** Run `make generate`; the sync test in `make test` fails otherwise, and the sections between the `GENERATED` markers are never edited by hand ([Tools and skills](../reference/tools-and-skills.md)).
- **A change to behaviour carries its spec document.** A new command, flag, refusal or `--json` key comes with the `.spec.yaml` that proves it end to end; `TEST_UPDATE=1 npm test` rewrites only the `exit:` and streams of existing documents ([Test](test.md)).
- **A change to behaviour updates its page.** The corpus is where this repository's knowledge is authored; a page left saying what is no longer true is a bug that shipped.
- **A comment states its kind, in both halves.** The default is none; a comment that stays opens with `invariant:`, `hazard:`, `outside:` or `value:` ([A comment states its kind](https://github.com/jterrazz/package-typescript/blob/main/docs/reference/lint-presets.md#a-comment-states-its-kind)). The Go half holds the same law in Go's shapes: a package comment (staticcheck's ST1000 asks for it), and one line of doc above an exported identifier or member, its hover. A kinded comment about an exported function goes inside its body, since ST1020 wants that function's doc to open with its name. `typescript check` reads the configs and scripts, not `.go` files, so on the Go side the review is the gate.
- **A change to a doctrine page updates its skill.** [The stack](../concepts/stack.md) ships as `studio-stack` and [Repo structure](../concepts/repo-structure.md) as `studio-repo-structure`; skills route, they never author.

## Related

- [Architecture](../concepts/architecture.md): the shape a change lands in.
- [Test](test.md): the two suites, and the document form of a spec.
- [Release](release.md): installing, and the tagged release.
