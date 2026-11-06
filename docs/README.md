# The manual of station

`station` is the CLI that sets up and runs a Mac and every machine around it: it installs from a catalogue of apps and agent skills, configures the machine and its dotfiles, and creates, runs and deletes the VMs where development lives. This repository also authors two doctrines every `@jterrazz` repository follows, shipped to agents as skills. This page lists every page of the manual, by what the reader came to do.

## Start

| Page                                        | Answers                                           |
| ------------------------------------------- | ------------------------------------------------- |
| [Getting started](start/getting-started.md) | From a fresh Mac to a named, set-up Mac, in order |

## Guides

| Page                         | Answers                                                                            |
| ---------------------------- | ---------------------------------------------------------------------------------- |
| [Develop](guides/develop.md) | Working from source: the gestures, which folder a change opens, what a change owes |
| [Test](guides/test.md)       | The suites, the spec documents, the stubs                                          |
| [Release](guides/release.md) | The tagged release, how a machine takes it, provisioning from a checkout           |

## Concepts

| Page                                         | Answers                                                                                       |
| -------------------------------------------- | --------------------------------------------------------------------------------------------- |
| [Architecture](concepts/architecture.md)     | The layers, the import table, the path of a request, the method table                         |
| [Design system](concepts/design-system.md)   | The palette, the marks, the pipe format, the TUI                                              |
| [Glossary](concepts/glossary.md)             | The one word for each concept, and the words rejected                                         |
| [The stack](concepts/stack.md)               | How `@jterrazz` projects compose: packages, naming, required files; shipped as `studio-stack` |
| [Repo structure](concepts/repo-structure.md) | Where knowledge lives in every `@jterrazz` repository; shipped as `studio-repo-structure`     |

## Reference

| Page                                            | Answers                                                                                   |
| ----------------------------------------------- | ----------------------------------------------------------------------------------------- |
| [Commands](reference/commands.md)               | Every command and flag, the hidden plumbing, the exit codes                               |
| [The status contract](reference/status-json.md) | The `station.status/v1` document `station status --json` prints and `studio doctor` reads |
| [Machines](reference/machines.md)               | Hosts, VMs, lifetimes, `--on`, the tailnet, ssh, the engine, the round-5 migration        |
| [Ports](reference/ports.md)                     | What a VM listens on: reserved ports, sharing, HTTPS, profiles, the engine protocol       |
| [Images](reference/images.md)                   | The VM images, the recipe, the binary inside                                              |
| [Secrets](reference/secrets.md)                 | Kept, shared and bound secrets; sharing accounts with a VM; Claude and studio in a VM     |
| [Configuration](reference/configuration.md)     | The settings, `preferences.json`, the environment variables                               |
| [Apps and skills](reference/apps-and-skills.md) | The catalogue, the method table, mise, skills as apps, the toolbelt                       |
| [Dotfiles](reference/dotfiles.md)               | The embedded application configs and the `~/.zshrc` block                                 |
| [Disk](reference/disk.md)                       | The Disk groups, the relocation rule, `station disk`, free, erase and move                |
| [Footprint](reference/footprint.md)             | Everything `station` and the apps write on a machine, generated from the table            |

## Decisions

The records of decisions this repository alone took go in [`decisions/`](decisions/), numbered in the order they were taken, the mold `_template.md` beside them.

- [001 station mirrors the studio CLI's layers](decisions/001-station-mirrors-the-studio-cli-layers.md)
- [002 brew bundle installs, native commands uninstall](decisions/002-brew-bundle-installs-native-commands-uninstall.md)
- [003 mise owns the runtimes](decisions/003-mise-owns-the-runtimes.md)
- [004 station status --json is a contract](decisions/004-station-status-json-is-a-contract.md)
- [005 The version is stamped from the tag](decisions/005-the-version-is-stamped-from-the-tag.md)
- [006 Tart is the engine, behind a port](decisions/006-tart-is-the-engine-behind-a-port.md)
- [007 Files live in the XDG folders](decisions/007-files-live-in-the-xdg-folders.md)
- [008 Every VM has a lifetime](decisions/008-every-vm-has-a-lifetime.md)
- [009 One bootstrap verb, and no values in argv](decisions/009-one-bootstrap-verb-and-no-values-in-argv.md)
- [010 VMs are reached through their Mac](decisions/010-vms-are-reached-through-their-mac.md)
- [011 One engine per Mac](decisions/011-one-engine-per-mac.md)
- [012 Ports are reserved and shared by their Mac](decisions/012-ports-are-reserved-and-shared.md)
- [013 Every port leaves through Serve](decisions/013-every-port-leaves-through-serve.md)

A decision spanning several repositories is recorded by the corpus that spans them. What was left for later is `TODO.md` at the repository root.
