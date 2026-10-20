#!/bin/sh
# One-shot RTC1 alarm experiment WHILE AWAKE. No suspend, clock-setting,
# wakeup-enable changes, framework changes, or USB configuration writes.
set -u
BASE=/mnt/us/eatby
RTC=/sys/class/rtc/rtc1
LOCK=/tmp/eatby-rtc1-check
[ -d /mnt/us ] && [ -e /proc/version ] || exit 1
umask 077
if [ "${1:-}" != worker ]; then
 mkdir -p "$BASE/logs"
 mkdir "$LOCK" 2>/dev/null || exit 1
 setsid sh "$BASE/rtc1-check.sh" worker </dev/null >"$BASE/logs/rtc1-check.txt" 2>&1 &
 exit 0
fi
target=
cleanup() {
 # Never clear a different alarm installed by another component.
 if [ -n "$target" ] && [ -r "$RTC/wakealarm" ]; then
  current=$(head -n 1 "$RTC/wakealarm")
  if [ "$current" = "$target" ]; then
   printf 'Clearing remaining test alarm only: %s\n' "$target"
   printf '0\n' > "$RTC/wakealarm" || true
  fi
 fi
 rmdir "$LOCK" 2>/dev/null || true
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
sleep 3
FB=/mnt/us/libkh/bin/fbink
say() { "$FB" -q -c -m -y 14 "$1" || true; }
number() { case "$1" in ''|*[!0-9]*) return 1;; *) return 0;; esac; }
snapshot() {
 date -u
 printf 'Selected RTC epoch: '; head -n 1 "$RTC/since_epoch"
 printf 'Selected alarm: '; head -n 1 "$RTC/wakealarm"
 printf '\nRTC interrupt counters\n'
 grep -iE 'rtc|snvs|bd718' /proc/interrupts || true
}
date -u
for property in name since_epoch wakealarm device/power/wakeup; do
 [ -r "$RTC/$property" ] || { echo "Missing RTC1 property: $property"; say 'RTC1 unavailable. Check log.'; exit 1; }
done
name=$(head -n 1 "$RTC/name")
enabled=$(head -n 1 "$RTC/device/power/wakeup")
printf 'RTC1 name=%s wakeup=%s\n' "$name" "$enabled"
case "$name" in *snvs*) ;; *) say 'Unexpected RTC1. Test cancelled.';exit 1;; esac
[ "$enabled" = enabled ] || { say 'RTC1 wake disabled. Test cancelled.';exit 1; }
state=$(lipc-get-prop com.lab126.powerd state 2>/dev/null) || state=unknown
[ "$state" = active ] || { say 'Must be awake. Test cancelled.';exit 1; }
# Refuse ANY existing selected-device alarm, including stale alarms, rather
# than replacing system-owned state. RTC0 is never written.
existing=$(head -n 1 "$RTC/wakealarm")
case "$existing" in ''|0) ;; *) echo 'Existing RTC1 alarm; no changes.';say 'Existing alarm. Test cancelled.';exit 1;; esac
now=$(head -n 1 "$RTC/since_epoch")
number "$now" || exit 1
# Crucial: use RTC1's own time base, NOT Unix wall time from date +%s.
target=$((now+30))
say 'RTC1 check: stay awake for 45 seconds'
printf 'Arming RTC1 using its own epoch: now=%s target=%s\n' "$now" "$target"
if ! printf '%s\n' "$target" > "$RTC/wakealarm"; then
 echo 'RTC1 rejected alarm.';target=;say 'RTC1 alarm rejected. Check log.';exit 1
fi
snapshot
armed=$(head -n 1 "$RTC/wakealarm")
if [ "$armed" != "$target" ]; then
 echo 'Alarm read-back differs; not treating as success.';say 'RTC1 verification failed. Check log.';exit 1
fi
printf '\nAlarm read-back matched. Waiting without suspending.\n'
sleep 40
printf '\nAfter deadline\n'
snapshot
printf '\nRecent RTC kernel messages\n'
dmesg 2>/dev/null | tail -n 100 | grep -iE 'rtc|snvs|alarm' || true
state=$(lipc-get-prop com.lab126.powerd state 2>/dev/null) || state=unknown
printf 'Final power state: %s\n' "$state"
say 'RTC1 check done. Reconnect USB.'
echo 'This check cannot prove wake from deep sleep. Review IRQ counters first.'
