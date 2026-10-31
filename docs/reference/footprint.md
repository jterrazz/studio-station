# Footprint

Everything `station` writes on a machine, by where it lives. Binaries, config, state and caches follow the estate's XDG rule; nothing lives in a clone.

| Section             | Answers                                |
| ------------------- | -------------------------------------- |
| The XDG folders     | The files station owns                 |
| Do not delete       | The two files nothing can rebuild      |
| Outside the folders | What station writes elsewhere, and why |

## The XDG folders

Each path honours its `XDG_*_HOME` variable when it is set and absolute.

| Path                                                   | Holds                                                                                                            |
| ------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------- |
| `~/.local/bin/station`                                 | the binary                                                                                                       |
| `~/.config/station/settings.json`                      | this machine's name, the hosts, the tailnet choice, `vm_user`, the lend list ([Configuration](configuration.md)) |
| `~/.local/state/station/vms/<name>.json`               | one record per VM on this host                                                                                   |
| `~/.local/state/station/known_hosts/<name>`            | one machine's ssh host key, deleted with the VM                                                                  |
| `~/.local/state/station/images/<image>/<version>.json` | image records ([Images](images.md))                                                                              |
| `~/.local/state/station/images/key`, `key.pub`         | the image key                                                                                                    |
| `~/.local/state/station/tailscaled/`                   | the userspace tailscaled's state, socket and log                                                                 |
| `~/.local/state/station/dns/`                          | the generated DNS profile                                                                                        |
| `~/.local/state/station/logs/`                         | each VM supervisor's log, and `setup.log`                                                                        |
| `~/.local/state/station/consoles/<vm>`                 | a VM's console file, read by its supervisor                                                                      |
| `~/.cache/station/`                                    | downloaded `station-linux-arm64` binaries                                                                        |
| keychain, service `station`                            | kept secrets: `vm-join-id`, `vm-join-secret`, `tailnet-authkey`                                                  |

`~/.studio/` is the old home of these files; `station setup` moves them and removes it, and station reads only the new paths.

## Do not delete

Two files are do-not-delete. The VM records alone know each VM's lifetime and lent names, and the image key is the only way into a fresh VM. Neither can be derived again; everything else in the table can.

## Outside the folders

- `~/Library/LaunchAgents/com.jterrazz.station.vm.<name>.plist`: one launchd agent per VM, macOS's own place for it ([Machines](machines.md#lifetimes)).
- `~/.tart`: Tart's storage, which station does not move; its images are named `station-image-<image>-<version>`.
- The managed block of `~/.zshrc` ([Dotfiles](dotfiles.md)).
- One managed `Host` block per machine of the fleet in `~/.ssh/config`, between `# >>> jterrazz machine <name> >>>` markers; removing a machine gives the file back as it was.
- The files each dotfile config item copies ([Dotfiles](dotfiles.md)), and mise's global `~/.config/mise/config.toml`.

## Related

- [Release](../guides/release.md): how the binary reaches `~/.local/bin`.
