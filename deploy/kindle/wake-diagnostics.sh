#!/bin/sh
# Read-only investigation after unsuccessful suspend/wake. No alarm writes,
# suspend requests, service changes or USB/network configuration changes.
set -u
BASE=/mnt/us/eatby
[ -d /mnt/us ] && [ -e /proc/version ] || exit 1
umask 077
mkdir -p "$BASE/logs"
exec > "$BASE/logs/wake-diagnostics.txt" 2>&1
date -u
printf '\nKernel\n'; uname -a
printf '\nPowerd status\n'
lipc-get-prop com.lab126.powerd state || true
lipc-get-prop com.lab126.powerd status || true
printf '\nRTC devices\n'
ls -l /dev/rtc* /sys/class/rtc/ 2>/dev/null || true
for rtc in /sys/class/rtc/rtc*; do
 for property in name hctosys date time since_epoch wakealarm device/power/wakeup; do
  file="$rtc/$property"
  if [ -r "$file" ]; then printf '%s: ' "$file"; timeout 2 head -c 256 "$file"; printf '\n'; fi
 done
done
printf '\nRTC driver state\n'
[ ! -r /proc/driver/rtc ] || head -c 4096 /proc/driver/rtc
printf '\nPower capabilities (read only)\n'
for file in /sys/power/state /sys/power/mem_sleep /sys/power/pm_wakeup_irq /sys/power/wakeup_count; do
 if [ -r "$file" ]; then printf '%s: ' "$file"; timeout 2 head -c 256 "$file"; printf '\n'; fi
done
printf '\nrtcwake usage only (no alarms)\n'
if command -v rtcwake >/dev/null 2>&1; then timeout 2 rtcwake --help || true; fi
printf '\nRecent kernel power/USB messages\n'
dmesg 2>/dev/null | tail -n 600 | grep -iE 'rtc|suspend|resume|wakeup|usb|g_mass_storage' | tail -n 120 || true
printf '\nRecent system power events (may belong to post-reboot state)\n'
for file in /var/log/messages /var/log/messages.0; do
 if [ -r "$file" ]; then
  printf 'Source: %s\n' "$file"
  tail -n 2500 "$file" | grep -iE 'rtcWakeup|rtcWakeup2|InvalidState|readyToSuspend|suspend|resume|wakeup|g_mass_storage' | tail -n 100 || true
 fi
done
printf '\nRoot/userstore mount states\n'
awk '$2 == "/" || $2 == "/mnt/us" {print}' /proc/mounts
printf '\nDone: no settings changed.\n'
