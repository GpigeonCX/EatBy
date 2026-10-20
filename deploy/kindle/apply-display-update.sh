#!/bin/sh
# Install a locally staged, checksummed display-only update after agent stop.
set -eu
BASE=/mnt/us/eatby
STAGE=$BASE/display-update
LOCK=/tmp/eatby-display-update
[ -d /mnt/us ] && [ -e /proc/version ] || exit 1
if [ "${1:-}" != worker ]; then
 mkdir "$LOCK" 2>/dev/null || exit 1
 mkdir -p "$BASE/logs"
 setsid sh "$BASE/apply-display-update.sh" worker </dev/null > "$BASE/logs/display-update.txt" 2>&1 &
 exit 0
fi
trap 'rmdir "$LOCK" 2>/dev/null || true' EXIT
say() { /mnt/us/libkh/bin/fbink -q -m -y 12 "$1" || true; }
sleep 3
for name in wake-test.sh refresh.sh; do
 if [ ! -s "$STAGE/$name" ]; then say 'Update package missing.';exit 1;fi
 sh -n "$STAGE/$name"
done
if ! (cd "$STAGE" && { if command -v sha256sum >/dev/null 2>&1; then sha256sum -c SHA256SUMS;else busybox sha256sum -c SHA256SUMS;fi; }); then
 say 'Update checksum failed; no files changed.';exit 1
fi
restart=0
if [ -d /tmp/eatby-agent ]; then
 restart=1
 touch /tmp/eatby-agent/stop
 say 'Stopping dashboard to apply update...'
 n=0
 while [ -d /tmp/eatby-agent ] && [ "$n" -lt 30 ]; do sleep 2;n=$((n+1));done
 if [ -d /tmp/eatby-agent ]; then say 'Stop did not finish; update cancelled.';exit 1;fi
fi
if [ -d /tmp/eatby-exclusive-test ] || [ -d /tmp/eatby-wake-test ] || [ -d /tmp/eatby-refresh.lock ]; then
 say 'Another test/refresh is active. Update cancelled.';exit 1
fi
mkdir -p "$BASE/backup-display-v1"
for name in wake-test.sh refresh.sh; do
 if [ ! -f "$BASE/backup-display-v1/$name" ]; then cp "$BASE/$name" "$BASE/backup-display-v1/$name";fi
 cp "$STAGE/$name" "$BASE/$name.new"
 mv "$BASE/$name.new" "$BASE/$name"
done
echo 'Display optimization installed; pairing and schedule unchanged.'
say 'Display update installed.'
if [ "$restart" = 1 ]; then
 sleep 2
 sh "$BASE/battery-agent.sh" start
 echo 'Dashboard restart requested.'
else
 echo 'Agent was stopped; use START battery dashboard when ready.'
fi
