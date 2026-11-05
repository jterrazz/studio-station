#!/bin/bash
# value: tier 3: one fresh macOS VM plays a Mac: install.sh, setup, the apps of every
# method, every setting, the shell block, Activity, then free and erase on a clone.
# Sourced by run.sh.

running=$(tart list --format json 2>/dev/null | jq -r '.[] | select(.State == "running" or .Running == true) | .Name')
[ -z "$running" ] || fail "a VM is running ($running): tier 3 runs one macOS VM at 8 GB with no other VM"

run_id="$$"
made=()
cleanup_tier() {
  local vm left
  for vm in "${made[@]:-}"; do
    [ -n "$vm" ] || continue
    tart stop "$vm" > /dev/null 2>&1 || true
    tart delete "$vm" > /dev/null 2>&1 || true
  done
  left=$(tart list --quiet --source local 2>/dev/null | grep "^station-e2e-macos-$run_id" || true)
  [ -z "$left" ] || say "WARN left behind: $left"
}

ssh_key="$work/id_ed25519"
ssh-keygen -q -t ed25519 -N '' -C "station-e2e" -f "$ssh_key" || fail "ssh-keygen"
hash=$(cd "$repo" && find src/catalog -type f | LC_ALL=C sort | xargs cat | shasum -a 256 | cut -c1-12)
base="station-e2e-macos-$hash"
user=admin
say "-- base $base"

# value: a fresh macOS guest knows only admin/admin; the first ssh answers the
# password through SSH_ASKPASS and installs this run's key, every later ssh uses it;
# ssh keeps the first value of an option, so BatchMode is turned off in place.
askpass="$work/askpass"
printf '#!/bin/sh\necho admin\n' > "$askpass"
chmod 700 "$askpass"
boot() {
  local vm=$1 ip n=0
  tart run --no-graphics "$vm" >> "$work/$vm.tart.log" 2>&1 &
  ip=$(tart ip "$vm" --wait 180) || return 1
  until SSH_ASKPASS="$askpass" SSH_ASKPASS_REQUIRE=force DISPLAY=none \
    ssh "${ssh_opts[@]/BatchMode=yes/BatchMode=no}" -o PubkeyAuthentication=no "$user@$ip" \
    "mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys" < "$ssh_key.pub" 2> /dev/null; do
    n=$((n + 1))
    [ $n -lt 60 ] || return 1
    sleep 2
  done
  echo "$ip"
}

if ! tart list --quiet --source local | grep -qx "$base"; then
  say "-- prebaking $base from ghcr.io/cirruslabs/macos-tahoe-base:latest"
  for old in $(tart list --quiet --source local | grep '^station-e2e-macos-' | grep -v -- "-$run_id"); do
    tart delete "$old" > /dev/null 2>&1 || true
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

vm="station-e2e-macos-$run_id"
made+=("$vm")
tart clone "$base" "$vm" >> "$log" 2>&1 || fail "clone $base"
tart set "$vm" --memory 8192 >> "$log" 2>&1 || fail "size $vm"
ip=$(boot "$vm") || fail "boot $vm"
say "-- $vm at $ip"

# invariant: inside, the guest plays a Mac: STATION_MACHINE_KIND=host on every call
# but the one that reads Activity as a macOS workspace's guest.
in_vm() { ssh -n "${ssh_opts[@]}" -i "$ssh_key" "$user@$ip" "export STATION_MACHINE_KIND=host; $1"; }
say "kern.hv_vmm_present: $(in_vm 'sysctl -n kern.hv_vmm_present' 2>&1)"

in_vm 'rm -rf /tmp/station-e2e && mkdir -p /tmp/station-e2e/hello /tmp/station-e2e/npm' || fail "scratch folder"
scp "${ssh_opts[@]}" -i "$ssh_key" -q "$repo/.artifacts/go/station" "$user@$ip:/tmp/station-e2e/station" || fail "copy the binary"
scp "${ssh_opts[@]}" -i "$ssh_key" -q "$repo/scripts/install.sh" "$user@$ip:/tmp/station-e2e/install.sh" || fail "copy install.sh"

listing() { in_vm 'cd ~ && find . -mindepth 1 -maxdepth 4 2>/dev/null | sed "s|^\./||" | LC_ALL=C sort'; }
station="\$HOME/.local/bin/station"
brew_path='PATH=/opt/homebrew/bin:$PATH'
mise_path="PATH=\$HOME/.local/share/mise/shims:\$HOME/.local/bin:/opt/homebrew/bin:\$PATH"
fp="$work/home"
mkdir -p "$fp"
steps=()
n=0
# invariant: the home is listed after every step, so an unmatched path names the
# step that made it.
step() {
  local name=$1 command=$2 code
  n=$((n + 1))
  say "-- step $n: $name"
  say "   \$ $command"
  in_vm "$command" >> "$log" 2>&1
  code=$?
  say "   exit $code"
  listing > "$fp/$n.txt"
  steps+=("$n:$name")
  [ $code -eq 0 ] || fail "step $n ($name) exited $code"
}


case_begin install-then-setup SU-08
listing > "$fp/0-before.txt" || fail "list the home"
step "install.sh" 'STATION_INSTALL_FROM=/tmp/station-e2e/station sh /tmp/station-e2e/install.sh </dev/null'
step "setup" "$brew_path $station setup --name e2e </dev/null"
while read -r app; do
  case "$app" in '' | '#'*) continue ;; esac
  step "apps install $app" "$brew_path $mise_path $station apps install $app </dev/null"
done < "$suite/macos/one-per-method.txt"
step "go build" "$mise_path; cd /tmp/station-e2e/hello && printf 'module hello\n\ngo 1.21\n' > go.mod && printf 'package main\n\nfunc main() { println(\"hello\") }\n' > main.go && go build -o hello . && ./hello"
step "npm i" "$mise_path; cd /tmp/station-e2e/npm && printf '{\"name\":\"e2e\",\"private\":true}\n' > package.json && npm i is-number@7.0.0"
step "bun add -g" "$mise_path; bun add -g is-number@7.0.0"
step "bunx" "$mise_path; cd /tmp/station-e2e && bunx cowsay@1.6.0 hello"
step "uv tool install" "$mise_path; uv tool install pyflakes==3.2.0"
step "brew install" "$brew_path; brew install hello && hello"
step "gopls" "$mise_path; go install golang.org/x/tools/gopls@v0.20.0"
in_vm 'test -x ~/.local/share/go/bin/gopls' || fail "go install put no gopls in ~/.local/share/go/bin: mise set GOBIN"
in_vm 'test -f ~/.bunfig.toml' || fail "setup wrote no ~/.bunfig.toml"
in_vm 'test -e ~/.bun' && fail "~/.bun present after apps install skills, bun add -g and bunx: bun ignored the bunfig"
in_vm "$station disk --json" > "$fp/disk.json" || fail "disk --json"
footprint "$fp" "$fp/disk.json" "$suite/macos/os-noise.txt" || fail "the footprint test"
case_pass


case_begin shell-block-native-calls SU-11
login=$(in_vm "zsh -lic 'command -v station; print -r -- \$PATH; node --version; go version; whence -w _station dclean gst; print -r -- \${STATION_TERMINAL:-unset}'" 2> /dev/null)
say "$(sed 's/^/   login: /' <<< "$login")"
grep -q '/.local/bin/station$' <<< "$login" || fail "a login shell finds no station in ~/.local/bin"
grep -q '^v[0-9]' <<< "$login" || fail "a login shell runs no mise node"
grep -q '^go version go' <<< "$login" || fail "a login shell runs no mise go"
grep -q '^_station: function' <<< "$login" || fail "a login shell loads no station completion"
grep -q '^dclean: function' <<< "$login" || fail "a login shell defines no Docker shortcuts"
! grep -qx unset <<< "$login" || fail "a login shell sets no STATION_TERMINAL"
plain=$(in_vm "zsh -c 'command -v station mise; go env GOPATH'" 2> /dev/null)
[ "$(wc -l <<< "$plain" | tr -d ' ')" -ge 3 ] || fail "zsh -c finds station, mise and go only partly: $plain"
step "IDE: go env" "$mise_path; d=\$(mise where go)/bin; env -i HOME=\$HOME PATH=\$d go env GOPATH"
step "IDE: gopls version" "$mise_path; d=\$(mise where go)/bin; env -i HOME=\$HOME PATH=\$HOME/.local/share/go/bin:\$d gopls version"
step "IDE: npm config get cache" "$mise_path; d=\$(mise where node)/bin; env -i HOME=\$HOME PATH=\$d:/usr/bin:/bin npm config get cache"
if in_vm "grep -n -e '/private/tmp' -e '/Developer/' -e '.artifacts' ~/.zshrc ~/.config/station/zsh/* 2>/dev/null"; then
  fail "the shell block names a clone path"
fi
case_pass


case_begin apps-every-method AP-24
while read -r app method; do
  case "$app" in '' | '#'*) continue ;; esac
  if [ "$method" = mas ]; then
    out=$(in_vm "$brew_path $mise_path $station apps install $app </dev/null" 2>&1)
    code=$?
    say "   mas $app: exit $code: $(tr '\n' ' ' <<< "$out" | cut -c1-200)"
    [ $code -ne 0 ] && grep -qi 'app store\|sign' <<< "$out" || miss "mas $app signed out gave no sign-in hint"
    continue
  fi
  for verb in install update remove; do
    in_vm "$brew_path $mise_path $station apps $verb $app </dev/null" >> "$log" 2>&1 || { miss "apps $verb $app ($method) exited $?"; break; }
    state=$(in_vm "$brew_path $mise_path $station apps --all --json" | jq -r --arg a "$app" '.apps[] | select(.name == $a) | .state')
    say "   $method $app: $verb, then $state"
    case "$verb" in
      remove) [ "$state" = not-installed ] || miss "$app is $state after remove" ;;
      *) [ "$state" != not-installed ] || miss "$app is not installed after $verb" ;;
    esac
  done
done < "$suite/macos/every-method.txt"
case_pass


case_begin settings-every-one SE-34
in_vm "$brew_path $station settings --json" > "$work/settings-0.json" || fail "settings --json"
jq -r '.settings[] | select(.group == "set") | "\(.name) \(.state)"' "$work/settings-0.json" > "$work/settings-set.txt"
[ -s "$work/settings-set.txt" ] || fail "settings --json lists no Set by station setting"
while read -r name state; do
  if grep -qx "$name" "$suite/macos/settings-asked.txt"; then
    say "   $name: asks values on a terminal, left to the owner"
    continue
  fi
  in_vm "$brew_path $station settings apply $name </dev/null" >> "$log" 2>&1 || { miss "settings apply $name exited $?"; continue; }
  now=$(in_vm "$brew_path $station settings --json" | jq -r --arg n "$name" '.settings[] | select(.name == $n) | .state')
  [ "$now" = applied ] || { miss "$name reads $now after apply"; continue; }
  in_vm "$brew_path $station settings off $name </dev/null" >> "$log" 2>&1 || { miss "settings off $name exited $?"; continue; }
  off=$(in_vm "$brew_path $station settings --json" | jq -r --arg n "$name" '.settings[] | select(.name == $n) | .state')
  [ "$off" != applied ] || miss "$name still reads applied after off"
  in_vm "$brew_path $station settings apply $name </dev/null" >> "$log" 2>&1 || miss "settings apply $name again exited $?"
  say "   $name: applied, off ($off), applied again"
done < "$work/settings-set.txt"
real=$(in_vm 'printf "filevault %s\nfirewall %s\nsip %s\ngatekeeper %s\n" "$(fdesetup status | grep -q "is On" && echo on || echo off)" "$(/usr/libexec/ApplicationFirewall/socketfilterfw --getglobalstate | grep -q enabled && echo on || echo off)" "$(csrutil status | grep -q enabled && echo on || echo off)" "$(spctl --status 2>&1 | grep -q enabled && echo on || echo off)"')
while read -r name value; do
  read_value=$(jq -r --arg n "$name" '.settings[] | select(.name == $n) | .value' "$work/settings-0.json")
  say "   checked $name: station reads $read_value, the system says $value"
  [ "$read_value" = "$value" ] || miss "checked $name reads $read_value, the system says $value"
done <<< "$real"
case_pass


case_begin activity-in-macos-vm AC-36
act=$(ssh -n "${ssh_opts[@]}" -i "$ssh_key" "$user@$ip" "STATION_MACHINE_KIND=workspace $station activity --json")
ncpu=$(in_vm 'sysctl -n hw.ncpu')
mem=$(in_vm 'sysctl -n hw.memsize')
[ "$(jq -r '.cpu.cpus' <<< "$act")" = "$ncpu" ] || fail "cpus $(jq -r '.cpu.cpus' <<< "$act") vs hw.ncpu $ncpu"
[ "$(jq -r '.memory.total_bytes' <<< "$act")" = "$mem" ] || fail "memory $(jq -r '.memory.total_bytes' <<< "$act") vs hw.memsize $mem"
[ "$(jq -r '.thermal, .battery' <<< "$act" | sort -u)" = null ] || fail "a macOS workspace reads a Thermal or a Battery line"
[ "$(jq -r '.top.by_cpu | length' <<< "$act")" -gt 0 ] || fail "no top processes"
say "   $(jq -c '{cpus: .cpu.cpus, busy: .cpu.busy_percent, used: .memory.used_bytes, total: .memory.total_bytes, up: .up_s}' <<< "$act")"
case_pass


case_begin disk-free-erase DK-16
data_sizes() { jq -r '.rows[] | select(.group == "your-data") | "\(.app) \(.bytes) \(.paths | join(" "))"' "$1"; }
in_vm "$station disk --json" > "$work/disk-before.json" || fail "disk --json"
data_sizes "$work/disk-before.json" > "$work/data-before.txt"
in_vm "$brew_path $mise_path $station disk free" >> "$log" 2>&1 || fail "disk free exited $?"
in_vm "$station disk --json" > "$work/disk-after.json" || fail "disk --json"
left=$(jq -r '.rows[] | select(.group == "can-rebuild" and .bytes > 0) | "\(.app) \(.bytes)"' "$work/disk-after.json")
[ -z "$left" ] || fail "disk free left: $left"
data_sizes "$work/disk-after.json" > "$work/data-after.txt"
diff "$work/data-before.txt" "$work/data-after.txt" >> "$log" || fail "disk free touched a Your data row"
say "   every Can rebuild row empty or absent, every Your data row untouched"
in_vm 'sync' || fail "sync before the clone"
tart stop "$vm" >> "$log" 2>&1
erase_vm="station-e2e-macos-$run_id-erase"
made+=("$erase_vm")
tart clone "$vm" "$erase_vm" >> "$log" 2>&1 || fail "clone for the erase"
ip=$(boot "$erase_vm") || fail "boot $erase_vm"
in_vm "$station disk --json" > "$work/erase-before.json" || fail "disk --json"
held=$(jq -r '[.rows[] | select(.app == "go" and .group == "your-data") | .paths[] | select(. == "~/.local/share/go/bin/*")] | length' "$work/erase-before.json")
[ "$held" -gt 0 ] || fail "go's bin holds nothing before the erase: the check would prove nothing"
in_vm "$mise_path $station disk erase go --confirm=go" >> "$log" 2>&1 || fail "disk erase go exited $?"
in_vm "$station disk --json" > "$work/erase-after.json" || fail "disk --json"
gone=$(jq -r '.rows[] | select(.app == "go" and (.group == "your-data" or .group == "can-rebuild") and .bytes > 0) | "\(.group) \(.paths | join(" "))"' "$work/erase-after.json")
[ -z "$gone" ] || fail "disk erase go left: $gone"
others() { jq -r '.rows[] | select(.app != "go" and .group == "your-data") | "\(.app) \(.bytes) \(.paths | join(" "))"' "$1"; }
diff <(others "$work/erase-before.json") <(others "$work/erase-after.json") >> "$log" || fail "disk erase go touched another app's data"
say "   only go's data gone"
case_pass
