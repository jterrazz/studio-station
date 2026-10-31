# station

A single CLI to set up and run a Mac and the fleet around it: tools, config, and the VMs where development lives. No sudo required for the install.

## Install

**Fresh machine** (no Go needed):

```sh
xcode-select --install
curl -fsSL https://raw.githubusercontent.com/jterrazz/studio-station/main/scripts/install.sh | sh
exec zsh
station setup --name macbook
```

The binary lives at `~/.local/bin/station`; its files follow the XDG folders. The first steps after it are [Getting started](docs/start/getting-started.md); what it writes is [Footprint](docs/reference/footprint.md), and the release it comes from [Release](docs/guides/release.md).

## What it does

```sh
station                 # on a terminal: the TUI (Status, Tools, Config, Clean, Fleet)

# Here
station status          # everything about this machine
station tools           # install, uninstall, upgrade tools and agent skills
station config          # dotfiles, logins and macOS defaults
station clean           # reclaim disk, with sizes

# The fleet
station machines        # hosts and VMs: create, start, stop, delete, probe
station images          # the VM images
station secrets         # what may be lent to a VM
station tailnet         # this machine on the tailnet

# Shortcuts
station run git         # commit, push, sync
station run docker      # remove containers and images

# The binary
station setup           # register, wire the shell, migrate
```

Every command is in [Commands](docs/reference/commands.md); `--on <machine>` runs any of them elsewhere.

## Documentation

The corpus lives in [`docs/`](docs/README.md); start at its map, which lists every page. The words are in the [Glossary](docs/concepts/glossary.md).

For agents: start at [`CLAUDE.md`](CLAUDE.md). The [`skills/`](skills/) directory ships the shared skills: `studio-toolbelt` (the machine's tool roster, generated from the catalogue), `studio-stack`, `studio-new-project` and `studio-repo-structure`. What is left for later is [TODO.md](TODO.md).

## License

MIT © [Jean-Baptiste Terrazzoni](https://github.com/jterrazz)
