# Services

`station ports` lists every TCP port that listens on a Mac or a VM, each in the class of who reaches it. A workspace's port is a service: a workspace is not on the tailnet, so its host shares what it listens on: the host's engine leases a host port for it, listens there on 127.0.0.1 only, and carries each connection into the VM over its one ssh connection. The tailnet reaches that port through the Tailscale app alone: `tailscale serve` publishes it, https for an HTTP service and raw TCP for any other, so nothing of station listens on the tailnet. Nothing is configured per service: a dev server that starts listening in a workspace is published within seconds. A server's ports are never shared by themselves: its own tooling chooses its routes (its own Tailscale device, a tunnel), and `station ports share infra 8080` shares one on demand.

| Section               | Answers                                                      |
| --------------------- | ------------------------------------------------------------ |
| How a port gets a URL | The guest, the lease, the publication                        |
| The commands          | `ports`, `ports share`, `ports stop-sharing`                 |
| The policy            | The `services` keys of `preferences.json`                    |
| Serve and HTTPS       | Serve on or off, the `port` and `path` modes, the admin step |
| Profiles              | How Expo and Vite learn their public URL                     |
| The engine protocol   | `station.api/v2`, for every client                           |

## How a port gets a URL

Inside each running VM, `station guest` (hidden, run by the host's [daemon](machines.md#the-engine) over its ssh connection) reports every listening TCP port, loopback included, and the ports Docker publishes. The VM never opens a connection to its host and never picks a host port. Ports below 1024 and the ignored ones (22) are never reported.

For each reported port the engine takes a **lease**: the lowest free port of the range (default 40000-40999), sticky on (VM, guest port), so the same dev server gets the same URL next week. It then **publishes** it: one host listener on `127.0.0.1:<host port>` (never the tailnet ip, `0.0.0.0` or `::`: station refuses to bind every interface), each connection carried by ssh `direct-tcpip` to the address the VM reported, and, while [Serve](#serve-and-https) is on, a `tailscale serve` entry that exposes that port on the tailnet. A tool bound to 127.0.0.1 in the VM needs no `--host`. tailscaled carries the tailnet's bytes, so a Mac's application firewall, which drops an unsigned binary's bytes on the tailnet ip with no prompt, never stands in the way, and who may reach a lease is the tailnet policy's, never station's.

| Event                                   | What happens                                                       |
| --------------------------------------- | ------------------------------------------------------------------ |
| the port starts listening               | leased and published                                               |
| it listens in a server                  | listed, never shared by itself                                     |
| it stops listening                      | unpublished after 5 s (a restart inside them keeps it); lease kept |
| a VM stops, then runs again             | unpublished, then published again on the same host port            |
| `machines delete`, a throwaway poweroff | released, its serve entry removed                                  |
| `station ports stop-sharing`            | released and remembered: not shared again until a share            |
| 7 days without listening                | released, unless pinned by a [profile](#profiles)                  |
| the range is full, or the VM holds 20   | waiting, `◐` with the cause and the fix                            |
| the host port is busy                   | the next free port (a pinned lease waits, retried every 10 s)      |
| the host reboots                        | the engine publishes every lease again as the guests report        |

The leases live in `~/.local/state/station/leases.json`, a kept file: deleting it changes every URL. Each lease has two URLs, one shape each: `local` `http://127.0.0.1:<port>` (no scheme for a raw TCP service) and `public`, the one `services url` prints and a profile exports. With Serve on, `public` is `https://<public host>:<port>` for an HTTP service or one not yet detected (per [mode](#serve-and-https)) and `<public host>:<port>` for a raw TCP one; with Serve off it is the local URL, the only one there is. `<public host>` is the host's MagicDNS name.

## The commands

```sh
station ports                        # the Tailscale line, then every listening port in its class
station ports --all-machines         # every Mac's, asked over ssh
station ports --all                  # the ports that listen on their machine only, too
station ports --json                 # station.ports.list/v1
station ports share api 3000         # reserve 3000's URL now and print it; in a VM: station ports share 3000
station ports stop-sharing api 3000  # on a Mac; in a VM: station ports stop-sharing 3000
```

The list begins with the Tailscale line, then every port that listens on a Mac or a VM, in the class of who reaches it, piped tab-separated, each row saying its class in words, so a `grep` of one port still tells Private from Public:

| Section                     | Holds                                                                                                              | Columns                                                           |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------- |
| Needs you (`✗`)             | a port reachable outside Tailscale (every interface, the LAN) that no declaration names public                     | machine, port, `needs you`, process, bind, `declared no`, the fix |
| Public (`●` `◐`)            | a port a declaration names public, `●` when its route is seen, `◐` with what is missing when it is not             | machine, port, `public`, process, bind, the route and its proof   |
| Private (`●` `◐` `○`)       | what is shared with you on Tailscale: a VM's reserved port, a machine's own Tailscale device                       | machine, port, `private`, URL, state, process, profile, fix       |
| Listens on its machine only | loopback and other ports only their machine reaches; one `N more listen on their machine only` line unless `--all` | machine, port, `this machine only`, process, bind                 |

A VM behind its Mac is reached through Tailscale only, so its ports are Private whatever it binds. Among the shared rows `●` is shared, `○` not listening (reserved, nothing listens), `◐` waiting (the VM shares 20 already, the range is taken), local only (Serve off, Tailscale signed out, a refused serve), `issuing certificate`, not shared while its VM is stopped or while the engine is stopped. A Mac that does not answer is one `◐` row, never a failure.

`station ports declare <machine> <port> --via <route>` declares a port public on the Mac that keeps the machine's declarations (`~/.config/station/public.json`, a VM's on its Mac), wherever it is asked from; the route is how the port is reached from elsewhere, and what the list looks for to call it seen:

| Route               | Seen when                                                                                                              |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `cloudflare-tunnel` | a `cloudflared` connector runs on that machine (its metrics listener; inside a VM, as its guest reads it)              |
| `tailscale-funnel`  | the Mac's Funnel config (`tailscale serve status --json`, `AllowFunnel`) lists the port; a VM's own device is not read |
| `lan`               | the port listens where the machine's network reaches it: every interface or a LAN address                              |

No `--via` and an unknown route are refused (exit 2) with the three routes. `undeclare` asks yes or no, then the port is red again while it is still reachable outside Tailscale; a port never declared is refused (exit 2). Inside a VM both are refused: its Mac keeps them. The Remote login setting (`sshd`), applied on a Mac with no battery (an always-on Mac, never a laptop), declares its `22` through `ports.declare` (`your LAN · Remote login · by Settings`), and `settings off sshd` takes it back. In `--json` a declared port carries `declared: {via, by, seen, missing}`.

Each Mac's reserved ports are one line at the end: `Reserved on This Mac: 2 of 1000 (40000-40999), 998 free`, one per Mac with `--all-machines`. In `--json` each Mac carries it in `range` (`mac`, `from`, `to`, `reserved`, `free`), read from its policy and lease file, so it is there with the engine down.

The engine's exposure job reads what listens on its Mac and its VMs at its start, every 5 minutes and whenever a guest reports a change. A port that turns red is a finding of `station overview` (`✗ port 9000 is reachable outside Tailscale`) and is told once, as a `port_exposed` event (`<machine>: port <port> is reachable outside Tailscale`), then again only after it stopped being red.

`ports share <port>` in a VM reserves the lease at once, whether or not anything listens yet, so a value can be computed before the tool starts: `$(station ports share 3000)`. On a Mac it names its VM, `station ports share api 3000`; with the Mac's engine away it is exit 1 (`Error: macbook is not connected to this VM`, `Hint: on macbook: station setup`), and in a VM no host has connected to yet, so with no host name to give, every `ports` command is exit 1 (`Error: no host has connected to this VM yet`, `Hint: on the VM's host: station setup`).

`ports stop-sharing` gives a lease back, after yes or no (`--yes` piped), and is remembered, so the next listen is not shared until `ports share`. A VM of another Mac is released on that Mac, over ssh. With the engine down, a Mac edits the lease store itself.

## The policy

The host owns it, in `preferences.json`; every key is optional and no command edits them:

```json
"services": { "range": "40000-40999", "max_per_vm": 20, "ignored_ports": [22], "min_port": 1024, "https": "port" }
```

The engine reads the file again when it changes. [Configuration](configuration.md) has the rest of `preferences.json`.

## Serve and HTTPS

Every lease leaves the host through Tailscale Serve. Serve is **on** when `tailscale status --json` lists a certificate domain (`CertDomains`: the admin has enabled HTTPS certificates, which Serve needs); the tailnet logged out or absent is off. The engine reads it at its start, at each VM run and every 30 s; `station status` shows it as the `Tailscale Serve` gateway and `services --json` as each host's `serve` (`on` or `off`).

To turn it on, once per tailnet: in the [admin console's DNS page](https://login.tailscale.com/admin/dns), MagicDNS on, then HTTPS Certificates (machine names then appear in Certificate Transparency logs), and Serve where the console asks for it.

With Serve on, an HTTP service (detected once per lease: a server that speaks first is not HTTP, else `HEAD /` must answer `HTTP/`) is served https with a valid certificate, so an iPhone on the tailnet opens it as a secure context; any other TCP service is served raw:

| Service              | Serve line                                                                              | Public URL                                  |
| -------------------- | --------------------------------------------------------------------------------------- | ------------------------------------------- |
| HTTP, `https` `port` | `tailscale serve --bg --https=<host port> http://127.0.0.1:<host port>`                 | `https://macbook.tail1234.ts.net:40000`     |
| HTTP, `https` `path` | `tailscale serve --bg --https=443 --set-path=/<vm>/<port> http://127.0.0.1:<host port>` | `https://macbook.tail1234.ts.net/api/8081/` |
| raw TCP              | `tailscale serve --bg --tcp=<host port> tcp://127.0.0.1:<host port>`                    | `macbook.tail1234.ts.net:40002`             |

Unpublishing runs the same line ending `off`. `https` `off`, and any other word, is read as `port`. A serve refused while Serve is on leaves the service published locally: its row `◐` with `serve:` and tailscale's own line (never the CLI's `Warning:` lines), its public URL unchanged, the serve retried at its next publish. In `path` mode a tool that builds absolute URLs needs its base path set to `/<vm>/<port>/`.

tailscaled issues the host's certificate at the first https request, 34.5 s measured, so the engine warms it: at its start with Serve on, and when Serve turns on, it runs `tailscale cert --cert-file=- --key-file=- <host>` in the background, its output thrown away (`/dev/null` is refused as not a regular file), one at a time, within 120 s, never holding a publish. Meanwhile every row served `--https` says `issuing certificate` (`◐`, its URL unchanged); `engine.log` then says `certificate for <host> ready in <d>` or `not issued:` with tailscale's line.

With Serve off no serve runs (one would block waiting for the admin): every lease is local only, every row of the host says `Serve is off on Tailscale: turn on HTTPS certificates in the Tailscale admin console`, and every public URL is the local one; `engine.log` says it once per change. Serve turning on serves every published lease at the next read.

Station owns only the serve entries on its range (both kinds) or under `/<vm>/` of its VMs (`path`), and, while Serve is on, removes the ones no lease justifies at the engine's start and when Serve turns on; it never touches another entry.

## Profiles

A tool that must know its own public URL gets it from its environment. A profile, data in the catalogue, names a dev tool's default port and the variables that carry the URL; at `machines create` the engine reserves every profile's port for the new VM, and the VM's shell (`station shell zsh`, from `~/.config/station/services.env`) exports the variables. The engine renders them at every run of the VM (create, `start`, `restart`, its own run at boot; never when it adopts a running VM) from Serve's state then, so they are fixed for a run, and a permanent VM picks up the public URLs at its first start after Serve is turned on:

| Profile  | Guest port | Variable                                 | Value                                                   |
| -------- | ---------- | ---------------------------------------- | ------------------------------------------------------- |
| expo     | 8081       | `EXPO_PACKAGER_PROXY_URL`                | the public URL, `https://macbook.tail1234.ts.net:40000` |
| vite     | 5173       | `__VITE_ADDITIONAL_SERVER_ALLOWED_HOSTS` | the public host, `macbook.tail1234.ts.net`              |
| every VM |            | `STATION_PUBLIC_HOST`                    | the public host                                         |

With Serve off they hold the local URL, `http://127.0.0.1:<host port>`, and the public host `127.0.0.1`, and `machines create` says so on a `◐ services` row. So `npx expo start` in a VM writes the published https URL into its manifests, and Vite accepts requests for the public host, with no flag. A tool on another port composes its URL itself: `PORT=3000 BASE_URL=$(station services url 3000) npm start`. Same-origin traffic (HMR websockets, relative URLs) needs nothing: it rides the same host port. A tool whose setting has no environment variable gets its line here.

## The engine protocol

`station.api/v2`: JSON lines over the unix socket `~/.local/state/station/engine.sock` (mode 0600; the engine listens on no TCP port, and the leases' forwarders bind `127.0.0.1` only), one frame per line. A request names a `call` or a `watch` under its `id`; the answer is `progress` frames, then one `out` or one `error`. The calls of one connection are answered in the order they came. The CLI is one client, the menu bar another.

```text
→ {"v":2,"id":"7","call":"machines.create-workspace","in":{"Request":{"Image":"linux","Lifetime":"throwaway"}}}
← {"v":2,"id":"7","progress":{"task":"Booting linux-1"}}
← {"v":2,"id":"7","out":{"VM":{"Name":"linux-1"}}}
→ {"v":2,"id":"8","call":"engine.stop","in":{"Confirm":"inf"}}
← {"v":2,"id":"8","error":{"type":"needs-confirmation","code":"confirm.yes-needed","subject":"stop","message":"refusing without --yes: stop","hint":"re-run with --yes"}}
```

The engine serves every call of [`calls.json`](../../src/remote/api/calls.json) (the registry `make generate` projects: each call's name, its output schema, its section, where it runs, its confirmation and the errors it declares), run on this host as the CLI runs it, and its own calls, which the CLI's reads and the VMs' guests ask of its live state:

| call                                                       | in                                | out                                                                                                                                                                                                                                                             |
| ---------------------------------------------------------- | --------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `engine.hello`                                             |                                   | `version pid started_at executable` and `calls`, the names of `calls.json` it serves                                                                                                                                                                            |
| `engine.vms`                                               |                                   | each VM's `name state run_pid guest ip detail`                                                                                                                                                                                                                  |
| `engine.vm-run`                                            | `vm`                              | supervises a new record; answers when its guest is connected                                                                                                                                                                                                    |
| `engine.vm-start` / `engine.vm-stop` / `engine.vm-restart` | `vm`                              | any VM                                                                                                                                                                                                                                                          |
| `engine.vm-delete` / `engine.vm-forget`                    | `vm`                              | the VM, its files, blocks, leases and serve entries / its supervision only                                                                                                                                                                                      |
| `engine.guest-exec`                                        | `vm, line, stdin`                 | `code stdout stderr` of a command run over the VM's guest connection                                                                                                                                                                                            |
| `engine.accounts`                                          |                                   | each running VM's accounts, read by its guest                                                                                                                                                                                                                   |
| `engine.services`                                          |                                   | this host's `station.services/v1` host object                                                                                                                                                                                                                   |
| `engine.reserve`                                           | `vm, guest_port, pinned, profile` | the lease (an existing one is returned)                                                                                                                                                                                                                         |
| `engine.release`                                           | `vm, guest_port`                  | the released lease                                                                                                                                                                                                                                              |
| `engine.stop-all`                                          |                                   | `vms` (each VM's result) and `power_ended`                                                                                                                                                                                                                      |
| `engine.power`                                             |                                   | the Power `session`, `last_end`, `reading` and `guards`                                                                                                                                                                                                         |
| `engine.power-awake`                                       | `lid, battery, for, by`           | the session (`for` in seconds), `refreshed` when one held                                                                                                                                                                                                       |
| `engine.power-sleep`                                       |                                   | `ended`, `after` (seconds)                                                                                                                                                                                                                                      |
| `engine.activity`                                          |                                   | `since` (the engine's start) and each machine's `last` reading, `fine` and `coarse`                                                                                                                                                                             |
| `engine.refresh`                                           | `pages`                           | `{}`: each page it names is read again now (the menu bar's Needs you, a machine's page, the switcher)                                                                                                                                                           |
| watch `page`                                               | `pages` (named below)             | an `out` per update: the opening `stream pages`, then `event` `page`, `tick`, `notice`                                                                                                                                                                          |
| watch `engine.watch`                                       | `machines` (none is This Mac)     | the opening `machines`; while it stays open those machines are sampled every 2 s                                                                                                                                                                                |
| watch `engine.events`                                      |                                   | the opening `stream engine.events`, then an `event` `notice` per event of SPEC §10.4's closed list alone (`server_restarting`, `server_loop`, `server_powered_off`, `server_back`, `power_ended`, `port_exposed`, `disk_low`), each in its notification's words |

An error carries its `type` (which decides the exit code), its `code` (a call's own, `call.unknown`, `call.bad-frame` for a line that is no v2 frame, or the engine's: `unknown_vm`, `temporary`, `not_running`, `range_full`, `max_per_vm`, `guest_timeout`, `internal`, `power_guard`, `power_range`, `power_unsupported`, `unknown_page`), its `subject` and `detail`, a `message` and a `hint`, worded as the CLI prints them. A watch takes its connection: a later line on it gets no answer, so a client sends its calls on a second connection. The first page is the frame (pending values), then a page whenever one changes, coalesced over 250 ms and never resent unchanged; a client that does not read for 5 s is dropped. The pages are `menubar` (Home), `menubar.switcher`, `menubar.machine:<name>` (one machine's page, `menubar.machine:infra`) and `power`. Each is read only while a client watches it. Home: the Strip every 2 s, the machines (each Mac's Awake with them) every 30 s, Needs you every 15 min or on `refresh`, this host's VMs, leases and Power at once. The switcher: the machines when it opens, then every 30 s. A machine's page: its activity every 2 s and the rest every 30 s, each read on that machine, so another Mac's page is that Mac's engine's answer, relayed by this one. A machine is sampled every 60 s, and every 2 s only while a client holds `engine.watch` on it, as the menu bar does while its popover is open: a watch of pages or events alone never makes it so. Inside a VM, the guest's socket serves `engine.services`, `engine.reserve` and `engine.release` in the same frames, relayed to the host's engine.

## Related

- [Machines](machines.md): the engine, and how a VM is reached by ssh.
- [Configuration](configuration.md): `preferences.json`.
- [Glossary](../concepts/glossary.md): service, lease, publish, profile, public host.
