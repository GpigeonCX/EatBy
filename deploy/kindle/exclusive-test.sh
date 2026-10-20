#!/bin/sh
# Bounded experiments only. Recovery is independent of KUAL/framework.
set -u
BASE=/mnt/us/eatby
LOCK=/tmp/eatby-exclusive-test
SELF=$BASE/exclusive-test.sh
RAMLOG=/tmp/eatby-cycle-logs
export_logs() {
 [ -d "$BASE/logs" ] || return 0
 for name in exclusive-test.txt exclusive-watchdog.txt integrated-wake.txt refresh.txt; do
  [ ! -f "$RAMLOG/$name" ] || cp "$RAMLOG/$name" "$BASE/logs/$name" 2>/dev/null || true
 done
}
[ -d /mnt/us ] && [ -e /proc/version ] || exit 1
umask 077
running() { initctl status "${1:-framework}" 2>/dev/null | grep -q 'start/running'; }
notify() { /mnt/us/libkh/bin/fbink -q -m -y 12 "$1" || true; }
snapshot() {
 echo "--- display snapshot: $1 ---"
 if [ -f "$BASE/display-diagnostics.sh" ]; then timeout 5 sh "$BASE/display-diagnostics.sh" || true; fi
}
restore_service() {
 [ -f "$LOCK/stop-requested-$1" ] || return 0
 running "$1" && return 0
 n=0
 while [ "$n" -lt 2 ]; do
  n=$((n+1)); timeout 10 start "$1" || true; sleep 2
  if running "$1"; then echo "Service restored: $1";return 0;fi
 done
 echo "ERROR: automatic restoration failed: $1";return 1
}
restore() {
 result=0
 restore_service framework || result=1
 restore_service statusbar || result=1
 if [ "$result" -eq 0 ]; then touch "$LOCK/restored";fi
 return "$result"
}
stop_display() {
 [ -f "$LOCK/armed" ] && [ ! -f "$LOCK/restored" ] || return 1
 kill -0 "$(head -n 1 "$LOCK/watchdog.pid")" 2>/dev/null || return 1
 echo 'Recovery guard verified.'
 if [ "${EATBY_FULL_CYCLE:-0}" = 1 ]; then
  echo 'Keeping framework running; stopping only statusbar for integrated cycle.'
 else
  echo 'Stopping framework temporarily.'
 fi
 if running statusbar; then touch "$LOCK/statusbar-was-running" "$LOCK/stop-requested-statusbar";fi
 if [ "${EATBY_FULL_CYCLE:-0}" != 1 ]; then
  touch "$LOCK/stop-requested-framework"
  if ! timeout 10 stop framework; then echo 'Stop failed; aborting.';return 1;fi
  running && return 1
 fi
 if [ -f "$LOCK/statusbar-was-running" ]; then
  if running statusbar; then
   if ! timeout 10 stop statusbar; then echo 'Statusbar stop failed; aborting.';return 1;fi
  fi
  running statusbar && return 1
 fi
 return 0
}
finish_worker() {
 worker_result=$?
 recovery_result=0
 restore || recovery_result=1
 touch "$LOCK/worker-finished"
 echo "Worker finished: result=$worker_result recovery=$recovery_result"
 export_logs
 if [ "${EATBY_FULL_CYCLE:-0}" = 1 ]; then
  if [ "$worker_result" -eq 0 ] && [ "$recovery_result" -eq 0 ]; then
   notify 'EatBy cycle finished. Reconnect USB.'
  else
   notify 'EatBy test failed. Reconnect USB.'
  fi
 fi
}
case "${1:-launch}" in
 launch|cycle)
  if [ -d /tmp/eatby-agent ]; then notify 'Stop battery dashboard before testing.';exit 1;fi
  if [ "${1:-}" = cycle ]; then export EATBY_FULL_CYCLE=1;fi
  mkdir -p "$BASE/logs"
  [ -s "$BASE/cache/current.png" ] || { notify 'Refresh dashboard first.';exit 1; }
  for cmd in setsid timeout initctl start stop; do command -v "$cmd" >/dev/null 2>&1 || exit 1;done
  # Recover a completed OLD test's sleeping watchdog lock. Only a matching
  # watchdog process is killed, and only after services are already restored.
  if [ -d "$LOCK" ] && [ -f "$LOCK/restored" ] && [ ! -f "$LOCK/run-id" ] && running; then
   status_ok=1
   if [ -f "$LOCK/stop-requested-statusbar" ] && ! running statusbar; then status_ok=0;fi
   if [ "$status_ok" = 1 ]; then
    pid=$(head -n 1 "$LOCK/watchdog.pid" 2>/dev/null) || pid=
    case "$pid" in ''|*[!0-9]*) ;; *)
     if [ -r "/proc/$pid/cmdline" ]; then
      cmdline=$(tr '\000' ' ' < "/proc/$pid/cmdline")
      case "$cmdline" in *"$SELF watchdog"*) kill -KILL "$pid" 2>/dev/null || true;; esac
     fi;;
    esac
    # New guards also check run identity before acting on a reused directory.
    rm -rf "$LOCK"
   fi
  fi
  if ! mkdir "$LOCK" 2>/dev/null; then notify 'A test is still running or needs recovery.';exit 1;fi
  if ! running; then rmdir "$LOCK";notify 'Restore normal Kindle UI first.';exit 1;fi
  export EATBY_TEST_RUN="$(date +%s)-$$"
  printf '%s\n' "$EATBY_TEST_RUN" > "$LOCK/run-id"
  mkdir -p "$RAMLOG"
  rm -f "$RAMLOG/integrated-wake.txt" "$RAMLOG/refresh.txt"
  export EATBY_LOG_DIR="$RAMLOG"
  setsid sh "$SELF" watchdog </dev/null >"$RAMLOG/exclusive-watchdog.txt" 2>&1 &
  setsid sh "$SELF" worker </dev/null >"$RAMLOG/exclusive-test.txt" 2>&1 &
  ;;
 watchdog)
  echo $$ > "$LOCK/watchdog.pid"
  guard_seconds=150
  if [ "${EATBY_FULL_CYCLE:-0}" = 1 ]; then guard_seconds=600;fi
  date -u;echo "Independent recovery watchdog armed (${guard_seconds} seconds)."
  touch "$LOCK/armed"
  elapsed=0
  while [ "$elapsed" -lt "$guard_seconds" ]; do
   [ ! -f "$LOCK/worker-finished" ] || break
   sleep 2;elapsed=$((elapsed+2))
  done
  run=$(head -n 1 "$LOCK/run-id" 2>/dev/null) || run=
  [ "$run" = "${EATBY_TEST_RUN:-}" ] || exit 0
  if restore; then echo 'Recovery finished; releasing test lock.';rm -rf "$LOCK"
  else echo 'Restoration failed; lock retained.';fi
  export_logs
  ;;
 take-display)
  # Called only AFTER readyToSuspend and verified RTC alarm installation.
  run=$(head -n 1 "$LOCK/run-id" 2>/dev/null) || run=
  [ -n "$run" ] && [ "$run" = "${EATBY_TEST_RUN:-}" ] || exit 1
  stop_display
  ;;
 worker)
  trap finish_worker EXIT
  trap 'exit 1' HUP INT TERM
  sleep 4;date -u
  if [ ! -f "$LOCK/armed" ] || ! kill -0 "$(head -n 1 "$LOCK/watchdog.pid" 2>/dev/null)" 2>/dev/null; then
   echo 'No live watchdog; refusing test.';exit 1
  fi
  snapshot before-stop
  if [ "${EATBY_FULL_CYCLE:-0}" = 1 ]; then
   echo 'Starting one integrated suspend/wake/refresh cycle. Framework remains running throughout.'
   if ! mkdir /tmp/eatby-wake-test 2>/dev/null; then notify 'Another wake test is running.';exit 1;fi
   EATBY_RTC1_TEST=1 EATBY_KEEP_DASHBOARD=1 timeout 360 sh "$BASE/wake-test.sh" worker > "$RAMLOG/integrated-wake.txt" 2>&1
   cycle_result=$?;echo "Integrated cycle returned: $cycle_result"
   if [ "$cycle_result" -eq 0 ]; then sleep 10;fi
   exit "$cycle_result"
  fi
  stop_display || exit 1
  [ ! -f "$LOCK/restored" ] || exit 1
  sleep 2
  if ! eips -g "$BASE/cache/current.png"; then echo 'Render failed; aborting.';exit 1;fi
  /mnt/us/libkh/bin/fbink -f -w -W GC16 -s || true
  for elapsed in 30 60 90; do
   sleep 30
   [ ! -f "$LOCK/restored" ] || break
   date -u;echo "Observation sample: ${elapsed}s"
   initctl status framework || true;initctl status statusbar || true
   snapshot "after-${elapsed}s"
  done
  echo 'Test complete. Restoring original interface.'
  ;;
 *) exit 1;;
esac
