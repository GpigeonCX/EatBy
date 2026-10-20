#!/bin/sh
# One-shot powerd RTC experiment. No permanent hooks, no forced suspend,
# no framework shutdown. Explicit EATBY_RTC1_TEST=1 selects a one-shot
# RTC1 sysfs alarm instead of the powerd alarm request.
set -u
BASE=/mnt/us/eatby
LOCK=/tmp/eatby-wake-test
[ -d /mnt/us ] && [ -e /proc/version ] || exit 1
umask 077
if [ "${1:-}" != worker ]; then
 [ ! -d /tmp/eatby-agent ] || exit 1
 mkdir -p "$BASE/logs"
 mkdir "$LOCK" 2>/dev/null || exit 1
 setsid sh "$BASE/wake-test.sh" worker </dev/null >"$BASE/logs/wake-test.txt" 2>&1 &
 exit 0
fi
listener=
rtc_target=
cleanup() {
 if [ -n "$rtc_target" ] && [ -r /sys/class/rtc/rtc1/wakealarm ]; then
  remaining=$(head -n 1 /sys/class/rtc/rtc1/wakealarm)
  if [ "$remaining" = "$rtc_target" ]; then printf '0\n' > /sys/class/rtc/rtc1/wakealarm || true; fi
 fi
 if [ -n "$listener" ]; then kill "$listener" 2>/dev/null || true; wait "$listener" 2>/dev/null || true; fi
 rm -f "$LOCK/event.txt"
 if [ "${EATBY_AGENT:-0}" = 1 ]; then rm -f "$LOCK/agent-owned";fi
 rmdir "$LOCK" 2>/dev/null || true
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
sleep 4
FB=/mnt/us/libkh/bin/fbink
say() { "$FB" -q -c -m -y 14 "$1" || true; }
paint_cached() {
 # One bounded partial refresh, never a polling loop or powerd call.
 [ -s "$BASE/cache/current.png" ] || return 0
 if timeout 5 eips -g "$BASE/cache/current.png"; then
  echo "Cached dashboard restored: $1"
 else
  echo "Cache redraw skipped/failed: $1; normal refresh remains available."
 fi
}
wake_if_needed() {
 current=$(lipc-get-prop com.lab126.powerd state 2>/dev/null) || current=unknown
 if [ "$current" = active ]; then
  echo 'Already active; not issuing wakeUp.'
 elif [ "$current" = screenSaver ]; then
  lipc-set-prop -i com.lab126.powerd wakeUp 1 || true
 else
  echo "Not forcing wakeUp in unverified state: $current"
 fi
}
date -u
printf 'Before: '; lipc-get-prop com.lab126.powerd state || true
printf '\nPowerd status\n'; lipc-get-prop com.lab126.powerd status || true
now=$(date +%s)
# Do not knowingly replace an existing scheduled wake.
for f in /sys/class/rtc/rtc0/wakealarm /sys/class/rtc/rtc1/wakealarm; do
 [ -r "$f" ] || continue
 alarm=$(head -n 1 "$f")
 printf 'Existing alarm %s: %s\n' "$f" "$alarm"
 case "$alarm" in ''|*[!0-9]*) continue;; esac
 if [ "$alarm" -gt "$now" ]; then
  say 'Existing RTC alarm. Test cancelled.'
  echo 'Refusing to replace existing future alarm.';exit 1
 fi
done
# Also refuse to test under a known screen-saver inhibition setting.
blocked=$(lipc-get-prop com.lab126.powerd preventScreenSaver 2>/dev/null) || blocked=unknown
printf 'preventScreenSaver=%s\n' "$blocked"
if [ "$blocked" != 0 ]; then say 'Sleep inhibited. Test cancelled.';exit 1;fi
# rtcWakeup was rejected in active state on this firmware. Subscribe BEFORE
# prompting for sleep; arm only after powerd announces readyToSuspend.
if ! command -v lipc-wait-event >/dev/null 2>&1; then
 say 'Missing sleep-event tool. See log.';exit 1
fi
printf '\nWaiting for readyToSuspend (120 second limit).\n'
timeout 125 lipc-wait-event -s 120 com.lab126.powerd readyToSuspend > "$LOCK/event.txt" 2>&1 &
listener=$!
if [ "${EATBY_AGENT:-0}" = 1 ]; then
 sleep 1
 current=$(lipc-get-prop com.lab126.powerd state 2>/dev/null) || current=unknown
 if [ "$current" = active ]; then
  if ! lipc-set-prop -i com.lab126.powerd powerButton 1; then echo 'Automatic sleep request failed';exit 1;fi
 elif [ "$current" != screenSaver ]; then echo "Unsupported sleep-entry state: $current";exit 1;fi
 # Let the native sleep transition draw first, then cover it once. Keep the
 # readyToSuspend redraw below as a fallback if native UI paints again.
 sleep 2
 paint_cached before-suspend-event
else
 say 'Wake test: press power ONCE now'
 "$FB" -q -m -y 18 'Wait up to 3 minutes after pressing.' || true
 "$FB" -q -m -y 22 'Do not reconnect USB during test.' || true
fi
if ! wait "$listener"; then
 listener=
 head -c 2048 "$LOCK/event.txt"
 echo 'No usable sleep event; no alarm was set.'
 say 'No sleep event. Reconnect USB.';exit 1
fi
listener=
head -c 2048 "$LOCK/event.txt"
# The subscription is already restricted to readyToSuspend. On this firmware
# lipc-wait-event returns only the payload (observed: "10 "), not the name.
# Accept a bounded numeric payload or a named event; reject empty timeout and
# unexpected diagnostics even when the command exits zero.
payload=$(tr -d ' \t\r\n' < "$LOCK/event.txt")
event_ok=0
case "$payload" in
 ''|*[!0-9]*) ;;
 *) if [ ${#payload} -le 6 ]; then event_ok=1; fi;;
esac
if grep -Eq '^readyToSuspend([[:space:]:,]|$)' "$LOCK/event.txt"; then event_ok=1; fi
if [ "$event_ok" -ne 1 ]; then
 echo 'Empty or unexpected event output; no alarm was set.'
 say 'Sleep event missing. Reconnect USB.';exit 1
fi
echo 'readyToSuspend subscription delivered a recognized payload.'
printf '\nState at event: '; lipc-get-prop com.lab126.powerd state || true
delay=90
if [ "${EATBY_AGENT:-0}" = 1 ]; then
 case "${EATBY_NEXT_LOCAL:-}" in ''|*[!0-9]*) exit 1;; esac
 delay=$((EATBY_NEXT_LOCAL-$(date +%s)))
 [ "$delay" -ge 10 ] || delay=10
 [ "$delay" -le 691200 ] || exit 1
fi
if [ "${EATBY_RTC1_TEST:-0}" = 1 ]; then
 rtc=/sys/class/rtc/rtc1
 name=$(head -n 1 "$rtc/name" 2>/dev/null)
 enabled=$(head -n 1 "$rtc/device/power/wakeup" 2>/dev/null)
 case "$name" in *snvs*) ;; *) echo 'Unexpected RTC1';exit 1;; esac
 [ "$enabled" = enabled ] || { echo 'RTC1 not wake-enabled';exit 1; }
 existing=$(head -n 1 "$rtc/wakealarm")
 case "$existing" in ''|0) ;; *) echo 'RTC1 already has an alarm; refusing overwrite';exit 1;; esac
 rtc_now=$(head -n 1 "$rtc/since_epoch")
 case "$rtc_now" in ''|*[!0-9]*) echo 'Invalid RTC1 time';exit 1;; esac
 rtc_target=$((rtc_now+delay))
 if [ "${EATBY_AGENT:-0}" = 1 ]; then printf '%s\n' "$rtc_target" > /tmp/eatby-agent/rtc-target;fi
 printf 'RTC1 baseline: epoch=%s target=%s\n' "$rtc_now" "$rtc_target"
 grep -iE 'rtc|snvs|bd718' /proc/interrupts || true
 if ! printf '%s\n' "$rtc_target" > "$rtc/wakealarm"; then echo 'RTC1 alarm write failed';exit 1;fi
 actual=$(head -n 1 "$rtc/wakealarm")
 [ "$actual" = "$rtc_target" ] || { echo 'RTC1 alarm read-back failed';exit 1; }
 echo 'RTC1 alarm verified. No powerd rtcWakeup call made.'
else
 if ! lipc-set-prop -i com.lab126.powerd rtcWakeup 90; then
  echo 'Wake request rejected even after readyToSuspend; needs further diagnosis.'
  wake_if_needed
  say 'RTC request rejected. Reconnect USB.';exit 1
 fi
fi
start=$(date +%s);due=$((start+delay))
printf 'Requested wake in %s seconds: start=%s due=%s\n' "$delay" "$start" "$due"
for f in /sys/class/rtc/rtc0/wakealarm /sys/class/rtc/rtc1/wakealarm; do
 if [ -r "$f" ]; then printf 'RTC immediately after request %s: ' "$f"; head -n 1 "$f"; fi
done
# Repaint once after screensaver preparation in the integrated test.
if [ "${EATBY_KEEP_DASHBOARD:-0}" = 1 ]; then
 echo 'Sleep event and RTC verified; now taking exclusive display.'
 if [ "${EATBY_AGENT:-0}" != 1 ]; then
  if ! sh "$BASE/exclusive-test.sh" take-display; then echo 'Exclusive display failed';exit 1;fi
 fi
 if ! eips -g "$BASE/cache/current.png"; then echo 'Cannot preserve dashboard';exit 1;fi
 /mnt/us/libkh/bin/fbink -f -w -W GC16 -s || true
fi
# No LIPC polling, screen drawing or log writes during the wait. The previous
# test stayed readyToSuspend; its own activity may have disturbed powerd.
# timer_list exposes CLOCK_MONOTONIC (excludes suspend), while /proc/uptime
# includes suspend. Capture each once before/after, not on every iteration.
mono_before=
if [ -r /proc/timer_list ]; then mono_before=$(timeout 2 awk '/^now at / {print $3; exit}' /proc/timer_list); fi
boot_before=$(awk '{printf "%.0f", $1}' /proc/uptime)
previous=$start
maxgap=0
samples=0
# Only a short sleep and wall-clock read in this loop; all observations stay
# in memory. A gap alone is still not proof of deep sleep.
while :; do
 samples=$((samples+1))
 if [ "$samples" -gt $((delay/5+120)) ]; then
  say 'Clock changed. Test ended.';echo 'Exceeded sample limit.';exit 1
 fi
 now=$(date +%s)
 gap=$((now-previous));previous=$now
 [ "$gap" -le "$maxgap" ] || maxgap=$gap
 [ "$now" -lt "$due" ] || break
 sleep 5
done
mono_after=
if [ -r /proc/timer_list ]; then mono_after=$(timeout 2 awk '/^now at / {print $3; exit}' /proc/timer_list); fi
boot_after=$(awk '{printf "%.0f", $1}' /proc/uptime)
late=$((now-due))
printf 'Quiet wait: samples=%s lateness=%ss max_gap=%ss\n' "$samples" "$late" "$maxgap"
printf 'Clocks: monotonic_ns=%s,%s boottime_seconds=%s,%s\n' "$mono_before" "$mono_after" "$boot_before" "$boot_after"
case "$mono_before:$mono_after" in
 :*|*:|*[!0-9:]*) echo 'Monotonic evidence unavailable; inspect kernel suspend/resume logs.';;
 *) active_seconds=$(((mono_after-mono_before)/1000000000))
    suspended_estimate=$((boot_after-boot_before-active_seconds))
    printf 'Estimated suspended duration: %ss (corroborate with kernel logs)\n' "$suspended_estimate";;
esac
# Ask the normal UI to leave its screensaver before indicating completion.
# Do not clear someone else's alarm or change sleep-inhibition properties.
wake_if_needed
if [ "${EATBY_AGENT:-0}" = 1 ]; then
 sleep 1
 paint_cached after-wake
else
 sleep 3
fi
if [ "$late" -gt 30 ]; then
 if [ "${EATBY_KEEP_DASHBOARD:-0}" != 1 ]; then say 'Wake was late. Please reconnect USB.'; fi
 echo 'Late wake: automatic RTC wake is NOT established.'
else
 if [ "${EATBY_KEEP_DASHBOARD:-0}" != 1 ]; then say 'Wake test finished. Reconnect USB.'; fi
 echo 'Deadline met. Need user observation and power-state log to assess suspend.'
fi
printf '\nAfter\n';date -u
if [ -n "$rtc_target" ]; then
 grep -iE 'rtc|snvs|bd718' /proc/interrupts || true
 dmesg 2>/dev/null | tail -n 150 | grep -iE 'rtc|suspend|resume|wakeup|PM:' || true
fi
lipc-get-prop com.lab126.powerd state || true
lipc-get-prop com.lab126.powerd status || true
if [ "${EATBY_KEEP_DASHBOARD:-0}" = 1 ] && [ "${EATBY_AGENT:-0}" != 1 ]; then
 echo 'Refreshing real dashboard after wake.'
 sh "$BASE/refresh.sh"
 exit $?
fi
