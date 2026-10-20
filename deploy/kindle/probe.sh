#!/bin/sh
# Read-only capability probe. Run on the Kindle, NOT on the Mac.
# Does not modify power settings, screen contents, networking or system files.
set -u
BASE=/mnt/us/eatby
if [ ! -d /mnt/us ] || [ ! -e /proc/version ]; then
  echo 'Run this script on the Kindle.' >&2
  exit 1
fi
mkdir -p "$BASE/logs"
umask 077
exec > "$BASE/logs/probe.txt" 2>&1
printf 'EatBy Kindle capability probe\n'
date -u
uname -m
printf '\nFirmware\n'
[ ! -f /mnt/us/system/version.txt ] || head -c 256 /mnt/us/system/version.txt
printf '\nCommands\n'
for c in lipc-get-prop lipc-set-prop rtcwake curl wget busybox powerd fbink; do
  command -v "$c" || true
done
printf '\nRoot filesystem mount state (read only)\n'
awk '$2 == "/" { print $0 }' /proc/mounts
printf '\nNetwork client versions\n'
if command -v curl >/dev/null 2>&1; then curl --version; fi
printf '\nFramebuffer\n'
for f in /sys/class/graphics/fb0/virtual_size /sys/class/graphics/fb0/bits_per_pixel; do
  if [ -r "$f" ]; then printf '%s: ' "$f"; head -c 128 "$f"; printf '\n'; fi
done
printf '\nFBInk help (no drawing)\n'
if [ -x /mnt/us/libkh/bin/fbink ]; then
  /mnt/us/libkh/bin/fbink --help
fi
printf '\nPower state (read only)\n'
if command -v lipc-get-prop >/dev/null 2>&1; then
  lipc-get-prop com.lab126.powerd state || true
  lipc-get-prop com.lab126.powerd battLevel || true
fi
printf '\nRTC presence (no alarm changes)\n'
ls -l /dev/rtc* /sys/class/rtc/ 2>/dev/null || true
printf '\nDone. No serial number, credentials or network addresses collected.\n'
