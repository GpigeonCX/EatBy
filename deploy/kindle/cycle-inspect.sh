#!/bin/sh
# Snapshot only: never stop/start services, signal processes, or change alarms.
set -u
BASE=/mnt/us/eatby
[ -d /mnt/us ] && [ -e /proc/version ] || exit 1
umask 077
mkdir -p "$BASE/logs"
# Export RAM-first traces even if the previous worker exited before export.
for name in exclusive-test.txt exclusive-watchdog.txt integrated-wake.txt refresh.txt; do
 [ ! -f "/tmp/eatby-cycle-logs/$name" ] || cp "/tmp/eatby-cycle-logs/$name" "$BASE/logs/$name"
done
for name in agent.txt agent-guard.txt agent-wake.txt agent-wake.previous.txt refresh.txt; do
 [ ! -f "/tmp/eatby-agent-logs/$name" ] || cp "/tmp/eatby-agent-logs/$name" "$BASE/logs/$name"
done
exec > "$BASE/logs/cycle-inspect.txt" 2>&1
date -u
printf '\nUptime\n'; head -n 1 /proc/uptime
printf '\nServices\n'
for service in framework statusbar pillow lab126_gui powerd; do initctl status "$service" || true; done
printf '\nPower state\n'
lipc-get-prop com.lab126.powerd state || true
lipc-get-prop com.lab126.powerd status || true
printf '\nTest markers (no credentials)\n'
for dir in /tmp/eatby-exclusive-test /tmp/eatby-wake-test /tmp/eatby-agent; do
 if [ -d "$dir" ]; then
  ls -la "$dir"
  for name in run-id watchdog.pid armed restored worker-finished stop-requested-framework stop-requested-statusbar lease pid guard-pid rtc-target; do
   [ ! -f "$dir/$name" ] || { printf '%s: ' "$dir/$name"; head -c 128 "$dir/$name"; printf '\n'; }
  done
 else printf '%s absent\n' "$dir";fi
done
printf '\nMatching process states (arguments are NOT printed)\n'
for proc in /proc/[0-9]*; do
 [ -r "$proc/comm" ] || continue
 IFS= read -r name < "$proc/comm" || continue
 relevant=0
 case "$name" in fbink|eips|timeout|sleep) relevant=1;; esac
 if [ "$name" = sh ] && [ -r "$proc/cmdline" ]; then
  if tr '\000' '\n' < "$proc/cmdline" | grep -qE '^/mnt/us/eatby/(exclusive-test|wake-test|refresh|battery-agent)\.sh$'; then relevant=1;fi
 fi
 [ "$relevant" = 1 ] || continue
 printf '\npid=%s name=%s\n' "${proc##*/}" "$name"
 grep -E '^(Name|State|Pid|PPid|TracerPid):' "$proc/status" || true
 printf 'wait-channel: '; head -c 256 "$proc/wchan";printf '\n'
 # Kernel stack contains kernel symbols, not process argv or device secrets.
 if [ -r "$proc/stack" ]; then timeout 1 head -n 12 "$proc/stack" 2>/dev/null || true;fi
done
printf '\nRTC1\n'
for property in since_epoch wakealarm; do
 file="/sys/class/rtc/rtc1/$property"
 if [ -r "$file" ]; then printf '%s: ' "$property";head -n 1 "$file";fi
done
printf '\nRecent lifecycle/suspend/kernel messages\n'
if [ -r /var/log/messages ]; then
 tail -n 4000 /var/log/messages | grep -iE 'framework.*(start|stop|kill|exit|restart)|statusbar.*(start|stop|kill|exit|restart)|lab126_gui|oom|killed process|rtc|suspend|resume' | tail -n 160 || true
fi
dmesg 2>/dev/null | tail -n 200 | grep -iE 'PM:|suspend|resume|rtc|oom|killed process|epdc|framebuffer' || true
printf '\nDone: no runtime state changed.\n'
