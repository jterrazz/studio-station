# Agent brief: `station` (studio-station)

A Go CLI that sets up and runs a Mac and its fleet of VMs: tools, config, hosts, images. This file **routes**; it does not restate what the corpus already says.

## Mental model

- **Layers.** input, command, domain, pure presenter, one output layer. A test refuses an import that crosses them ([`docs/concepts/architecture.md`](docs/concepts/architecture.md)).
- **Add to the catalogue, not to command code.** Tools (agent skills included, each with its storage row), config items (the Git and Docker shortcuts among them) and images are values in `src/catalog/`.
- **The binary is hermetic.** Dotfiles and VM recipes are embedded; nothing reads a path inside the clone.
- **The words are the glossary's.** One word per concept ([`docs/concepts/glossary.md`](docs/concepts/glossary.md)).
- **Specs drive the real binary.** Every behaviour has a `<case>.spec.yaml` document under `specs/cli/`; module tests exist only where a spec cannot reach.

## Where knowledge lives (route here first)

The corpus is `docs/` + `README.md`, mapped by `docs/README.md`. Do not duplicate it; link to it.

| Working on                                      | Read                                |
| ----------------------------------------------- | ----------------------------------- |
| A fresh machine: install, setup, configure      | `docs/start/getting-started.md`     |
| The layers, the method table                    | `docs/concepts/architecture.md`     |
| Palette, marks, pipe format, the TUI            | `docs/concepts/design-system.md`    |
| What a word means                               | `docs/concepts/glossary.md`         |
| Building, and which folder a change opens       | `docs/guides/develop.md`            |
| The suites, the spec documents                  | `docs/guides/test.md`               |
| The tagged release, `make provision`            | `docs/guides/release.md`            |
| Every command and flag                          | `docs/reference/commands.md`        |
| Config items, `preferences.json`, env variables | `docs/reference/configuration.md`   |
| Tools, methods, skills, the toolbelt            | `docs/reference/apps-and-skills.md` |
| Hosts, VMs, lifetimes, `--on`, the engine       | `docs/reference/machines.md`        |
| Services, leases, HTTPS, profiles               | `docs/reference/ports.md`           |
| VM images and their recipe                      | `docs/reference/images.md`          |
| Kept, lent and bound secrets                    | `docs/reference/secrets.md`         |
| The `station.status/v1` contract                | `docs/reference/status-json.md`     |
| The shell block, the embedded dotfiles          | `docs/reference/dotfiles.md`        |
| What station writes on a machine                | `docs/reference/footprint.md`       |
| Why a choice was made                           | `docs/decisions/`                   |
| What is left for later                          | `TODO.md`                           |
| `@jterrazz` stack conventions                   | `docs/concepts/stack.md`            |
| Repo doctrine (corpus/injection/compiler)       | `docs/concepts/repo-structure.md`   |

The repo-structure doctrine is authored **here** and ships to agents as the `studio-repo-structure` skill. This repo is an application: it adopts the corpus + injection layers and generates no API reference. Its one compiler is `make generate` (`src/cmd/generate/`), which projects the catalogue into the `studio-toolbelt` skill and `docs/reference/footprint.md`, and writes the page fixtures of `src/ui/testdata/pages/` from the real page functions.

## Setup & commands

```bash
make install    # Go modules + the npm half
make build      # Build .artifacts/go/station
make check      # lint (gofmt, golangci-lint, deadcode, typescript check), test, build-linux, then menubar-check (apps/menubar: swift test, the bundle)
make fix        # gofmt -w, then typescript fix
make provision  # put this build on the machine as the real station, then station setup
```

`make install` never touches the machine; `make provision` does.

## Standing rules

- **Add to the catalogue, not to command code.** Which folder a change opens is the table in `docs/guides/develop.md`.
- **Four things land in the same commit as the change that makes them true**: the `make generate` projection, the spec document that proves the behaviour, the page the behaviour falsified, and the skill a doctrine page feeds. Each is stated once, in `docs/guides/develop.md` § What a change owes.
- **Never edit a generated section by hand**: the blocks between the `GENERATED` markers of `skills/studio-toolbelt/SKILL.md` come from `make generate`, and `docs/reference/footprint.md` is generated whole.
