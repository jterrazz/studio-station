# Getting started

From a fresh Mac to a machine `station` knows: the one-line install, naming the machine, then tools and config.

## 1. Install `station`

```sh
xcode-select --install
curl -fsSL https://raw.githubusercontent.com/jterrazz/studio-station/main/scripts/install.sh | sh
exec zsh
```

The script reads the latest release tag, downloads `station-<os>-<arch>`, checks it against the release's `checksums.txt`, installs it to `~/.local/bin/station` and runs `station setup` on the terminal, so setup asks this Mac's name. There is no clone: the dotfiles are in the binary. Running the script again is the upgrade.

## 2. Name this machine

```sh
station setup --name macbook
```

Where no terminal can be asked (a script, an ssh command without a terminal), setup on a Mac with no name yet is refused, exit 2, before any step: `--name` is how such a run names it; the install stands. `setup` names this Mac, writes the managed block of `~/.zshrc`, moves an old `~/.studio/` to the XDG folders, starts the engine, installs the missing system apps and applies the settings that need nobody; the others it lists as next steps. It is safe to run again ([Dotfiles](../reference/dotfiles.md)).

## 3. Tools, then config

```sh
station apps install homebrew tailscale tart
station settings apply ghostty-dotfiles zed-dotfiles hushlogin
station overview
```

Open the Tailscale app once and log in: station only reads it. A config item that needs values (a password, a token) opens a form on a terminal ([Configuration](../reference/configuration.md)). A host stays clean: dev work lives in VMs ([Machines](../reference/machines.md)), and Claude Code reaches them with a lent token ([Secrets](../reference/secrets.md#claude-in-a-vm)).

Every command is in [Commands](../reference/commands.md); every file written is in [Footprint](../reference/footprint.md).
