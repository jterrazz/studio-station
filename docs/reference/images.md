# Images

An image is a versioned, read-only VM disk that VMs are cloned from. Each host builds its own, from a base, with a recipe embedded in the binary. This build has one image, `linux`.

| Section           | Answers                                                    |
| ----------------- | ---------------------------------------------------------- |
| Building          | `station images build`, the base and the version           |
| The recipe        | What the image contains, and the two sessions that make it |
| The binary inside | `--binary` and `make build-linux`                          |
| Cleaning          | `images delete` and `clean old-images`                     |

## Building

```sh
station images build            # every image that is missing or whose recipe changed
station images build linux
station images                  # ●  linux  2026.11.07  4.9 GB  0 VMs
```

The base of `linux` is `ghcr.io/cirruslabs/ubuntu`, pulled once (about 5 GB) and cloned for the build. The version is the UTC date, `2026.11.07`, with `.2` for a second build the same day. Station records the hash of the recipe files, so a changed recipe is rebuilt by a bare `build`. Images need Tart: `station tools install tart`.

## The recipe

The recipes are shell files embedded under `src/catalog/images/`. The build runs two ssh sessions over the NAT:

1. as the base's own user, `linux.sh` creates `vm_user` with passwordless sudo and the host's image key, installs the packages, Docker, Tailscale, mise, the `station` binary, runs `station setup` for the shell block, and installs two systemd units: `station-intent` (tells the host about a poweroff, see [Machines](machines.md#lifetimes)) and `station-hostkeys` (generates ssh host keys at first boot, ordered before `ssh.socket` outside the default dependencies, which would make a cycle);
2. as `vm_user`, `linux-cleanup.sh` stops the base's `tart-guest-agent` (it runs as `admin`), removes the base's user and fails the build if it cannot, empties the machine id and the host keys, and powers off.

A VM's first boot sets its own name and machine id, grows the disk and joins the tailnet.

## The binary inside

The image carries `station-linux-arm64`. A release build downloads the asset of its own version into `~/.cache/station/` and checks it against the release's `checksums.txt`. A development build has no release, so `make build-linux` makes the binary and you pass it:

```sh
make build-linux
station images build linux --binary .artifacts/go/station-linux-arm64
```

## Cleaning

`station images delete <image>[@<version>]` and `station clean old-images` never touch a VM: a VM is an APFS clone, independent of its image once made. `clean old-images` also prunes Tart's cache of the base, which the next build pulls again.

## Related

- [Machines](machines.md): creating a VM from an image.
- [Footprint](footprint.md): the image records and the image key.
