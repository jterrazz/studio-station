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
station                 # on a terminal: the TUI (Home · Tools · Config · Storage │ Fleet · Services · Accounts · Power)

# Here
station overview        # everything about this machine
station apps            # install, update, remove apps and agent skills
station settings        # dotfiles, logins and macOS defaults
station disk            # what each app keeps on disk: free what rebuilds, erase data
station power           # keep this host awake for a while, the lid closed too

# The fleet
station machines        # hosts and VMs: create, start, stop, delete, probe
station ports           # what the VMs share on the tailnet, and each URL
station accounts        # who has access to what, here and in the VMs: never a token
station images          # the VM images
station secrets         # what may be lent to a VM
station engine         # this Mac's engine: running, its version, the VMs it runs
station engine stop    # stop everything on this Mac: its VMs (disks kept), Awake, then the engine
station engine start   # start the engine again; the VMs it stopped stay stopped

# The binary
station setup           # register, wire the shell, migrate
```

Every command is in [Commands](docs/reference/commands.md); `--on <machine>` runs any of them elsewhere.

The menu bar app, `apps/menubar`, shows the fleet, the To do and Power from this host's engine. `station setup` never installs it: `make menubar-install` copies it to `~/Applications`, and [Develop](docs/guides/develop.md) holds its targets.

## Documentation

The corpus lives in [`docs/`](docs/README.md); start at its map, which lists every page. The words are in the [Glossary](docs/concepts/glossary.md).

For agents: start at [`CLAUDE.md`](CLAUDE.md). The [`skills/`](skills/) directory ships the shared skills: `studio-toolbelt` (the machine's tool roster, generated from the catalogue), `studio-stack`, `studio-new-project` and `studio-repo-structure`. What is left for later is [TODO.md](TODO.md).

## License

MIT © [Jean-Baptiste Terrazzoni](https://github.com/jterrazz)
