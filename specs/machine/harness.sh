#!/bin/bash
# value: the machine tiers' shared words: the log, a case's verdict, ssh into a guest and
# the footprint test. Sourced by run.sh, never run alone.

say() { printf '%s\n' "$*" | tee -a "$log"; }
fail() { say "FAIL ${current:-setup} $*"; exit 1; }

# invariant: every case says its name and the use cases it proves on its first line
# and PASS on its last, so the log reads as the list of what this tier proved.
current=""
case_begin() {
  # value: E2E_UNTIL=<case> ends the run before that case and keeps its VMs and
  # processes, to look at by hand; their cleanup is then the caller's.
  if [ "${E2E_UNTIL:-}" = "$1" ]; then
    say "-- stopped before $1: the VMs and processes stay"
    trap - EXIT
    exit 0
  fi
  current=$1
  shift
  say "== case $current ($*)"
}
# value: a miss is a failed check the run goes on past, so one run of a slow tier
# names every one; its case is then failed, the next case still runs, and the
# tier exits 1.
missed=()
miss() {
  say "   MISS $*"
  missed+=("$*")
}
case_pass() {
  if [ ${#missed[@]} -gt 0 ]; then
    say "FAIL $current ${#missed[@]} missed"
    failed+=("$current")
  else
    say "PASS $current"
    passed+=("$current")
  fi
  missed=()
  current=""
}
passed=()
failed=()

ssh_opts=(-F /dev/null -o IdentitiesOnly=yes -o IdentityAgent=none -o BatchMode=yes
  -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR
  -o ConnectTimeout=5 -o ServerAliveInterval=15)

# value: the footprint test over one VM's listings: $1 the folder of the listings
# (0-before.txt, then one per step), $2 the disk --json of the guest, $3 the
# tier's os-noise file; the steps are the "n:name" entries of the steps array.
footprint() {
  local dir=$1 disk=$2 noise=$3 unmatched=0 prev entry i name path
  {
    jq -r '.rows[] | select(.group != "misplaced") | .paths[] | select(startswith("~/")) | ltrimstr("~/")' "$disk"
    sed -n 's/^| `~\/\([^`]*\)`.*/\1/p' "$repo/docs/reference/footprint.md"
    grep -v '^#' "$noise"
    grep -v '^#' "$suite/suite-made.txt"
  } | grep -v '^$' | LC_ALL=C sort -u > "$dir/allowed.txt"
  prev="$dir/0-before.txt"
  for entry in "${steps[@]}"; do
    i=${entry%%:*}
    name=${entry#*:}
    LC_ALL=C comm -13 "$prev" "$dir/$i.txt" > "$dir/$i.new.txt"
    awk -f "$suite/footprint.awk" "$dir/allowed.txt" "$dir/$i.new.txt" > "$dir/$i.unmatched.txt"
    while IFS= read -r path; do
      say "   unmatched ~/$path (made by step $i: $name)"
      unmatched=$((unmatched + 1))
    done < "$dir/$i.unmatched.txt"
    prev="$dir/$i.txt"
  done
  say "   $unmatched unmatched paths"
  [ "$unmatched" -eq 0 ]
}

# value: within N percent of the reference, plus an absolute slack (the bytes the
# reads themselves move between one count and the other), both whole numbers.
near() {
  local got=$1 want=$2 pct=$3 slack=${4:-1}
  awk -v g="$got" -v w="$want" -v p="$pct" -v s="$slack" 'BEGIN { d = g - w; if (d < 0) d = -d; exit !(d <= w * p / 100 + s) }'
}
