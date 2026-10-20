#!/bin/sh
# Battery agent v1: no autostart; only statusbar is stopped. Framework stays up.
set -u
BASE=/mnt/us/eatby
RUN=/tmp/eatby-agent
LOG=/tmp/eatby-agent-logs
SELF=$BASE/battery-agent.sh
[ -d /mnt/us ] && [ -e /proc/version ] || exit 1
umask 077
notice() { /mnt/us/libkh/bin/fbink -q -m -y 12 "$1" || true; }
lease_until() { printf '%s\n' "$1" > "$RUN/lease.new" && mv "$RUN/lease.new" "$RUN/lease"; }
running_bar() { initctl status statusbar 2>/dev/null | grep -q start/running; }
export_logs() {
 mkdir -p "$BASE/logs" 2>/dev/null || return 0
 for f in agent.txt agent-guard.txt agent-wake.txt agent-wake.previous.txt refresh.txt; do
  [ ! -f "$LOG/$f" ] || cp "$LOG/$f" "$BASE/logs/$f" 2>/dev/null || true
 done
}
identity() {
 pid=$(head -n 1 "$RUN/pid" 2>/dev/null) || return 1
 birth=$(head -n 1 "$RUN/birth" 2>/dev/null) || return 1
 case "$pid:$birth" in *[!0-9:]*|:*|*:) return 1;; esac
 [ -r "/proc/$pid/stat" ] || return 1
 [ "$(awk '{print $22}' "/proc/$pid/stat")" = "$birth" ]
}
restore() {
 if [ -f "$RUN/rtc-target" ]; then
  target=$(head -n 1 "$RUN/rtc-target")
  current=$(head -n 1 /sys/class/rtc/rtc1/wakealarm 2>/dev/null) || current=
  if [ -n "$target" ] && [ "$target" = "$current" ]; then printf '0\n' > /sys/class/rtc/rtc1/wakealarm || true;fi
 fi
 ok=1
 if [ -f "$RUN/restore-statusbar" ]; then
  n=0
  while ! running_bar && [ "$n" -lt 3 ]; do n=$((n+1));timeout 10 start statusbar || true;sleep 2;done
  running_bar || ok=0
 fi
 # Only remove a wake-helper lock that this agent created.
 if [ -f /tmp/eatby-wake-test/agent-owned ]; then rm -rf /tmp/eatby-wake-test;fi
 if [ -f /tmp/eatby-refresh.lock/agent-owned ] && [ "$(head -n 1 /tmp/eatby-refresh.lock/agent-owned)" = "$RUN" ]; then
  rm -rf /tmp/eatby-refresh.lock
 fi
 if [ "$ok" = 1 ]; then echo 'Statusbar restored; agent stopped.';export_logs;rm -rf "$RUN"
 else echo 'Restore failed; state retained for recovery.';export_logs;return 1;fi
}
case "${1:-start}" in
 start)
  [ -s "$BASE/image-url.txt" ] || { notice 'Pair device first.';exit 1; }
  if [ -d /tmp/eatby-exclusive-test ] || [ -d /tmp/eatby-wake-test ]; then notice 'Finish the diagnostic test first.';exit 1;fi
  if ! mkdir "$RUN" 2>/dev/null; then notice 'Agent already active. Use Stop first.';exit 1;fi
  mkdir -p "$LOG"
  printf '%s\n' $(( $(date +%s)+300 )) > "$RUN/lease"
  setsid sh "$SELF" guard </dev/null > "$LOG/agent-guard.txt" 2>&1 &
  setsid sh "$SELF" worker </dev/null >> "$LOG/agent.txt" 2>&1 &
  ;;
 stop)
  if [ ! -d "$RUN" ]; then notice 'Battery dashboard is not running.';exit 0;fi
  touch "$RUN/stop"
  notice 'Stopping battery dashboard...'
  ;;
 guard)
  echo $$ > "$RUN/guard-pid"
  sleep 5
  while :; do
   reason=
   if [ -f "$RUN/stop" ]; then reason='stop requested'
   elif ! identity; then reason='worker exited'
   else
    lease=$(head -n 1 "$RUN/lease")
    case "$lease" in ''|*[!0-9]*) reason='invalid lease';; *) [ "$(date +%s)" -le "$lease" ] || reason='worker deadline exceeded';; esac
   fi
   [ -z "$reason" ] || break
   sleep 5
  done
  date -u;echo "$reason"
  if identity; then
   # Worker owns a setsid session; never signal an unrelated/reused PID.
   session=$(awk '{print $6}' "/proc/$pid/stat")
   if [ "$session" = "$pid" ]; then kill -TERM "-$pid" 2>/dev/null || true;else kill -TERM "$pid" 2>/dev/null || true;fi
   sleep 2
   if identity; then
    if [ "$session" = "$pid" ]; then kill -KILL "-$pid" 2>/dev/null || true;else kill -KILL "$pid" 2>/dev/null || true;fi
   fi
  fi
  if restore; then notice 'Battery dashboard stopped.';else notice 'Restore failed. Check agent log.';fi
  ;;
 worker)
  echo $$ > "$RUN/pid"
  awk '{print $22}' /proc/$$/stat > "$RUN/birth"
  trap 'export_logs' EXIT
  trap 'exit 1' HUP INT TERM
  sleep 3
  kill -0 "$(head -n 1 "$RUN/guard-pid")" 2>/dev/null || exit 1
  if running_bar; then touch "$RUN/restore-statusbar";timeout 10 stop statusbar || exit 1;fi
  export EATBY_LOG_DIR="$LOG" EATBY_AGENT_OWNER="$RUN"
  cycles=0
  while [ ! -f "$RUN/stop" ]; do
   cycles=$((cycles+1))
   if [ $((cycles%50)) -eq 0 ]; then : > "$LOG/agent.txt";fi
   lease_until $(( $(date +%s)+300 )) || exit 1
   date -u;echo "Refresh cycle $cycles"
   # Two bounded attempts; on sustained outage exit safely, retaining cache.
   if ! sh "$BASE/refresh.sh"; then
    sleep 10
    if ! sh "$BASE/refresh.sh"; then echo 'Network/render failure; leaving agent mode.';exit 1;fi
   fi
   server=$(awk 'tolower($1)=="x-eatby-server-time:" {gsub("\r", "", $2); print $2}' "$BASE/cache/headers.txt" | tail -n 1)
   next=$(awk 'tolower($1)=="x-eatby-next-refresh:" {gsub("\r", "", $2); print $2}' "$BASE/cache/headers.txt" | tail -n 1)
   case "$server:$next" in *[!0-9:]*|:*|*:) echo 'Invalid scheduling headers';exit 1;; esac
   if [ "$next" = 0 ]; then echo 'Server schedule paused; leaving agent mode.';exit 0;fi
   interval=$((next-server))
   [ "$interval" -gt 0 ] && [ "$interval" -le 691200 ] || exit 1
   # Schedule from receipt time, conservatively late by transfer/render time,
   # never early. Each response recalculates wall-clock slots, avoiding drift.
   due=$(( $(date +%s)+interval ))
   lease_until $((due+600)) || exit 1
   echo "Next local deadline=$due (server slot=$next)"
   export_logs
   if [ $((due-$(date +%s))) -lt 150 ]; then
    # Too little time for normal suspend preparation. Do not miss the slot.
    while [ "$(date +%s)" -lt "$due" ]; do [ ! -f "$RUN/stop" ] || exit 0;sleep 5;done
   else
    mkdir /tmp/eatby-wake-test 2>/dev/null || exit 1
    touch /tmp/eatby-wake-test/agent-owned
    [ ! -f "$LOG/agent-wake.txt" ] || cp "$LOG/agent-wake.txt" "$LOG/agent-wake.previous.txt"
    EATBY_AGENT=1 EATBY_RTC1_TEST=1 EATBY_KEEP_DASHBOARD=1 EATBY_NEXT_LOCAL="$due" sh "$BASE/wake-test.sh" worker > "$LOG/agent-wake.txt" 2>&1 || exit 1
   fi
  done
  ;;
 *) exit 1;;
esac
