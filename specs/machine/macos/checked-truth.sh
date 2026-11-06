#!/bin/bash
# value: the system's own answer for every checked setting, asked of the tool macOS
# answers it with, printed as "<name> <value>" in the words station reads it in.
set -uo pipefail
onoff() { if "$@"; then echo on; else echo off; fi; }
echo "filevault $(onoff sh -c 'fdesetup status | grep -q "is On"')"
echo "firewall $(onoff sh -c '/usr/libexec/ApplicationFirewall/socketfilterfw --getglobalstate | grep -q enabled')"
echo "sip $(onoff sh -c 'csrutil status | grep -q enabled')"
echo "gatekeeper $(onoff sh -c 'spctl --status 2>&1 | grep -q enabled')"
critical=$(defaults read /Library/Preferences/com.apple.SoftwareUpdate CriticalUpdateInstall 2> /dev/null || echo 1)
echo "automatic-updates $([ "$critical" = 0 ] && echo off || echo on)"
echo "remote-login $(onoff sh -c 'sudo -n systemsetup -getremotelogin | grep -q "Remote Login: On"')"
lock=$(sysadminctl -screenLock status 2>&1)
case "$lock" in
  *"screenLock is off"*) echo "screen-lock never locks" ;;
  *"delay is immediate"*) echo "screen-lock locks when the display turns off" ;;
  *) echo "screen-lock $lock" ;;
esac
if tmutil destinationinfo 2>&1 | grep -q "No destinations configured"; then
  echo "time-machine no backup yet"
else
  echo "time-machine $(tmutil latestbackup 2>&1 | tail -1)"
fi
waiting=$(softwareupdate -l 2>&1 | grep -c '^\* Label:')
case "$waiting" in
  0) echo "macos-updates up to date" ;;
  1) echo "macos-updates 1 update waiting" ;;
  *) echo "macos-updates $waiting updates waiting" ;;
esac
