# Developing

How to work on `workstation` itself: build it, run it from the checkout, and know which file a change belongs in.

| Section             | Answers                                     |
| ------------------- | ------------------------------------------- |
| Working from source | The clone, Go 1.26+, and the loop           |
| The make targets    | Every gesture the Makefile offers           |
| Where a change goes | Which file to open for which kind of change |
| What a change owes  | What must land in the same commit           |

## Working from source

```sh
git clone https://github.com/jterrazz/studio-workstation.git ~/Developer/studio-workstation
cd ~/Developer/studio-workstation
make build                # .artifacts/go/workstation
./.artifacts/go/workstation status  # run the build without installing it
```

The path is the one the install script clones to, because the machine serves one clone: the `~/.zshrc` line sources its `zshrc.sh`, and `workstation` reads its dotfiles and scripts from it unless `WORKSTATION_REPO_PATH` names another ([Footprint](../reference/footprint.md)). A second clone elsewhere is a checkout to build, not the one the shell runs.

Go 1.26+ and npm are the two requirements: `make install` fetches the Go modules and the npm half the specs and the toolchain run on ([Test](test.md)). `make provision` puts the build on the machine as the real `workstation`; that gesture, and everything else about a machine that has `workstation` on it, is [Release](release.md)'s.

## The make targets

| Target             | Does                                                                                                     |
| ------------------ | -------------------------------------------------------------------------------------------------------- |
| `make install`     | Fetch the Go modules and install the npm half (`npm ci`)                                                 |
| `make build`       | Build `.artifacts/go/station`, its version stamped from `git describe`                                   |
| `make build-linux` | Build `.artifacts/go/station-linux-arm64`, the binary a VM image carries                                 |
| `make lint`        | Both halves: `gofmt -l`, `go tool golangci-lint` and `go tool deadcode` over Go, then `typescript check` |
| `make fix`         | Every fixer: `gofmt -w ./src ./specs`, then `typescript fix`                                             |
| `make test`        | `go test ./src/... ./specs/...`, then `typescript test`: the `specs/cli` suite against a fresh build     |
| `make check`       | `lint`, `test`, then `build-linux`: the gate a change passes                                             |
| `make skills`      | Regenerate the toolbelt skill's rosters from the catalogue                                               |
| `make provision`   | Build, put `station` in `~/.local/bin`, then run `station setup`                                         |
| `make deprovision` | Remove the provisioned binary                                                                            |
| `make clean`       | Remove `.artifacts/`                                                                                     |

`install`, `build`, `lint`, `fix`, `test` and `check` are the gestures every repository of the estate answers, so a session that never opened this one installs, proves and repairs it the same way; `make install` touches nothing outside the checkout, and putting `workstation` on the machine is `make provision`'s alone. `make check` is the whole gate, and it fails on the first finding of either half. The Go half runs `golangci-lint` and `deadcode` with `go tool`, at the versions the `tool` directive of `go.mod` declares, so the Go that builds the binary builds its linters and a green run is never a fact about the day it ran. Which linters judge the Go sources, and the four exclusions that carry a reason, are `.golangci.yml`. The TypeScript half is `typescript check` from `@jterrazz/typescript`, whose passes are the toolchain's own [Quality checks](https://github.com/jterrazz/package-typescript/blob/main/docs/reference/quality-checks.md).

## Where a change goes

Most changes are a registry entry, not new code: that is the shape [Architecture](../concepts/architecture.md) describes. The file to open:

| Change                                        | File                                                                              |
| --------------------------------------------- | --------------------------------------------------------------------------------- |
| A tool `workstation install` offers           | `src/internal/config/tools_catalog.go`                                            |
| A `workstation config` item                   | `src/internal/config/scripts.go` (server items: `server_*.go` beside the command) |
| A `workstation run` shortcut                  | `src/internal/config/commands.go`                                                 |
| Something `workstation clean` reclaims        | `src/internal/config/cleanables.go`                                               |
| A package manager `workstation upgrade` knows | `src/internal/config/upgraders.go`                                                |
| A curated agent skill                         | `src/internal/config/skills.go`                                                   |
| A new verb                                    | A file under `src/internal/commands/`, registering itself in `init()`             |
| What a TUI shows                              | `src/internal/presentation/views/<view>/`                                         |
| A shell shortcut or an app's dotfile          | `dotfiles/applications/`                                                          |

## What a change owes

Five things land in the same commit as the change that makes them true:

- **A registry change regenerates the projection.** Run `make skills`; the sync test in `make test` fails otherwise, and the sections between the `GENERATED` markers are never edited by hand ([Tools and skills](../reference/tools-and-skills.md)).
- **A change to a command's output regenerates the specs.** `TEST_UPDATE=1 npm test` rewrites the `exit:` and the streams of the affected documents, and nothing else ([Test](test.md)).
- **A change to behaviour updates its page.** The corpus is where this repository's knowledge is authored; a page left saying what is no longer true is a bug that shipped.
- **A comment states its kind, in both halves.** The default is none; a comment that stays opens with `invariant:`, `hazard:`, `outside:` or `value:` ([A comment states its kind](https://github.com/jterrazz/package-typescript/blob/main/docs/reference/lint-presets.md#a-comment-states-its-kind)). The Go half holds the same law in Go's shapes: a package comment (staticcheck's ST1000 asks for it), and one line of doc above an exported identifier or member, its hover. A kinded comment about an exported function goes inside its body, since ST1020 wants that function's doc to open with its name. `typescript check` reads the configs and scripts, not `.go` files, so on the Go side the review is the gate.
- **A change to a doctrine page updates its skill.** [The stack](../concepts/stack.md) ships as `studio-stack` and [Repo structure](../concepts/repo-structure.md) as `studio-repo-structure`; skills route, they never author.

## Related

- [Architecture](../concepts/architecture.md): the shape a change lands in.
- [Test](test.md): the two suites, and the document form of a spec.
- [Release](release.md): installing, and the tagged release.
