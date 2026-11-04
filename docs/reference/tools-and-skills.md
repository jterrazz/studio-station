# Tools and skills

A tool is installed content: software or an agent skill. The catalogue in `src/catalog/` declares each one as a value, and `station tools`, `station status` and the toolbelt skill all read it.

| Section         | Answers                                                    |
| --------------- | ---------------------------------------------------------- |
| The catalogue   | How a tool is declared, and its place                      |
| Methods         | How each method installs, uninstalls, upgrades and is read |
| Runtimes        | Why mise owns node, go, python, bun, rust and java         |
| Skills as tools | Agent skills installed with `station tools install`        |
| The toolbelt    | The committed projection agents read                       |

## The catalogue

A tool has a name, a description, a category, its install per kind of machine, the commands it provides and what it requires. Its place, the kinds it may live on (`host`, `linux`, `macos`), is derived from its installs, so it cannot disagree with them. Dev tools live in VMs: Xcode, CocoaPods, XcodeGen, Android Studio and Lens are placed `macos` only and install nowhere until the macOS VM exists ([TODO](../../TODO.md)); Ollama stays on hosts for the GPU; Docker lives in the Linux VM.

A tool asked for on a machine outside its place is refused with exit 2, naming where it lives and, when a VM is running, the command with `--on`.

A tool's state is one of `ok` (the method's manager lists it), `degraded` (found on PATH or as an app but not managed by its method), `broken`, `absent` or `unknown` (the manager did not answer). `Requires` orders an install and refuses one whose requirement is missing.

## Methods

One registry, `Installers` in `src/apps/contract/`, holds an installer per method, each its own adapter under `src/apps/adapter/<method>/` with its commands and its parser; nothing else switches on a method. `wire` builds the registry; `domain/tools` asks each installer what it can do.

| Method                | Install                                                            | Uninstall                               | Upgrade                          | Kinds       |
| --------------------- | ------------------------------------------------------------------ | --------------------------------------- | -------------------------------- | ----------- |
| `brew`, `cask`, `mas` | `brew bundle` (Brewfile on stdin)                                  | `brew uninstall`, `mas uninstall`       | `brew bundle --upgrade`          | host, macos |
| `npm`, `uv`           | `brew bundle`                                                      | `npm uninstall -g`, `uv tool uninstall` | `brew bundle --upgrade`          | host, macos |
| `bun`                 | `bun install -g`                                                   | `bun remove -g`                         | `bun update -g`                  | all         |
| `mise`                | `mise use --global <tool>@<version>`                               | `mise unuse --global`                   | `mise upgrade`                   | all         |
| `script`, `shell`     | the vendor's installer                                             | not reversible                          | rerun the script; none for shell | all         |
| `app`                 | manual: the refusal names how                                      | not reversible                          | none                             | host, macos |
| `skill`               | `skills add <repo> -g -y --skill <name> --agent claude-code codex` | `skills remove -g -y <name>`            | the install again                | all         |

`brew bundle` installs a whole request in one call and taps a trusted formula itself; the catalogue's `trusted` taps are the owner's act of trust ([ADR-002](../decisions/002-brew-bundle-installs-native-commands-uninstall.md)). `station tools upgrade` upgrades catalogue tools only; `--via <manager>` selects every installed tool of a manager (`brew` covers brew and cask).

## Superseded installs and App Store copies

A catalogue tool can name the formula it `Supersedes`: with the formula alone the tool is `◐ formula installed, app absent`, and once the tool itself is `ok` `station setup` uninstalls the formula. An `AppStore` tool whose bundle carries the App Store's receipt (`Contents/_MASReceipt`) is `ok` through that copy, unmanaged, detail `app-store`: `tools install` says it is already installed, `tools upgrade` leaves it to the App Store (`○ the App Store updates it` when named), and `tools uninstall` refuses it (`Hint: remove it from /Applications`). Tailscale is both ([The tailnet](machines.md#the-tailnet)).

## Runtimes

mise owns the language runtimes on every machine ([ADR-003](../decisions/003-mise-owns-the-runtimes.md)). `station tools install node` writes mise's global config and installs; a machine that still has node from elsewhere shows it `degraded`, "on PATH, not managed by mise", and the repair is that same command. `station shell zsh` activates mise in every shell ([Dotfiles](dotfiles.md)).

## Skills as tools

An agent skill is a tool whose method is `skill`, in the category "Agent skills", installed through the `skills` CLI, which `station` requires like it requires Homebrew for a cask. There is no separate family:

```sh
station tools install frontend-design better-ui
station tools                       # skills are the last group; ◐ means upstream moved
station tools uninstall shadcn
```

Only community skills are in the catalogue. The `@jterrazz` skills are not curated here: `studio doctor` derives them from the repositories the estate declares. Two skills share a name with a tool, so they carry a `-skill` suffix: `qmd-skill` and `agent-browser-skill`. A skill is installed for Claude Code (`~/.claude/skills`) and the shared `~/.agents/skills` that codex and the other universal agents read, never for every agent the skills CLI knows, which it does on a home where it detects none. Installed state is read from the `skills` lock file `~/.agents/.skill-lock.json`; currency is compared against GitHub with a 5 second budget and is `unknown` on failure.

## The toolbelt

[`skills/studio-toolbelt/`](../../skills/studio-toolbelt/SKILL.md) tells an agent on a studio machine what is available. The sections between its `GENERATED` markers are compiled from the catalogue by `make generate` (`src/cmd/generate/`), never edited by hand, and a test run by `make test` fails when the catalogue and the skill drift. Change the catalogue, run `make generate`, commit both together.

## Related

- [Commands](commands.md): `station tools`.
- [Architecture](../concepts/architecture.md): why the catalogue is values and the method table is one.
