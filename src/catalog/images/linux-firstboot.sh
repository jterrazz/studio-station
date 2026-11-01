#!/bin/bash
# invariant: A new VM's first boot: run as root (sudo bash -s) over the NAT session. The
# rendering replaces the {{name}} and {{lifetime}} placeholders.
set -euo pipefail

hostnamectl set-hostname '{{name}}'
rm -f /etc/machine-id
systemd-machine-id-setup
growpart /dev/vda 1 || true
resize2fs /dev/vda1 || true
if [ '{{lifetime}}' = temporary ]; then
    echo 'temporary VM: poweroff deletes it; reboot keeps it' > /etc/motd
fi
