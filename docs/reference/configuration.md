# Configuration

`station settings` sets a machine up: dotfiles, logins and macOS defaults. Each entry is a setting, named by what it sets, and `station`'s own file is `preferences.json`.

| Section          | Answers                                        |
| ---------------- | ---------------------------------------------- |
| The items        | Every config item, its place and what it does  |
| Values           | How an item that needs values asks for them    |
| preferences.json | The file station keeps its own choices in      |
| Environment      | The variables station reads, all resolved once |

## The items

A setting that does not live on this machine's kind reads Not for this machine, and an apply of it is refused with exit 2. Each that lives here reads Applied (exactly as station applied it), Changed since (applied, then changed by someone: `station settings diff` shows how), Not applied, or Off here (the owner's choice, kept as `off_here` in `preferences.json`). What station applied, its fingerprint and when, is kept under `~/.local/state/station/applied/`.

| Item                        | Place       | Sets                                                                                                  | Reversible |
| --------------------------- | ----------- | ----------------------------------------------------------------------------------------------------- | ---------- |
| `hushlogin`                 | all         | `~/.hushlogin`                                                                                        | no         |
| `tmux-dotfiles`             | all         | `~/.tmux.conf`, reloaded in a running tmux                                                            | no         |
| `starship-dotfiles`         | all         | `~/.config/starship.toml`                                                                             | no         |
| `github-login`              | all         | `gh auth login` over ssh; in a VM this token is the VM's own                                          | no         |
| `ghostty-dotfiles`          | host, macos | `~/.config/ghostty/` config and themes                                                                | no         |
| `zed-dotfiles`              | host, macos | `~/.config/zed/` config and formatter values                                                          | no         |
| `git-signing`               | host, macos | GPG commit signing                                                                                    | no         |
| `spotlight-exclude`         | host, macos | Spotlight exclusions for build folders                                                                | no         |
| `dock-reset`, `dock-spacer` | host, macos | the Dock                                                                                              | no         |
| `backup-exclusions`         | host        | `~/.cache` and Tart's VMs out of Time Machine (`tmutil addexclusion`) ([Storage](storage.md#backups)) | yes        |
| `tart-launcher`             | host        | `~/.local/bin/tart`, the launcher that sets `TART_HOME` ([Storage](storage.md#tart))                  | no         |
| `ssh-key`                   | host        | `~/.ssh/id_ed25519`                                                                                   | no         |
| `dns`                       | host        | the Quad9 encrypted DNS profile                                                                       | no         |
| `autologin`                 | host        | automatic login, for an unattended host; needs root                                                   | yes        |
| `power`                     | host        | the power policy of an always-on host; needs root                                                     | yes        |
| `lock-after-login`          | host        | lock the screen after the automatic login; needs root                                                 | yes        |
| `sshd`                      | host        | Remote Login; needs root                                                                              | yes        |
| `claude-token`              | host        | the Claude Code token lent to VMs ([Secrets](secrets.md#claude-in-a-vm))                              | yes        |

The settings are Set by station; `autologin`, `power`, `lock-after-login` and `sshd` are its Always-on Mac group, for a Mac that hosts servers: opt-in, applied by name only, one folded line until one of it is applied. A setting that needs root is refused without it with `sudo station settings apply <setting>`.

## Values

An item with inputs (`autologin`, `claude-token`) or an interactive procedure (`git-signing`, `ssh-key`, `github-login`) runs on a terminal only. A form asks each value, a password is never echoed, and `AGENT_PASSWORD` pre-fills the autologin password. Off a terminal the command exits 2 with `Hint: run it on a terminal`.

There is no flag to pass a value: a secret never travels in argv or shell history ([ADR-009](../decisions/009-one-bootstrap-verb-and-no-values-in-argv.md)).

## preferences.json

`~/.config/station/preferences.json` holds what station decides for itself:

```json
{
    "here": "macbook",
    "hosts": {
        "macbook": {},
        "mac-mini": { "user": "jterrazz.agent", "key": "~/.ssh/id_ed25519" }
    },
    "vm_user": "jterrazz",
    "lend_list": ["session:openpanel"],
    "user_email": "admin@jterrazz.com",
    "user_name": "Jean-Baptiste Terrazzoni",
    "services": { "max_per_vm": 10 }
}
```

`here` is this machine's name, `hosts` the other hosts it can drive, `vm_user` the one login of every VM in the fleet, `lend_list` the secrets that may be lent, `services` the policy of [published services](services.md#the-policy): `range` the host ports leases take, `max_per_vm` the leases one VM may hold, `ignored_ports` and `min_port` what is never published, `https` `port`, `path` or `off`; every `services` key is optional, the defaults being `40000-40999`, 20, `[22]`, 1024 and `port`. No secret is in it. A key no group names, such as the `tailnet` an earlier round wrote, is dropped at the next save and at setup's layout step. A `v` field names the version of the shape the file was written at; absent, it is version 1, and a file a newer station wrote is refused rather than read.

## Environment

`app` reads the environment once and passes typed options down; no other layer reads a variable.

| Variable                                                               | Read for                                                                                            |
| ---------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `AGENT_PASSWORD`                                                       | pre-fills the `autologin` form                                                                      |
| `OPENCLAW_GATEWAY_PORT`, `HERMES_GATEWAY_PORT`, `API_SERVER_PORT`      | the gateway ports `machines probe` checks                                                           |
| `MISE_DATA_DIR`                                                        | where mise keeps its shims                                                                          |
| `NO_COLOR`, `COLUMNS`                                                  | the output form and width                                                                           |
| `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME` | the folders station uses, when absolute ([Storage](storage.md#the-folders))                         |
| `GOENV`                                                                | where `station setup` writes Go's env file; `off` writes none                                       |
| `TART_HOME`                                                            | set by the tart launcher, never by the owner ([Storage](storage.md#tart))                           |
| `STATION_GITHUB_API_URL`                                               | the GitHub API origin, so a spec can point it at a stub                                             |
| `STATION_SYSTEM_DIRS`, `STATION_MACHINE_KIND`                          | which system folders count, and the machine kind, for a spec                                        |
| `STATION_SYSTEM_ROOT`                                                  | the root `/Applications`, `/opt/homebrew`, `/Library` and a VM's `/proc` are read under, for a spec |
| `STATION_POWER_STUB`                                                   | a folder whose files are the Power reading and whose calls go to `$HOME/.calls`, for a spec         |
| `STATION_GUEST_SSH_PORT`                                               | the port the engine dials a VM's sshd on (default 22), for a spec's stub sshd                       |
| `STATION_XDG_DETACHED`                                                 | internal: `setup`'s detached layout step                                                            |

## Related

- [Commands](commands.md): `station settings`.
- [Dotfiles](dotfiles.md): the files the dotfile items copy.
- [Footprint](footprint.md): every file station writes.
