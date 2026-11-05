# Testing

Two suites prove a change: Go tests (the module tests and the boundary test of the layers), and end-to-end specs that drive the real `station` binary and state what it printed.

| Section                  | Answers                                               |
| ------------------------ | ----------------------------------------------------- |
| The two suites           | What `make test` runs, and in which order             |
| A scenario is a document | The `<case>.spec.yaml` form and how it is regenerated |
| The sandbox              | Why no spec reaches the real machine                  |
| A use case over a world  | `specs/usecases/`, the spec build and its fake world  |
| The stubs                | How a spec scripts a system tool                      |
| What CI runs             | The gate a pull request passes                        |

## The two suites

```sh
make test      # go test ./src/... ./specs/..., then typescript test: the specs against a fresh build of station, then the machines and engine/keep use cases again over vz
npm test       # the specs alone
```

The Go tests come first. The module tests are `*_test.go` files next to the code they judge, kept only for what no session of the binary reaches honestly: a state the terminal driver cannot observe (the TUI loading in `Init`, a busy screen, the renderer's colours), a timing budget, root, a pure algorithm whose edge cases are cheaper at its own level (the renderer goldens: every page fixture of `src/ui/testdata/pages/`, written by `make generate` from the real page functions, rendered by every renderer of `src/ui/render/` and compared with its file in `src/ui/testdata/__expected__/`; `go test ./src/ui/... -update` rewrites them). The boundary test, `specs/architecture/`, proves each of its rules still bites by adding one violating file in memory: `layers_test.go` owns the import table and the forbidden symbols of today's layers, `imports_test.go` the import table of the families.

Then `make test` runs the CLI specs under `specs/cli/` on Vitest, through `@jterrazz/test`. There is exactly one runner, and every spec drives one binary, `.artifacts/go/station-test`, stamped with the version `git describe` prints: the spec build (`-tags world`), which runs the real adapters unless a document hands it a world ([below](#a-use-case-over-a-world)). `make test` builds it once before the runner starts, so no worker links it while another drives a session (with one build per worker, nine links ran beside the first sessions). Run directly, `specs/cli/cli.specification.ts` rebuilds it when it is missing or when any file under `src/` is newer than it, beside the binary and renamed into place, so a worker never executes a half-written file. It is an mtime check, so editing the CLI and re-running the suite always exercises the change instead of a stale build.

`vitest.config.ts` is the one line `@jterrazz/test` derives from `studio.config.ts`: its `cli` project collects `specs/cli/`, `specs/adapters/` and `specs/usecases/`, and every `*.spec.yaml` there becomes a one-test module driving that runner. A document of `specs/adapters/<tech>/` proves one adapter speaks its tool (its argv, its parsing, against the stub CLIs a fixture carries): `.calls` assertions belong there, and nowhere else once a family has moved. The runner's defaults are [the sandbox](#the-sandbox).

## A scenario is a document

Almost every spec here is a terminal session (a command, its exit code, what it printed), so it is written as one: a `<case>.spec.yaml` in the command's folder, in the literate format of `@jterrazz/test`. The file IS the test; `description:` is its title in the runner, and each entry of `runs:` states the command, its `exit:` and its `stdout:`/`stderr:` byte-exact.

```yaml
# specs/cli/root/unknown-command.spec.yaml
description: an unknown command is one error block on stderr, without colour, and exits 2
runs:
    - command: nope
      exit: 2
      stderr: |
          Error: unknown command "nope" for "station"
          Hint: run station --help
```

A document may stand on ground of its own: `fixture: ./__fixtures__/<case>/` spreads that folder into the working directory, which is the run's `HOME`, so a stub tool under its `bin/` answers the binary. Ground sits in a special folder spelt with double underscores; a subject folder never starts with one. Where a stream is long and only some of it is the claim, the command writes it to a file (`help > help.txt`) and `files:` asserts what it `contains`.

`TEST_UPDATE=1 npm test` rewrites the `exit:` and the streams of every document from what the binary actually printed: deliberately, after a change to a command's output, and never as a way to make a red suite go green. Nothing else in the file is touched.

## The sandbox

No spec reaches the real machine. The runner sets `HOME` to the run's working directory, `PATH` to its `bin/` alone (the fixture's stubs), and `STATION_SYSTEM_DIRS` to nothing, so a system tool no fixture stubs is "not found" alike on a Mac and on the Linux CI runner, and `STATION_SYSTEM_ROOT` to the run's `root/`, so `/Applications`, `/opt/homebrew` and `/Library` are read there and the owner's own apps are neither listed nor sized. `STATION_POWER_STUB` points Power at the run's `power/` folder (the `power` fixture writes one): its files are the reading, every assertion and clamshell call is a line of `.calls`, and with no such folder the run has no Power at all, so no spec takes a real assertion. `STATION_KERNEL_STUB` points a Mac's counters and process tables at the run's `kernel/` folder, recorded kernel tables (`counters.json`, and `procs.json`, the process tables successive reads walk through), and with no such folder the kernel answers nothing, so no spec reads the real machine's activity. It also pins `NO_COLOR=1`, `COLUMNS=80`, `TZ=UTC` and `STATION_MACHINE_KIND=host`, so every stream is the pipe form and a document means the same thing on every host.

## A use case over a world

A document of `specs/usecases/<family>/<verb>/<outcome>.spec.yaml` tells one outcome of one call (`machines/stop` is `machines.stop`; `engine/<job>` is an engine job; `root/`, `remote/`, `output/`, `errors/`, `migrate/` and `security/` hold what spans calls) against a seeded fake world, and asserts what the owner reads, never an argv. It names its world as an env set, `env: two-macs`: the runner registers one per file of `specs/usecases/worlds/`, which points `$STATION_WORLD` at `world.json` in the run's directory and seeds it from that file at the first run. Every run then reads and writes that one world, so a document can say what the world became by reading it back through the CLI.

The world (`src/world`) holds what is outside station, one slice per family: each Mac's engine (its job, when it started and when the owner stopped it), preferences, the files a store has not read yet (a hand edit, a file station v2 left) and station's other stores, its Self, its power reading (a battery, a lid), what listens on it, its tailnet node, its installers, meter, disk, jobs, vault, facts and ssh config, its images and VMs (what station keeps of a VM, beside its disk, its run and when it began and ended on the world's clock, its guest and what listens in it), and the faults scripted for the fakes. Under `$STATION_WORLD` the spec build wires every family's `adapter/fake` over its Mac's slice in place of the real adapters (`src/wire/world_on.go`, the one file that knows the tag; `release_test.go` fails when a family's fake is not linked), runs each Mac's engine inside the command, stepped over the world's clock until it is quiet (the clock moves only for the command that waits, a tick and an advance: what the engine runs in its own lifetime waits for it), and reaches another Mac of the world in-process, through the wire form. A world event is a run of the hidden `world` command, `command: 'world reboot jterrazz-mac-mini'`; its closed list is USE-CASES.md §0.2; an event no fake models yet is refused as such. `world edit <machine> <file>` gives a file of station the text on its stdin, as a hand edit would. `world fault Hypervisor.Boot [2 times]` fails that contract's method the next calls (every call, without a count) in the fakes that obey it: `Hypervisor.Boot`, `Installer.<verb>`, `Bundler.Bundle`, `Jobs.Install`, `Jobs.Start`, `Tailnet.Share`, `Unlocker.PlannedRestart`, `CheckRunner.Run`, a server's check that stops answering, and `EndpointDialer.Dial`, a Mac whose ssh answers with no station there.

A second hypervisor must pass the VM use cases unchanged (AR-02), so `make test` runs `specs/usecases/machines/` and `specs/usecases/engine/keep/` a second time with `STATION_SPEC_HYPERVISOR=vz`: the runner then sets `$STATION_WORLD_HYPERVISOR` in every world, and each Mac that had station's own hypervisor runs the fake `vz` instead (a Mac whose world says `"hypervisor": "none"` keeps none). The env set `two-macs-vz` is two-macs over vz for a document that names it.

`specs/architecture/coverage_test.go` reads the registry and walks `specs/usecases/`: every call needs a story of each outcome it declares (done, each code it may end in, `not-for-this-machine-<machine>` for each reference machine its rule refuses, `on-another-machine` when it is routable), and a document names only a real call and outcome. The calls not covered yet are its allowlist, which only shrinks. `release_test.go` proves the release binary links no world and no fake.

## The TUI on a terminal

A run is never given a terminal, so a document that needs one names the `terminal` env set and runs `--version >/dev/null; terminal "$STATION" [words]`. The `terminal` stub of `specs/__fixtures__/terminal/`, which any noun's document lists in its `fixture:`, opens a pseudo-terminal (80x24, or `--size`), replays what the binary draws on a screen of cells, and plays the steps the document writes on `stdin:`: `see` and `gone` wait for a text on the screen (10 s; `within SECONDS see TEXT` bounds the wait, so a document can say a row appeared before a slow probe ended), `file PATH` waits for a stub to have written PATH, so a step can follow a probe's start, `send` types keys (`\r`, `\x1b[B`), `screen` prints the screen, and `scrollback` waits for the binary to exit and prints the history above the screen with the screen, as tmux keeps it (a clear from the screen's first cell joins the history), so a document can count the copies a live read leaves. It then exits with the binary's code, and a wait that times out, or a `see`/`gone` the binary exits before, prints the screen, the bytes the binary drew and the seconds it ran on stderr and exits 124, so a binary that never started reads apart from a slow first frame. The size is set before the binary starts, so its first frame never races the driver for it. The `terminal-*.spec.yaml` documents of `specs/cli/root/` are the TUI sessions; `setup`, the `config` items that ask values, the confirm gate of a destructive verb and `machines unlock` drive the binary the same way.

The full grammar is `@jterrazz/test`'s [Spec documents](https://github.com/jterrazz/package-test/blob/main/docs/guides/spec-documents.md) and [CLI specs](https://github.com/jterrazz/package-test/blob/main/docs/guides/cli-specs.md).

## The stubs

A spec copies one `fixture:` directory into its working directory, so stubs live beside the command word that uses them: `specs/cli/<noun>/__fixtures__/<case>/bin/<tool>`. A fixture both suites read (the VMs `vms/`, the running VM `engine-vm/`, what Tailscale serves `serving/`, the two Tailscale apps) lives in `specs/__fixtures__/`, the lowest folder `specs/cli/` and `specs/adapters/` share. A stub answers by argv from a table, appends `<tool> <argv>` (and the stdin of `brew bundle --file=-`) to `$HOME/.calls`, and exits 97 on anything unscripted. A document asserts `.calls` through `files:` to prove which installer line ran. The GitHub API is a stub backend the runner serves, reached through `STATION_GITHUB_API_URL`; a VM's sshd is the stub `vm-sshd` (`specs/cli/__fixtures__/vm-sshd/`), served to the documents that declare `serve: sshd` and reached through `STATION_GUEST_SSH_PORT`.

## What a test is for

A test exists because a real regression would turn it red. Every behaviour of a command (each flag, refusal, exit code, `--json` schema and migration) has a spec document through the real binary; the architecture test proves the layers; a module test exists only where the logic is dense and a spec cannot reach it honestly (what the terminal driver cannot observe, a timing budget, root, a pure algorithm). A test that restates a declaration, a table or a mock's own return is deleted.

## The machine specs

Tiers 2 and 3 run the real binary on a real VM of this host, where the sandbox cannot reach: `make e2e-machine KIND=linux` (tier 2, a clone of the product `linux` image, 4 GB) and `make e2e-machine KIND=macos` (tier 3, one macOS VM at 8 GB with no other VM running). They live in `specs/machine/` and run only there: never in `make check` nor CI, since GitHub's macOS runners are VMs and nest none. The verdict is the exit code; the log is `.artifacts/e2e/<kind>.log`.

Tier 3 clones a test-only image, `station-e2e-macos-<catalogue hash>`: the upstream `macos-tahoe-base` with Homebrew added by `specs/machine/macos/prebake.sh`, rebuilt only when `src/catalog/` changes. It is not a product image. Inside it the harness exports `STATION_MACHINE_KIND=host`, so the guest plays a fresh host, and records what `kern.hv_vmm_present` prints there.

Every VM the harness makes is named `station-e2e-*`, outside station's ownership rule, and deleted at the end, pass or fail. One VM runs the suite: `scripts/install.sh` through its one test seam, `STATION_INSTALL_FROM=<the binary just built>` (a local file instead of the release download, no tag and no checksum read), then `setup`, the tools of `specs/machine/<kind>/one-per-method.txt`, the native commands (`go build`, `npm i`, `bun add -g`, `bunx`, `uv tool install`, `brew install` on macOS, `zsh -c`, `zsh -ic`, and IDE-like calls under `env -i` with only the tool's own bin on PATH, plus `/usr/bin:/bin` for npm, whose launcher is `#!/usr/bin/env bash`).

Then the footprint test: the home is listed (depth 4) before install.sh and after every step, and every new path must be one a Disk row holds (`station disk --json`) or [Footprint](../reference/footprint.md) declares, a home-root exception of Footprint, a path the OS writes itself (`specs/machine/<kind>/os-noise.txt`, never an app's), or one the suite's own native commands put where their user asked (`specs/machine/suite-made.txt`: `uv tool install` links into `~/.local/bin`). An unmatched path fails the suite, named with the step that made it. `station disk free` must leave every Can rebuild row empty and every Your data row untouched, and `station disk erase go --confirm=go`, on a fresh clone of the VM, must take only go's data.

## What CI runs

`.github/workflows/check.yaml` calls the shared `studio-actions` check workflow on every push and every pull request to `main`; what that workflow runs against a repository's `make` targets is [The stack](../concepts/stack.md)'s. Locally the same gate is `make build` then `make check`.

The npm half is linted by the same toolchain as every other repository: `npm run lint` is `typescript check`, and `make lint` runs it after the Go half ([Develop](develop.md)).

## Related

- [Develop](develop.md): the change that these suites judge.
- [Release](release.md): the tagged release, what happens after a green run.
