# The status contract

`station status --json` prints one JSON document, `station.status/v1`, and `studio doctor` reads it to judge the machine.

| Section               | Answers                                     |
| --------------------- | ------------------------------------------- |
| The document          | Every key of the document and what it holds |
| Rules of the encoding | States, nulls, numbers and the budget       |
| Stability             | What may change inside v1                   |
| Who reads it          | The consumer and what it reads              |

## The document

The document holds these keys, in this order. Its identity comes first.

```json
{
    "schema": "station.status/v1",
    "generated_at": "2026-10-31T14:00:00Z",
    "station": { "version": "v3.0.0", "commit": "abc1234" },
    "machine": {
        "name": "macbook",
        "kind": "host",
        "hostname": "macbook",
        "os": "Darwin 27.0.0",
        "arch": "arm64",
        "user": "jterrazz",
        "shell": "zsh"
    }
}
```

Then `system`, the live part, and `environment`.

```json
{
    "system": {
        "uptime": "3d 4h",
        "battery": "92% charging",
        "cpu": [{ "pid": 1, "name": "x", "value": 12.5 }],
        "memory": {
            "total_bytes": 34359738368,
            "top": [{ "pid": 1, "name": "x", "value": 1048576 }]
        }
    },
    "environment": {
        "network": [{ "name": "local ip", "state": "ok", "value": "192.168.1.2", "detail": "" }],
        "ports": [{ "pid": 1, "name": "node", "port": "3000" }],
        "security": [{ "name": "filevault", "state": "ok", "value": "on", "detail": "" }],
        "updates": [
            {
                "name": "brew",
                "state": "degraded",
                "value": "3 outdated",
                "detail": "station tools upgrade --via brew"
            }
        ]
    }
}
```

Then `workspace`.

```json
{
    "workspace": {
        "repositories": [{ "path": "jterrazz/x", "branch": "main", "dirty": 0 }],
        "dependencies": [{ "path": "jterrazz/x", "manager": "pnpm", "outdated": 2 }],
        "docker": {
            "state": "ok",
            "containers": 0,
            "images": 0,
            "volumes": 0,
            "dangling_bytes": 0,
            "total_bytes": 0
        },
        "disk": [{ "name": "homebrew", "bytes": 123 }]
    }
}
```

Then the catalogue: `tools`, `config` and `setup`.

```json
{
    "tools": [
        {
            "name": "node",
            "category": "Runtimes",
            "method": "mise",
            "places": ["host", "linux", "macos"],
            "state": "ok",
            "version": "24.11.0",
            "managed": true,
            "provides": ["node", "npm", "npx"],
            "detail": ""
        }
    ],
    "config": [
        {
            "name": "hushlogin",
            "category": "Terminal",
            "places": ["host", "linux", "macos"],
            "state": "ok",
            "detail": ""
        }
    ],
    "setup": {
        "state": "ok",
        "steps": [
            { "name": "name", "state": "ok", "detail": "macbook" },
            { "name": "zshrc", "state": "ok", "detail": "" }
        ]
    }
}
```

Last, who the machine is and what runs on it.

```json
{
    "identity": [
        { "name": "git-email", "state": "ok", "value": "admin@jterrazz.com", "detail": "" }
    ],
    "daemons": [
        { "name": "ai.jterrazz.x", "state": "ok", "value": "running · pid 12", "detail": "" }
    ],
    "services": [
        {
            "name": "OpenClaw runtime",
            "state": "broken",
            "value": "not running",
            "detail": "gateway process not found"
        }
    ],
    "tailnet": {
        "state": "ok",
        "connected": true,
        "backend_state": "Running",
        "daemon": "userspace",
        "name": "macbook",
        "ip": "100.64.0.1",
        "keep_awake": true,
        "mode": "userspace",
        "auth_method": "login",
        "vm_join": "ok"
    }
}
```

The `tools` and `config` objects are the ones `station tools --json` and `station config --json` print, and `tailnet` is the object `station tailnet --json` prints without its peers.

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
