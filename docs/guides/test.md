# Testing

Two suites prove a change: Go tests (the module tests and the boundary test of the layers), and end-to-end specs that drive the real `station` binary and state what it printed.

| Section                  | Answers                                               |
| ------------------------ | ----------------------------------------------------- |
| The two suites           | What `make test` runs, and in which order             |
| A scenario is a document | The `<case>.spec.yaml` form and how it is regenerated |
| The sandbox              | Why no spec reaches the real machine                  |
| The stubs                | How a spec scripts a system tool                      |
| What CI runs             | The gate a pull request passes                        |

## The two suites

```sh
make test      # go test ./src/... ./specs/..., then typescript test: the specs against a fresh build of station
npm test       # the specs alone
```

The Go tests come first. The module tests are `*_test.go` files next to the code they judge, kept only for what no session of the binary reaches honestly: a terminal, a timing budget, a pure algorithm whose edge cases are cheaper at its own level (the renderer goldens of `src/output/`, the confirm gate of `src/cli/kit/`). The boundary test, `specs/architecture/layers_test.go`, owns the import table and the forbidden symbols of the layers, and proves each of its rules still bites by adding one violating file in memory.

Then `make test` runs the CLI specs under `specs/cli/` on Vitest, through `@jterrazz/test`. There is exactly one runner: `specs/cli/cli.specification.ts` builds `.artifacts/go/station-test`, stamped with the version `git describe` prints, before any spec runs, so the whole suite shares one compilation, and every spec drives that binary. It rebuilds when the binary is missing, when `STATION_FORCE_REBUILD=1` is set (what `make test` does), or when any file under `src/` is newer than the binary. It is an mtime check, so editing the CLI and re-running the suite always exercises the change instead of a stale build.

`vitest.config.ts` is the one line `@jterrazz/test` derives from `studio.config.ts`: its `cli` project collects `specs/cli/`, and every `*.spec.yaml` there becomes a one-test module driving that runner. The runner's defaults are [the sandbox](#the-sandbox).

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

No spec reaches the real machine. The runner sets `HOME` to the run's working directory, `PATH` to its `bin/` alone (the fixture's stubs), and `STATION_SYSTEM_DIRS` to nothing, so a system tool no fixture stubs is "not found" alike on a Mac and on the Linux CI runner. It also pins `NO_COLOR=1`, `COLUMNS=80`, `TZ=UTC` and `STATION_MACHINE_KIND=host`, so every stream is the pipe form and a document means the same thing on every host.

The full grammar is `@jterrazz/test`'s [Spec documents](https://github.com/jterrazz/package-test/blob/main/docs/guides/spec-documents.md) and [CLI specs](https://github.com/jterrazz/package-test/blob/main/docs/guides/cli-specs.md).

## The stubs

A spec copies one `fixture:` directory into its working directory, so stubs live beside the command word that uses them: `specs/cli/<noun>/__fixtures__/<case>/bin/<tool>`. A stub answers by argv from a table, appends `<tool> <argv>` (and the stdin of `brew bundle --file=-`) to `$HOME/.calls`, and exits 97 on anything unscripted. A document asserts `.calls` through `files:` to prove which installer line ran. The GitHub and Tailscale APIs are stub backends the runner serves, reached through `STATION_GITHUB_API_URL` and `STATION_TAILSCALE_API_URL`.

## What a test is for

A test exists because a real regression would turn it red. Every behaviour of a command (each flag, refusal, exit code, `--json` schema and migration) has a spec document through the real binary; the architecture test proves the layers; a module test exists only where the logic is dense and a spec cannot reach it honestly (a terminal, a timing budget, root, a pure algorithm). A test that restates a declaration, a table or a mock's own return is deleted.

## What CI runs

`.github/workflows/check.yaml` calls the shared `studio-actions` check workflow on every push and every pull request to `main`; what that workflow runs against a repository's `make` targets is [The stack](../concepts/stack.md)'s. Locally the same gate is `make build` then `make check`.

The npm half is linted by the same toolchain as every other repository: `npm run lint` is `typescript check`, and `make lint` runs it after the Go half ([Develop](develop.md)).

## Related

- [Develop](develop.md): the change that these suites judge.
- [Release](release.md): the tagged release, what happens after a green run.
