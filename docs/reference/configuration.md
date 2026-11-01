# Configuration

`station config` sets a machine up: dotfiles, logins and macOS defaults. Each entry is a config item, named by what it sets, and `station`'s own file is `settings.json`.

| Section       | Answers                                        |
| ------------- | ---------------------------------------------- |
| The items     | Every config item, its place and what it does  |
| Values        | How an item that needs values asks for them    |
| settings.json | The file station keeps its own choices in      |
| Environment   | The variables station reads, all resolved once |

## The items

An item lists only on a machine whose kind is in its place ([Glossary](../concepts/glossary.md)); asked for elsewhere it is refused with exit 2. Only reversible items uninstall.

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
| `tailnet-auth`              | host        | how this host authenticates to the tailnet: login or auth key                                         | yes        |
| `vm-join`                   | host        | the OAuth client that lets this host mint VM auth keys ([Machines](machines.md#vm-join))              | yes        |
| `claude-token`              | host        | the Claude Code token lent to VMs ([Secrets](secrets.md#claude-in-a-vm))                              | yes        |

Items group by category: Terminal, Security, Editor, System, Network, and Headless (`autologin`, `power`, `lock-after-login` and `sshd`: what an unattended host needs). An item that needs root is refused without it with `sudo station config install <item>`.

## Values

An item with inputs (`tailnet-auth`, `vm-join`, `autologin`, `claude-token`) or an interactive procedure (`git-signing`, `ssh-key`, `github-login`) runs on a terminal only. A form asks each value, a password is never echoed, and `AGENT_PASSWORD` pre-fills the autologin password. Off a terminal the command exits 2 with `Hint: run it on a terminal`.

There is no flag to pass a value: a secret never travels in argv or shell history ([ADR-009](../decisions/009-one-bootstrap-verb-and-no-values-in-argv.md)).

## settings.json

`~/.config/station/settings.json` holds what station decides for itself:

```json
{
    "here": "macbook",
    "hosts": {
        "macbook": {},
        "mac-mini": { "user": "jterrazz.agent", "key": "~/.ssh/id_ed25519" }
    },
    "tailnet": { "mode": "userspace", "auth_method": "login" },
    "vm_user": "jterrazz",
    "lend_list": ["session:openpanel"],
    "user_email": "admin@jterrazz.com",
    "user_name": "Jean-Baptiste Terrazzoni"
}
```

`here` is this machine's name, `hosts` the other hosts it can drive, `vm_user` the one login of every VM in the fleet, `lend_list` the secrets that may be lent. No secret is in it: those are in the keychain ([Footprint](footprint.md)).

## Environment

`app` reads the environment once and passes typed options down; no other layer reads a variable.

| Variable                                                               | Read for                                                                            |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| `AGENT_PASSWORD`                                                       | pre-fills the `autologin` form                                                      |
| `OPENCLAW_GATEWAY_PORT`, `HERMES_GATEWAY_PORT`, `API_SERVER_PORT`      | the gateway ports `machines probe` and `status` check                               |
| `MISE_DATA_DIR`                                                        | where mise keeps its shims                                                          |
| `NO_COLOR`, `COLUMNS`                                                  | the output form and width                                                           |
| `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME` | the folders station uses, when absolute ([Storage](storage.md#the-folders))         |
| `GOENV`                                                                | where `station setup` writes Go's env file; `off` writes none                       |
| `TART_HOME`                                                            | set by the tart launcher, never by the owner ([Storage](storage.md#tart))           |
| `STATION_GITHUB_API_URL`, `STATION_TAILSCALE_API_URL`                  | the API origins, so a spec can point them at a stub                                 |
| `STATION_SYSTEM_DIRS`, `STATION_MACHINE_KIND`                          | which system folders count, and the machine kind, for a spec                        |
| `STATION_SYSTEM_ROOT`                                                  | the root `/Applications`, `/opt/homebrew` and `/Library` are read under, for a spec |
| `STATION_VM_AGENT`                                                     | internal: set in a VM's launchd agent                                               |
| `STATION_XDG_DETACHED`                                                 | internal: `setup`'s detached layout step                                            |

## Related

- [Commands](commands.md): `station config`.
- [Dotfiles](dotfiles.md): the files the dotfile items copy.
- [Footprint](footprint.md): every file station writes.
