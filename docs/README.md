# The manual of station

`station` is the CLI that sets up and runs a Mac and the fleet around it: it installs from a catalogue of tools and agent skills, configures the machine and its dotfiles, and creates, runs and deletes the VMs where development lives. This repository also authors two doctrines every `@jterrazz` repository follows, shipped to agents as skills. This page lists every page of the manual, by what the reader came to do.

## Start

| Page                                        | Answers                                                |
| ------------------------------------------- | ------------------------------------------------------ |
| [Getting started](start/getting-started.md) | From a fresh Mac to a named, configured host, in order |

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

| Page                                              | Answers                                                                                   |
| ------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| [Commands](reference/commands.md)                 | Every command and flag, the hidden plumbing, the exit codes                               |
| [The status contract](reference/status-json.md)   | The `station.status/v1` document `station status --json` prints and `studio doctor` reads |
| [Machines](reference/machines.md)                 | Hosts, VMs, lifetimes, `--on`, the tailnet join, ssh, launchd                             |
| [Images](reference/images.md)                     | The VM images, the recipe, the binary inside                                              |
| [Secrets](reference/secrets.md)                   | Kept, lent and bound secrets; Claude and studio in a VM                                   |
| [Configuration](reference/configuration.md)       | The config items, `settings.json`, the environment variables                              |
| [Tools and skills](reference/tools-and-skills.md) | The catalogue, the method table, mise, skills as tools, the toolbelt                      |
| [Dotfiles](reference/dotfiles.md)                 | The embedded application configs and the `~/.zshrc` block                                 |
| [Footprint](reference/footprint.md)               | Everything `station` writes on a machine                                                  |

## Decisions

The records of decisions this repository alone took go in [`decisions/`](decisions/), numbered in the order they were taken, the mold `_template.md` beside them. A decision spanning several repositories is recorded by the corpus that spans them. What was left for later is `TODO.md` at the repository root.
