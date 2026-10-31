# The status contract

`station status --json` prints one JSON document, `station.status/v1`, and `studio doctor` reads it to judge the machine.

| Section               | Answers                                     |
| --------------------- | ------------------------------------------- |
| The document          | Every key of the document and what it holds |
| Rules of the encoding | States, nulls, numbers and the budget       |
| Stability             | What may change inside v1                   |
| Who reads it          | The consumer and what it reads              |

## The document

The top-level keys, in order:

| Key            | Holds                                                                                                                          |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `schema`       | `station.status/v1`                                                                                                            |
| `generated_at` | The moment of the reading, RFC 3339 in UTC                                                                                     |
| `station`      | `version` and `commit` of the binary that answered                                                                             |
| `machine`      | `name` (`null` when unregistered), `kind` (`host`, `linux`, `macos`), `hostname`, `os`, `arch`, `user`, `shell`                |
| `system`       | `uptime`, `battery`, `cpu` and `memory.top` (the busiest processes: `pid`, `name`, `value`), `memory.total_bytes`              |
| `environment`  | `network`, `security` and `updates` (checks), `ports` (`pid`, `name`, `port`)                                                  |
| `workspace`    | `repositories` (`path`, `branch`, `dirty`), `dependencies` (`path`, `manager`, `outdated`), `docker`, `disk` (`name`, `bytes`) |
| `tools`        | One object per catalogue tool, as `station tools --json` prints it                                                             |
| `config`       | One object per config item placed here, as `station config --json` prints it                                                   |
| `setup`        | `state` and `steps` (`name`, `state`, `detail`) of `station setup`                                                             |
| `identity`     | Checks: git email, name and signing, gpg key, github, ssh key                                                                  |
| `daemons`      | Checks: one per `ai.jterrazz.*` launch agent                                                                                   |
| `services`     | Checks: the OpenClaw and Hermes gateways, when detected                                                                        |
| `tailnet`      | The object `station tailnet --json` prints, without its peers                                                                  |

A check is `{"name": "filevault", "state": "ok", "value": "on", "detail": ""}`. `workspace.docker` is `{state, containers, images, volumes, dangling_bytes, total_bytes}`.

`environment.security` and `environment.updates`, `daemons` and `services` are macOS-only: they are `[]` on a Linux machine. The `setup.steps` of a machine that is not a host leave out `name` and `layout`.

`remote-login` and `encrypted-dns` are read from the state of the `sshd` and `dns` config items, so `status` and `station config` cannot disagree about them. A machine where those items are not placed has no such row.

`services` holds a service only when it is detected, by its gateway process or by its config file: an OpenClaw with a config and no gateway is `broken`, a machine with neither lists nothing.

## Rules of the encoding

Every `state` is one of `ok`, `degraded`, `absent`, `broken` or `unknown`.

Arrays are never `null`: an empty one is `[]`. The only `null` is `machine.name` on a machine that is not registered.

Numbers are JSON numbers, sizes are `*_bytes`, `generated_at` is RFC 3339 in UTC, and a dependency whose scan outlived its budget reports `outdated: -1`.

Every probe runs under its own deadline, all at once: 2 s by default, 10 s for `brew outdated` and `softwareupdate -l`, 5 s per repository and 20 s overall for the dependency scan. A probe past its budget answers `unknown`, so the command is bounded by about 20 s on a Mac and finishes well under 6 s where every tool is missing.

No kept secret reaches the document.

## Stability

A field is only ever added within v1. A rename or a removal makes the document `station.status/v2`.

The Go structs behind the document are `src/present/status/json.go`, their only definition, and `src/present/status/json_test.go` pins the encoding of a fixed snapshot byte for byte.

## Who reads it

`studio doctor` runs `station status --json` and reads `station`, `tools[].{name,provides,state,version}` and `tailnet.{state,connected,backend_state}`. Its decoder lives in the OS repository, in the station connector of its connectors package.
