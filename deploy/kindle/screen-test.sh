#!/bin/sh
# One-shot screen test: no services stopped and no power settings changed.
set -u
BASE=/mnt/us/eatby
[ -d /mnt/us ] && [ -e /proc/version ] || exit 1
mkdir -p "$BASE/logs"
umask 077
exec > "$BASE/logs/screen-test.txt" 2>&1
# Give KUAL time to close before rendering.
sleep 3
date -u
FBINK=/mnt/us/libkh/bin/fbink
printf '\nActual FBInk geometry\n'
"$FBINK" -e
printf '\nNative image renderer\n'
if command -v eips >/dev/null 2>&1; then
  eips -g "$BASE/screen-test.png"
  result=$?
  printf 'PNG render exit status: %s\n' "$result"
else
  result=127
  printf 'eips unavailable\n'
fi
if [ "$result" -eq 0 ]; then
  "$FBINK" -m -y 3 'EatBy - 600 x 800'
  "$FBINK" -m -y 5 'PNG display test'
else
  "$FBINK" -c -m -y 3 'EatBy text display OK'
  "$FBINK" -m -y 5 'PNG test failed - check log'
fi
printf '\nPower capabilities (read only)\n'
if command -v lipc-probe >/dev/null 2>&1; then
  lipc-probe -v com.lab126.powerd
fi
printf '\nRTC alarm state (read only)\n'
for f in /sys/class/rtc/rtc0/wakealarm /sys/class/rtc/rtc1/wakealarm; do
  if [ -r "$f" ]; then printf '%s: ' "$f"; head -c 128 "$f"; printf '\n'; fi
done
printf '\nDone. Reopen Home to restore normal UI.\n'
