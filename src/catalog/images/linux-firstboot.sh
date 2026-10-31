#!/bin/bash
# invariant: A new VM's first boot: run as root (sudo bash -s) over the NAT session. The
# rendering replaces the {{name}}, {{lifetime}} and {{authkey}} placeholders.
set -euo pipefail

hostnamectl set-hostname '{{name}}'
rm -f /etc/machine-id
systemd-machine-id-setup
growpart /dev/vda 1 || true
resize2fs /dev/vda1 || true
if [ '{{lifetime}}' = temporary ]; then
    echo 'temporary VM: poweroff deletes it; reboot keeps it' > /etc/motd
fi
tailscale up --auth-key=file:/dev/stdin --hostname='{{name}}' --ssh <<'KEY' >/dev/null
{{authkey}}
KEY
tailscale status --json | sed -n 's/.*"ID": *"\([^"]*\)".*/\1/p' | head -1
