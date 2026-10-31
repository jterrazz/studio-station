#!/bin/bash
# invariant: The second build session: run as root (sudo bash -s) as VM_USER with the image
# key, once the base's admin session has ended, so its user can be removed.
set -euo pipefail

pkill -u admin || true
userdel -r admin || true
truncate -s 0 /etc/machine-id
rm -f /etc/ssh/ssh_host_*
apt-get clean
rm -rf /var/lib/apt/lists/*
sync
systemctl --no-block poweroff
