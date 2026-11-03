# Services

A service is a TCP port something listens on inside a VM. The VM is never on the tailnet, so its host publishes what it listens on: the host's daemon leases a host port for it, listens there on 127.0.0.1 only, and carries each connection into the VM over its one ssh connection. The tailnet reaches that port through the Tailscale app alone: `tailscale serve` publishes it, https for an HTTP service and raw TCP for any other, so nothing of station listens on the tailnet. Nothing is configured per service: a dev server that starts listening is published within seconds.

| Section               | Answers                                                      |
| --------------------- | ------------------------------------------------------------ |
| How a port gets a URL | The guest, the lease, the publication                        |
| The commands          | `services`, `services url`, `services release`               |
| The policy            | The `services` keys of `settings.json`                       |
| Serve and HTTPS       | Serve on or off, the `port` and `path` modes, the admin step |
| Profiles              | How Expo and Vite learn their public URL                     |
| The daemon protocol   | `station.daemon/v1`, for another client                      |

## How a port gets a URL

Inside each running VM, `station guest` (hidden, run by the host's [daemon](machines.md#the-daemon) over its ssh connection) reports every listening TCP port, loopback included, and the ports Docker publishes. The VM never opens a connection to its host and never picks a host port. Ports below 1024 and the ignored ones (22) are never reported.

For each reported port the daemon takes a **lease**: the lowest free port of the range (default 40000-40999), sticky on (VM, guest port), so the same dev server gets the same URL next week. It then **publishes** it: one host listener on `127.0.0.1:<host port>` (never the tailnet ip, `0.0.0.0` or `::`: station refuses to bind every interface), each connection carried by ssh `direct-tcpip` to the address the VM reported, and, while [Serve](#serve-and-https) is on, a `tailscale serve` entry that exposes that port on the tailnet. A tool bound to 127.0.0.1 in the VM needs no `--host`. tailscaled carries the tailnet's bytes, so a Mac's application firewall, which drops an unsigned binary's bytes on the tailnet ip with no prompt, never stands in the way, and who may reach a lease is the tailnet policy's, never station's.

| Event                                   | What happens                                                       |
| --------------------------------------- | ------------------------------------------------------------------ |
| the port starts listening               | leased and published                                               |
| it stops listening                      | unpublished after 5 s (a restart inside them keeps it); lease kept |
| a lasting VM stops, then runs again     | unpublished, then published again on the same host port            |
| `machines delete`, a temporary poweroff | released, its serve entry removed                                  |
| `station services release`              | released: the next listen gets a new port                          |
| 7 days without listening                | released, unless pinned by a [profile](#profiles)                  |
| the range is full, or the VM holds 20   | refused, `✗` with the cause                                        |
| the host port is busy                   | the next free port (a pinned lease waits, retried every 10 s)      |
| the host reboots                        | the daemon publishes every lease again as the guests report        |

The leases live in `~/.local/state/station/leases.json`, a kept file: deleting it changes every URL. Each lease has two URLs, one shape each: `local` `http://127.0.0.1:<port>` (no scheme for a raw TCP service) and `public`, the one `services url` prints and a profile exports. With Serve on, `public` is `https://<public host>:<port>` for an HTTP service or one not yet detected (per [mode](#serve-and-https)) and `<public host>:<port>` for a raw TCP one; with Serve off it is the local URL, the only one there is. `<public host>` is the host's MagicDNS name.

## The commands

```sh
station services                     # every host's services, asked over ssh; --here for this host only
station services --json              # station.services/v1
station services url 3000            # in a VM: reserve 3000's URL now and print it
station services release api:3000    # on a host; in a VM: station services release 3000
```

Rows have one schema, piped tab-separated: mark, VM, guest port, public URL, host, state, process, detail. `●` published, `○` waiting (leased, nothing listens), `✗` refused, `◐` degraded (Serve off, a refused serve, a moved port) or unknown (the host's daemon is down). An unreachable host is one `◐` row, never a failure.

So one command shows what ports exist and what is free across the fleet, each host's group is headed on a terminal by its range: `macbook  40000-40999 · 3 leased · 997 free · 20 per VM`. In `--json` each host carries it as `range` (`low`, `high`, `leased`, `free`, `max_per_vm`), read from that host's policy and lease file, so it is there with the daemon down; a host too old to tell its range has none. The piped form stays rows only.

`services url <port>` in a VM reserves the lease at once, whether or not anything listens yet, so a value can be computed before the tool starts: `$(station services url 3000)`. On a host it is exit 2 (`Hint: station services url runs inside a VM; here: station services`); with the host's daemon away it is exit 1 (`Error: macbook is not connected to this VM`, `Hint: on macbook: station setup`), and in a VM no host has connected to yet, so with no host name to give, every `services` command is exit 1 (`Error: no host has connected to this VM yet`, `Hint: on the VM's host: station setup`).

`services release` gives a lease back. A VM of another host is released on that host, over ssh. A profile's lease asks first (`--yes` piped): its URL is in the VM's environment. With the daemon down, a host edits the lease store itself.

## The policy

The host owns it, in `settings.json`; every key is optional and no command edits them:

```json
"services": { "range": "40000-40999", "max_per_vm": 20, "ignored_ports": [22], "min_port": 1024, "https": "port" }
```

The daemon reads the file again when it changes. [Configuration](configuration.md) has the rest of `settings.json`.

## Serve and HTTPS

Every lease leaves the host through Tailscale Serve. Serve is **on** when `tailscale status --json` lists a certificate domain (`CertDomains`: the admin has enabled HTTPS certificates, which Serve needs); the tailnet logged out or absent is off. The daemon reads it at its start, at each VM run and every 30 s; `station status` shows it as the `Tailscale Serve` gateway and `services --json` as each host's `serve` (`on` or `off`).

To turn it on, once per tailnet: in the [admin console's DNS page](https://login.tailscale.com/admin/dns), MagicDNS on, then HTTPS Certificates (machine names then appear in Certificate Transparency logs), and Serve where the console asks for it.

With Serve on, an HTTP service (detected once per lease: a server that speaks first is not HTTP, else `HEAD /` must answer `HTTP/`) is served https with a valid certificate, so an iPhone on the tailnet opens it as a secure context; any other TCP service is served raw:

| Service              | Serve line                                                                              | Public URL                                  |
| -------------------- | --------------------------------------------------------------------------------------- | ------------------------------------------- |
| HTTP, `https` `port` | `tailscale serve --bg --https=<host port> http://127.0.0.1:<host port>`                 | `https://macbook.tail1234.ts.net:40000`     |
| HTTP, `https` `path` | `tailscale serve --bg --https=443 --set-path=/<vm>/<port> http://127.0.0.1:<host port>` | `https://macbook.tail1234.ts.net/api/8081/` |
| raw TCP              | `tailscale serve --bg --tcp=<host port> tcp://127.0.0.1:<host port>`                    | `macbook.tail1234.ts.net:40002`             |

Unpublishing runs the same line ending `off`. `https` `off`, and any other word, is read as `port`. A serve refused while Serve is on leaves the service published locally: its row `◐` with `serve:` and tailscale's own line (never the CLI's `Warning:` lines), its public URL unchanged, the serve retried at its next publish. In `path` mode a tool that builds absolute URLs needs its base path set to `/<vm>/<port>/`.

tailscaled issues the host's certificate at the first https request, 34.5 s measured, so the daemon warms it: at its start with Serve on, and when Serve turns on, it runs `tailscale cert --cert-file=- --key-file=- <host>` in the background, its output thrown away (`/dev/null` is refused as not a regular file), one at a time, within 120 s, never holding a publish. Meanwhile every row served `--https` says `issuing certificate` (`◐`, its URL unchanged); `daemon.log` then says `certificate for <host> ready in <d>` or `not issued:` with tailscale's line.

With Serve off no serve runs (one would block waiting for the admin): every lease is local only, every row of the host says `Serve is off on the tailnet: enable HTTPS certificates in the Tailscale admin console`, and every public URL is the local one; `daemon.log` says it once per change. Serve turning on serves every published lease at the next read.

Station owns only the serve entries on its range (both kinds) or under `/<vm>/` of its VMs (`path`), and, while Serve is on, removes the ones no lease justifies at the daemon's start and when Serve turns on; it never touches another entry.

## Profiles

A tool that must know its own public URL gets it from its environment. A profile, data in the catalogue, names a dev tool's default port and the variables that carry the URL; at `machines create` the daemon reserves every profile's port for the new VM, and the VM's shell (`station shell zsh`, from `~/.config/station/services.env`) exports the variables. The daemon renders them at every run of the VM (create, `start`, `restart`, its own run at boot; never when it adopts a running VM) from Serve's state then, so they are fixed for a run, and a lasting VM picks up the public URLs at its first start after Serve is turned on:

| Profile  | Guest port | Variable                                 | Value                                                   |
| -------- | ---------- | ---------------------------------------- | ------------------------------------------------------- |
| expo     | 8081       | `EXPO_PACKAGER_PROXY_URL`                | the public URL, `https://macbook.tail1234.ts.net:40000` |
| vite     | 5173       | `__VITE_ADDITIONAL_SERVER_ALLOWED_HOSTS` | the public host, `macbook.tail1234.ts.net`              |
| every VM |            | `STATION_PUBLIC_HOST`                    | the public host                                         |

With Serve off they hold the local URL, `http://127.0.0.1:<host port>`, and the public host `127.0.0.1`, and `machines create` says so on a `◐ services` row. So `npx expo start` in a VM writes the published https URL into its manifests, and Vite accepts requests for the public host, with no flag. A tool on another port composes its URL itself: `PORT=3000 BASE_URL=$(station services url 3000) npm start`. Same-origin traffic (HMR websockets, relative URLs) needs nothing: it rides the same host port. A tool whose setting has no environment variable gets its line here.

## The daemon protocol

`station.daemon/v1`: JSON lines over the unix socket `~/.local/state/station/daemon.sock` (mode 0600), one request and one response per line, answered in order. The CLI is one client; a menu-bar app would be another.

```text
→ {"v":1,"id":"1","op":"ping"}
← {"v":1,"id":"1","ok":true,"result":{"version":"v3.0.0","pid":812,"started_at":"2026-11-01T09:00:00Z"}}
```

| op                           | args                     | result                                                                                  |
| ---------------------------- | ------------------------ | --------------------------------------------------------------------------------------- |
| `ping`                       |                          | `version pid started_at`                                                                |
| `vms`                        |                          | each VM's `name state run_pid guest ip detail`                                          |
| `run`                        | `vm`                     | supervises a new record; answers when its guest is connected                            |
| `start` / `stop` / `restart` | `vm`                     | lasting VMs only                                                                        |
| `delete`                     | `vm`                     | the VM, its files, blocks, leases and serve entries                                     |
| `services`                   |                          | this host's `station.services/v1` host object                                           |
| `reserve`                    | `vm, guest_port, pinned` | the lease (an existing one is returned)                                                 |
| `release`                    | `vm, guest_port`         | the released lease                                                                      |
| `stop_all`                   |                          | `vms` (each VM's result) and `power_ended`                                              |
| `power`                      |                          | the Power `session`, `last_end`, `reading` and `guards`                                 |
| `power_awake`                | `lid, battery, for, by`  | the session (`for` in seconds), `refreshed` when one held                               |
| `power_sleep`                |                          | `ended`, `after` (seconds)                                                              |
| `subscribe`                  | `pages` (`power`)        | a stream: one full page per change, `tick` every 15 s, `notice` when a guard ends Power |

An error carries `code` (`unknown_op`, `unknown_vm`, `temporary`, `not_running`, `range_full`, `max_per_vm`, `guest_timeout`, `internal`, `power_guard`, `power_range`, `power_unsupported`, `unknown_page`), a `message` and a `hint`, worded as the CLI prints them. After `subscribe`'s `ok` the connection only streams: a later request line on it gets no answer, so a client sends its actions on a second connection.

## Related

- [Machines](machines.md): the daemon, and how a VM is reached by ssh.
- [Configuration](configuration.md): `settings.json`.
- [Glossary](../concepts/glossary.md): service, lease, publish, profile, public host.
