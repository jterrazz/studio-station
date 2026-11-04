# Images

An image is a versioned, read-only VM disk that VMs are cloned from. Each Mac holds its own: built there from a base with a recipe embedded in the binary, or fetched where it is published. The catalogue has two images: `linux`, built or fetched, and `macos`, fetched only (station has no recipe for it).

| Section           | Answers                                                    |
| ----------------- | ---------------------------------------------------------- |
| Listing           | `station images`, every Mac's                              |
| Building          | `station images build`, the base and the version           |
| Updating          | `station images update`, the published versions            |
| The recipe        | What the image contains, and the two sessions that make it |
| The binary inside | `--binary` and `make build-linux`                          |
| Cleaning          | `images delete`; Tart's pull cache is `storage clean tart` |

## Listing

```sh
station images                  # ●  linux  2026.11.01  4.0 GB  jterrazz-macbook-pro  api · linux-1
station images --on mac-mini    # that Mac's alone
station images --json           # station.images.list/v1: name, version, size_bytes, mac, vms, newest
```

One row per version per Mac, This Mac first, then every other Mac that answers; an image no Mac holds is one row, `not built yet`. Images live on a Mac: inside a VM the family is refused (exit 2).

## Building

```sh
station images build            # every image that is missing or whose recipe changed
station images build linux
```

The base of `linux` is `ghcr.io/cirruslabs/ubuntu`, pulled once (about 5 GB) and cloned for the build. The version is the UTC date, `2026.11.07`, with `.2` for a second build the same day. Station records the hash of the recipe files, so a changed recipe is rebuilt by a bare `build`. Images need Tart: `station apps install tart`.

## Updating

```sh
station images update           # every image this Mac holds
station images update linux     # ●  linux  2026.11.03  fetched
```

The images are published at `ghcr.io/jterrazz/station-<image>`, one tag per version. An update asks the registry for the newest version, and fetches it only when it is newer than every version held: `is the newest` otherwise. The fetched version lands whole as an image disk beside the others, or not at all: a registry that does not answer, or a fetch that fails, exits 1 and leaves the image as it was. Like a build, it changes only the next VMs.

## The recipe

The recipes are shell files embedded under `src/catalog/images/`. The build copies the `station` binary into the build VM, then runs two sessions inside it over the NAT, each over station's own ssh client: the first by the base image's password, the second by the image key the first installs:

1. as the base's own user, `linux.sh` creates `vm_user` with passwordless sudo and the host's image key, installs the packages, Docker, mise, the `station` binary, lets sshd carry the engine's forwards (`/etc/ssh/sshd_config.d/60-station.conf`: `AllowTcpForwarding local`), runs `station setup` for the shell block, installs two systemd units: `station-intent` (tells the host about a poweroff, see [Machines](machines.md#lifetimes)) and `station-hostkeys` (generates ssh host keys at first boot, ordered before `ssh.socket` outside the default dependencies, which would make a cycle), and masks the getty on `/dev/hvc0`, since tart reads the console file back into the guest as input;
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
