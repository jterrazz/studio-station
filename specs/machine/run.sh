#!/bin/bash
# outside: GitHub's macOS runners are VMs and nest none, so only
# `make e2e-machine KIND=linux|macos` runs this, on a host, never `make check` nor CI.
# invariant: every VM and image made here is named station-e2e-*, outside station's
# ownership rule, so the product never lists nor touches them.
set -uo pipefail

kind=${1:-}
case "$kind" in
  linux | macos) ;;
  *) echo "usage: make e2e-machine KIND=linux|macos" >&2; exit 2 ;;
esac

repo=$(cd "$(dirname "$0")/../.." && pwd)
suite="$repo/specs/machine"
out="$repo/.artifacts/e2e"
log="$out/$kind.log"
work="$out/$kind"
mkdir -p "$out"
rm -rf "$work"
mkdir -p "$work"
: > "$log"

say() { printf '%s\n' "$*" | tee -a "$log"; }
fail() { say "FAIL $*"; exit 1; }

tart_bin="$HOME/.local/bin/tart"
[ -x "$tart_bin" ] || tart_bin=$(command -v tart || true)
[ -n "$tart_bin" ] || fail "tart is not installed: station tools install tart"
tart() { "$tart_bin" "$@"; }

run_id="$$"
made=()

# invariant: whatever happens, every VM this run made is stopped and deleted; only
# the macOS image cache survives, and it is never in `made`.
cleanup() {
  local vm
  for vm in "${made[@]:-}"; do
    [ -n "$vm" ] || continue
    tart stop "$vm" >/dev/null 2>&1 || true
    tart delete "$vm" >/dev/null 2>&1 || true
  done
  local left
  left=$(tart list --quiet --source local 2>/dev/null | grep "^station-e2e-$kind-$run_id" || true)
  [ -z "$left" ] || say "WARN left behind: $left"
}
trap cleanup EXIT
trap 'exit 130' INT TERM


running=$(tart list --format json 2>/dev/null | jq -r '.[] | select(.State == "running" or .Running == true) | .Name')
if [ "$kind" = macos ] && [ -n "$running" ]; then
  fail "a VM is running ($running): tier 3 runs one macOS VM at 8 GB with no other VM"
fi

ssh_key="$work/id_ed25519"
ssh-keygen -q -t ed25519 -N '' -C "station-e2e" -f "$ssh_key" || fail "ssh-keygen"
known="$work/known_hosts"
ssh_opts=(-o StrictHostKeyChecking=no -o UserKnownHostsFile="$known" -o LogLevel=ERROR -o ConnectTimeout=5 -o ServerAliveInterval=15)

if [ "$kind" = linux ]; then
  base=$(tart list --quiet --source local | grep '^station-image-linux-' | sort | tail -1)
  [ -n "$base" ] || fail "no product linux image: station images build"
  user=$(sed -n 's/.*"vm_user": *"\([^"]*\)".*/\1/p' "${XDG_CONFIG_HOME:-$HOME/.config}/station/preferences.json" 2>/dev/null)
  user=${user:-$USER}
  image_key="${XDG_STATE_HOME:-$HOME/.local/state}/station/images/key"
  [ -f "$image_key" ] || fail "no image key at $image_key: station images build"
  memory=4096
else
  hash=$(cd "$repo" && find src/catalog -type f | LC_ALL=C sort | xargs cat | shasum -a 256 | cut -c1-12)
  base="station-e2e-macos-$hash"
  user="admin"
  memory=8192
fi
say "== tier $([ "$kind" = linux ] && echo 2 || echo 3): $kind, base $base"

# value: a fresh macOS guest knows only admin/admin; the first ssh answers the
# password through SSH_ASKPASS and installs this run's key, every later ssh uses it.
askpass="$work/askpass"
printf '#!/bin/sh\necho admin\n' > "$askpass"
chmod 700 "$askpass"

boot() {
  local vm=$1 ip
  tart run --no-graphics "$vm" >> "$work/$vm.tart.log" 2>&1 &
  ip=$(tart ip "$vm" --wait 180) || return 1
  local n=0
  if [ "$kind" = linux ]; then
    until ssh "${ssh_opts[@]}" -o BatchMode=yes -i "$image_key" "$user@$ip" true 2>/dev/null; do
      n=$((n + 1)); [ $n -lt 60 ] || return 1; sleep 2
    done
    ssh "${ssh_opts[@]}" -o BatchMode=yes -i "$image_key" "$user@$ip" \
      "mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys" < "$ssh_key.pub" || return 1
  else
    until SSH_ASKPASS="$askpass" SSH_ASKPASS_REQUIRE=force DISPLAY=none \
      ssh "${ssh_opts[@]}" -o PubkeyAuthentication=no "$user@$ip" \
      "mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys" < "$ssh_key.pub" 2>/dev/null; do
      n=$((n + 1)); [ $n -lt 60 ] || return 1; sleep 2
    done
  fi
  echo "$ip"
}

if [ "$kind" = macos ] && ! tart list --quiet --source local | grep -qx "$base"; then
  say "-- prebaking $base from ghcr.io/cirruslabs/macos-tahoe-base:latest"
  for old in $(tart list --quiet --source local | grep '^station-e2e-macos-' | grep -v -- "-$run_id"); do
    tart delete "$old" >/dev/null 2>&1 || true
  done
  bake="station-e2e-macos-bake-$run_id"
  made+=("$bake")
  tart clone ghcr.io/cirruslabs/macos-tahoe-base:latest "$bake" >> "$log" 2>&1 || fail "pull the macOS base"
  ip=$(boot "$bake") || fail "boot $bake"
  ssh "${ssh_opts[@]}" -i "$ssh_key" "admin@$ip" 'bash -s' < "$suite/macos/prebake.sh" >> "$log" 2>&1 || fail "prebake"
  ssh "${ssh_opts[@]}" -i "$ssh_key" "admin@$ip" 'rm -f ~/.ssh/authorized_keys' || true
  tart stop "$bake" >> "$log" 2>&1
  tart rename "$bake" "$base" >> "$log" 2>&1 || fail "keep $base"
  made=()
fi


vm="station-e2e-$kind-$run_id"
made+=("$vm")
tart clone "$base" "$vm" >> "$log" 2>&1 || fail "clone $base"
tart set "$vm" --memory "$memory" >> "$log" 2>&1 || fail "size $vm"
ip=$(boot "$vm") || fail "boot $vm"
say "-- $vm at $ip"

env_prefix=""
[ "$kind" = macos ] && env_prefix="export STATION_MACHINE_KIND=host;"
on() { ssh -n "${ssh_opts[@]}" -o BatchMode=yes -i "$ssh_key" "$user@$1" "$env_prefix $2"; }
in_vm() { on "$ip" "$1"; }

if [ "$kind" = macos ]; then
  say "kern.hv_vmm_present: $(in_vm 'sysctl -n kern.hv_vmm_present' 2>&1)"
fi

if [ "$kind" = linux ]; then
  binary="$repo/.artifacts/go/station-linux-arm64"
else
  binary="$repo/.artifacts/go/station"
fi
[ -x "$binary" ] || fail "no binary at $binary: make build build-linux"
in_vm 'rm -rf /tmp/station-e2e && mkdir -p /tmp/station-e2e/hello /tmp/station-e2e/npm' || fail "scratch folder"
scp "${ssh_opts[@]}" -o BatchMode=yes -i "$ssh_key" -q "$binary" "$user@$ip:/tmp/station-e2e/station" || fail "copy the binary"
scp "${ssh_opts[@]}" -o BatchMode=yes -i "$ssh_key" -q "$repo/scripts/install.sh" "$user@$ip:/tmp/station-e2e/install.sh" || fail "copy install.sh"

listing() { in_vm 'cd ~ && find . -mindepth 1 -maxdepth 4 2>/dev/null | sed "s|^\./||" | LC_ALL=C sort'; }

listing > "$work/0-before.txt" || fail "list the home"
steps=()
n=0

# invariant: the home is listed after every step, so an unmatched path names the
# step that made it.
step() {
  local name=$1 command=$2
  n=$((n + 1))
  say "-- step $n: $name"
  say "   \$ $command"
  in_vm "$command" >> "$log" 2>&1
  local code=$?
  say "   exit $code"
  listing > "$work/$n.txt"
  steps+=("$n:$name")
  [ $code -eq 0 ] || fail "step $n ($name) exited $code"
}

station="\$HOME/.local/bin/station"
if [ "$kind" = macos ]; then
  brew_path='PATH=/opt/homebrew/bin:$PATH'
else
  brew_path=''
fi
mise_path="PATH=\$HOME/.local/share/mise/shims:\$HOME/.local/bin:/opt/homebrew/bin:\$PATH"

step "install.sh" 'STATION_INSTALL_FROM=/tmp/station-e2e/station sh /tmp/station-e2e/install.sh </dev/null'
if [ "$kind" = macos ]; then
  step "setup" "$brew_path $station setup --name e2e </dev/null"
else
  step "setup" "$station setup </dev/null"
fi
while read -r tool; do
  case "$tool" in '' | '#'*) continue ;; esac
  step "tools install $tool" "$brew_path $mise_path $station tools install $tool </dev/null"
done < "$suite/$kind/one-per-method.txt"

step "go build" "$mise_path; cd /tmp/station-e2e/hello && printf 'module hello\n\ngo 1.21\n' > go.mod && printf 'package main\n\nfunc main() { println(\"hello\") }\n' > main.go && go build -o hello . && ./hello"
step "npm i" "$mise_path; cd /tmp/station-e2e/npm && printf '{\"name\":\"e2e\",\"private\":true}\n' > package.json && npm i is-number@7.0.0"
step "bun add -g" "$mise_path; bun add -g is-number@7.0.0"
step "bunx" "$mise_path; cd /tmp/station-e2e && bunx cowsay@1.6.0 hello"
step "uv tool install" "$mise_path; uv tool install pyflakes==3.2.0"
if [ "$kind" = macos ]; then
  step "brew install" "$brew_path; brew install hello && hello"
fi
step "gopls" "$mise_path; go install golang.org/x/tools/gopls@v0.20.0"
in_vm 'test -x ~/.local/share/go/bin/gopls' || fail "go install put no gopls in ~/.local/share/go/bin: mise set GOBIN"
step "zsh -c" "$mise_path; zsh -c 'go env GOPATH'"
step "zsh -ic" "zsh -ic 'command -v station && command -v mise && go env GOPATH && mise --version'"
step "IDE: go env" "$mise_path; d=\$(mise where go)/bin; env -i HOME=\$HOME PATH=\$d go env GOPATH"
step "IDE: gopls version" "$mise_path; d=\$(mise where go)/bin; env -i HOME=\$HOME PATH=\$HOME/.local/share/go/bin:\$d gopls version"
step "IDE: npm config get cache" "$mise_path; d=\$(mise where node)/bin; env -i HOME=\$HOME PATH=\$d:/usr/bin:/bin npm config get cache"

in_vm 'test -f ~/.bunfig.toml' || fail "setup wrote no ~/.bunfig.toml"
if in_vm 'test -e ~/.bun'; then
  fail "~/.bun present after tools install skills, bun add -g and bunx: bun ignored the bunfig"
fi
say "   ~/.bun absent after tools install skills, bun add -g and bunx: bun honours the bunfig"


say "-- the footprint test"
in_vm "$station disk --json" > "$work/disk.json" || fail "disk --json"
{
  jq -r '.rows[] | select(.group != "misplaced") | .paths[] | select(startswith("~/")) | ltrimstr("~/")' "$work/disk.json"
  sed -n 's/^| `~\/\([^`]*\)`.*/\1/p' "$repo/docs/reference/footprint.md"
  grep -v '^#' "$suite/$kind/os-noise.txt"
  grep -v '^#' "$suite/suite-made.txt"
} | grep -v '^$' | LC_ALL=C sort -u > "$work/allowed.txt"
unmatched=0
prev="$work/0-before.txt"
for entry in "${steps[@]}"; do
  i=${entry%%:*}
  name=${entry#*:}
  LC_ALL=C comm -13 "$prev" "$work/$i.txt" > "$work/$i.new.txt"
  awk -f "$suite/footprint.awk" "$work/allowed.txt" "$work/$i.new.txt" > "$work/$i.unmatched.txt"
  while IFS= read -r path; do
    say "   unmatched ~/$path (made by step $i: $name)"
    unmatched=$((unmatched + 1))
  done < "$work/$i.unmatched.txt"
  prev="$work/$i.txt"
done
say "   $unmatched unmatched paths"
[ "$unmatched" -eq 0 ] || fail "the footprint test"


say "-- disk free"
data_sizes() { jq -r '.rows[] | select(.group == "your-data") | "\(.app) \(.bytes) \(.paths | join(" "))"' "$1"; }
data_sizes "$work/disk.json" > "$work/data-before.txt"
in_vm "$brew_path $mise_path $station disk free" >> "$log" 2>&1 || fail "disk free exited $?"
in_vm "$station disk --json" > "$work/disk-after.json" || fail "disk --json"
left=$(jq -r '.rows[] | select(.group == "can-rebuild" and .bytes > 0) | "\(.app) \(.bytes)"' "$work/disk-after.json")
[ -z "$left" ] || { say "   Can rebuild left: $left"; fail "disk free"; }
data_sizes "$work/disk-after.json" > "$work/data-after.txt"
diff "$work/data-before.txt" "$work/data-after.txt" >> "$log" || fail "disk free touched a Your data row"
say "   every Can rebuild row empty or absent, every Your data row untouched"


say "-- disk erase go, on a fresh clone"
# hazard: tart stop can end a Linux guest before it writes its page cache back,
# so the clone boots an older disk (2026-11-01: go's bin empty, the free's
# deletions undone); sync first.
in_vm 'sync' || fail "sync before the clone"
tart stop "$vm" >> "$log" 2>&1
erase_vm="station-e2e-$kind-$run_id-erase"
made+=("$erase_vm")
tart clone "$vm" "$erase_vm" >> "$log" 2>&1 || fail "clone for the erase"
ip=$(boot "$erase_vm") || fail "boot $erase_vm"
in_vm "$station disk --json" > "$work/erase-before.json" || fail "disk --json"
held=$(jq -r '[.rows[] | select(.app == "go" and .group == "your-data") | .paths[] | select(. == "~/.local/share/go/bin/*")] | length' "$work/erase-before.json")
[ "$held" -gt 0 ] || fail "go's bin holds nothing before the erase: the check would prove nothing"
in_vm "$mise_path $station disk erase go --confirm=go" >> "$log" 2>&1 || fail "disk erase go exited $?"
in_vm "$station disk --json" > "$work/erase-after.json" || fail "disk --json"
gone=$(jq -r '.rows[] | select(.app == "go" and (.group == "your-data" or .group == "can-rebuild") and .bytes > 0) | "\(.group) \(.paths | join(" "))"' "$work/erase-after.json")
[ -z "$gone" ] || { say "   go still holds: $gone"; fail "disk erase go"; }
others() { jq -r '.rows[] | select(.app != "go" and .group == "your-data") | "\(.app) \(.bytes) \(.paths | join(" "))"' "$1"; }
diff <(others "$work/erase-before.json") <(others "$work/erase-after.json") >> "$log" || fail "disk erase go touched another app's data"
say "   only go's data gone"

say "PASS tier $([ "$kind" = linux ] && echo 2 || echo 3): $kind"
