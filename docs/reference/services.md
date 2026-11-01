# Services

A service is a TCP port something listens on inside a VM. The VM is never on the tailnet, so its host publishes what it listens on: the host's daemon leases a host port for it, listens there on 127.0.0.1 and on the host's tailnet ip, and carries each connection into the VM over its one ssh connection. An HTTP service also gets `tailscale serve` https; in the `port` mode that tailnet port is `tailscale serve`'s, so the daemon listens on 127.0.0.1 alone. Nothing is configured per service: a dev server that starts listening is published within seconds.

| Section               | Answers                                                        |
| --------------------- | -------------------------------------------------------------- |
| How a port gets a URL | The guest, the lease, the publication                          |
| The commands          | `services`, `services url`, `services release`                 |
| The policy            | The `services` keys of `settings.json`                         |
| HTTPS                 | The `port`, `path` and `off` modes, and what the tailnet needs |
| Profiles              | How Expo and Vite learn their public URL                       |
| The daemon protocol   | `station.daemon/v1`, for another client                        |

## How a port gets a URL

Inside each running VM, `station guest` (hidden, run by the host's [daemon](machines.md#the-daemon) over its ssh connection) reports every listening TCP port, loopback included, and the ports Docker publishes. The VM never opens a connection to its host and never picks a host port. Ports below 1024 and the ignored ones (22) are never reported.

For each reported port the daemon takes a **lease**: the lowest free port of the range (default 40000-40999), sticky on (VM, guest port), so the same dev server gets the same URL next week. It then **publishes** it: host listeners on `127.0.0.1:<host port>` and on the host's tailnet ip (never `0.0.0.0`: station refuses to bind every interface; an HTTP service in the https `port` mode has no tailnet-ip listener, since `tailscale serve` holds that port), each connection carried by ssh `direct-tcpip` to the address the VM reported. A tool bound to 127.0.0.1 in the VM needs no `--host`.

| Event                                   | What happens                                                       |
| --------------------------------------- | ------------------------------------------------------------------ |
| the port starts listening               | leased and published                                               |
| it stops listening                      | unpublished after 5 s (a restart inside them keeps it); lease kept |
| a lasting VM stops, then runs again     | unpublished, then published again on the same host port            |
| `machines delete`, a temporary poweroff | released, its https entry removed                                  |
| `station services release`              | released: the next listen gets a new port                          |
| 7 days without listening                | released, unless pinned by a [profile](#profiles)                  |
| the range is full, or the VM holds 20   | refused, `✗` with the cause                                        |
| the host port is busy                   | the next free port (a pinned lease waits, retried every 10 s)      |
| the host reboots                        | the daemon publishes every lease again as the guests report        |

The leases live in `~/.local/state/station/leases.json`, a kept file: deleting it changes every URL. Each URL has one shape: `local` `http://127.0.0.1:<port>`, `tailnet` `<public host>:<port>` and `https` per [mode](#https). The public URL, the one `services url` prints, is the https one for an HTTP service when https is on, else the tailnet one.

## The commands

```sh
station services                     # every host's services, asked over ssh; --here for this host only
station services --json              # station.services/v1
station services url 3000            # in a VM: reserve 3000's URL now and print it
station services release api:3000    # on a host; in a VM: station services release 3000
```

Rows have one schema, piped tab-separated: mark, VM, guest port, public URL, host, state, process, detail. `●` published, `○` waiting (leased, nothing listens), `✗` refused, `◐` degraded (local only, no https, a moved port) or unknown (the host's daemon is down). An unreachable host is one `◐` row, never a failure.

So one command shows what ports exist and what is free across the fleet, each host's group is headed on a terminal by its range: `macbook  40000-40999 · 3 leased · 997 free · 20 per VM`. In `--json` each host carries it as `range` (`low`, `high`, `leased`, `free`, `max_per_vm`), read from that host's policy and lease file, so it is there with the daemon down; a host too old to tell its range has none. The piped form stays rows only.

`services url <port>` in a VM reserves the lease at once, whether or not anything listens yet, so a value can be computed before the tool starts: `$(station services url 3000)`. On a host it is exit 2 (`Hint: station services url runs inside a VM; here: station services`); with the host's daemon away it is exit 1 (`Error: macbook is not connected to this VM`, `Hint: on macbook: station setup`), and in a VM no host has connected to yet, so with no host name to give, every `services` command is exit 1 (`Error: no host has connected to this VM yet`, `Hint: on the VM's host: station setup`).

`services release` gives a lease back. A VM of another host is released on that host, over ssh. A profile's lease asks first (`--yes` piped): its URL is in the VM's environment. With the daemon down, a host edits the lease store itself.

## The policy

The host owns it, in `settings.json`; every key is optional and no command edits them:

```json
"services": { "range": "40000-40999", "max_per_vm": 20, "ignored_ports": [22], "min_port": 1024, "https": "port" }
```

The daemon reads the file again when it changes. [Configuration](configuration.md) has the rest of `settings.json`.

## HTTPS

An HTTP service (detected once per lease: a server that speaks first is not HTTP, else `HEAD /` must answer `HTTP/`) gets an https URL from the Tailscale app's `tailscale serve`, with a valid certificate: an iPhone on the tailnet opens it as a secure context.

| `https` | Serve line                                                                              | URL                                              |
| ------- | --------------------------------------------------------------------------------------- | ------------------------------------------------ |
| `port`  | `tailscale serve --bg --https=<host port> http://127.0.0.1:<host port>`                 | `https://macbook.tail1234.ts.net:40000`          |
| `path`  | `tailscale serve --bg --https=443 --set-path=/<vm>/<port> http://127.0.0.1:<host port>` | `https://macbook.tail1234.ts.net/api/8081/`      |
| `off`   | none                                                                                    | the tailnet one, `macbook.tail1234.ts.net:40000` |

The tailnet needs HTTPS certificates enabled once, in the [admin console's DNS page](https://login.tailscale.com/admin/dns) (MagicDNS on, then HTTPS Certificates). A refused serve (or one that waits for the admin to enable Serve) leaves the service published without https: its row `◐` with tailscale's own line, its public URL the tailnet one, and in `port` mode its tailnet port bound by the daemon instead. In `path` mode a tool that builds absolute URLs needs its base path set to `/<vm>/<port>/`. Whether the app serves https on every port of the range is proved on a real host before `port` stays the default ([TODO](../../TODO.md)).

Station owns only the serve entries on its range (`port`) or under `/<vm>/` of its VMs (`path`), and removes the ones no lease justifies at the daemon's start; it never touches another entry.

## Profiles

A tool that must know its own public URL gets it from its environment. A profile, data in the catalogue, names a dev tool's default port and the variables that carry the URL; at `machines create` the daemon reserves every profile's port for the new VM, and the VM's shell (`station shell zsh`, from `~/.config/station/services.env`) exports the variables, fixed for the VM's whole life:

| Profile  | Guest port | Variable                                 | Value                                                   |
| -------- | ---------- | ---------------------------------------- | ------------------------------------------------------- |
| expo     | 8081       | `EXPO_PACKAGER_PROXY_URL`                | the public URL, `https://macbook.tail1234.ts.net:40000` |
| vite     | 5173       | `__VITE_ADDITIONAL_SERVER_ALLOWED_HOSTS` | the public host, `macbook.tail1234.ts.net`              |
| every VM |            | `STATION_PUBLIC_HOST`                    | the public host                                         |

So `npx expo start` in a VM writes the published https URL into its manifests, and Vite accepts requests for the public host, with no flag. A tool on another port composes its URL itself: `PORT=3000 BASE_URL=$(station services url 3000) npm start`. Same-origin traffic (HMR websockets, relative URLs) needs nothing: it rides the same host port. A tool whose setting has no environment variable gets its line here.

## The daemon protocol

`station.daemon/v1`: JSON lines over the unix socket `~/.local/state/station/daemon.sock` (mode 0600), one request and one response per line, answered in order. The CLI is one client; a menu-bar app would be another.

```text
→ {"v":1,"id":"1","op":"ping"}
← {"v":1,"id":"1","ok":true,"result":{"version":"v3.0.0","pid":812,"started_at":"2026-11-01T09:00:00Z"}}
```

| op                           | args                     | result                                                       |
| ---------------------------- | ------------------------ | ------------------------------------------------------------ |
| `ping`                       |                          | `version pid started_at`                                     |
| `vms`                        |                          | each VM's `name state run_pid guest ip detail`               |
| `run`                        | `vm`                     | supervises a new record; answers when its guest is connected |
| `start` / `stop` / `restart` | `vm`                     | lasting VMs only                                             |
| `delete`                     | `vm`                     | the VM, its files, blocks, leases and serve entries          |
| `services`                   |                          | this host's `station.services/v1` host object                |
| `reserve`                    | `vm, guest_port, pinned` | the lease (an existing one is returned)                      |
| `release`                    | `vm, guest_port`         | the released lease                                           |

An error carries `code` (`unknown_op`, `unknown_vm`, `temporary`, `not_running`, `range_full`, `max_per_vm`, `guest_timeout`, `internal`), a `message` and a `hint`, worded as the CLI prints them.

## Related

- [Machines](machines.md): the daemon, and how a VM is reached by ssh.
- [Configuration](configuration.md): `settings.json`.
- [Glossary](../concepts/glossary.md): service, lease, publish, profile, public host.
