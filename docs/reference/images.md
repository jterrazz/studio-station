# Images

An image is a versioned, read-only VM disk that VMs are cloned from. Each host builds its own, from a base, with a recipe embedded in the binary. This build has one image, `linux`.

| Section           | Answers                                                    |
| ----------------- | ---------------------------------------------------------- |
| Building          | `station images build`, the base and the version           |
| The recipe        | What the image contains, and the two sessions that make it |
| The binary inside | `--binary` and `make build-linux`                          |
| Cleaning          | `images delete`; Tart's pull cache is `storage clean tart` |

## Building

```sh
station images build            # every image that is missing or whose recipe changed
station images build linux
station images                  # ●  linux  2026.11.07  4.9 GB  0 VMs
```

The base of `linux` is `ghcr.io/cirruslabs/ubuntu`, pulled once (about 5 GB) and cloned for the build. The version is the UTC date, `2026.11.07`, with `.2` for a second build the same day. Station records the hash of the recipe files, so a changed recipe is rebuilt by a bare `build`. Images need Tart: `station tools install tart`.

## The recipe

The recipes are shell files embedded under `src/catalog/images/`. The build runs two ssh sessions over the NAT:

1. as the base's own user, `linux.sh` creates `vm_user` with passwordless sudo and the host's image key, installs the packages, Docker, mise, the `station` binary, lets sshd carry the daemon's forwards (`/etc/ssh/sshd_config.d/60-station.conf`: `AllowTcpForwarding local`), runs `station setup` for the shell block, installs two systemd units: `station-intent` (tells the host about a poweroff, see [Machines](machines.md#lifetimes)) and `station-hostkeys` (generates ssh host keys at first boot, ordered before `ssh.socket` outside the default dependencies, which would make a cycle), and masks the getty on `/dev/hvc0`, since tart reads the console file back into the guest as input;
2. as `vm_user`, `linux-cleanup.sh` stops the base's `tart-guest-agent` (it runs as `admin`), removes the base's user and fails the build if it cannot, empties the machine id and the host keys, and powers off.

A VM's first boot sets its own name and machine id and grows the disk. No tailscale is in the image: a VM is reached through its host ([Machines](machines.md#ssh-and-herdr)).

## The binary inside

The image carries `station-linux-arm64`. A release build downloads the asset of its own version into `~/.cache/station/` and checks it against the release's `checksums.txt`. A development build has no release, so `make build-linux` makes the binary and you pass it:

```sh
make build-linux
station images build linux --binary .artifacts/go/station-linux-arm64
```

## Cleaning

`station images delete <image>[@<version>]` never touches a VM: a VM is an APFS clone, independent of its image once made. Bare `station images` marks every version older than the newest. Deleting never prunes Tart's cache of the base: that cache is Tart's, emptied by `station storage clean tart` ([Storage](storage.md)), and the next build pulls the base again after it.

## Related

- [Machines](machines.md): creating a VM from an image.
- [Footprint](footprint.md): the image records and the image key.
