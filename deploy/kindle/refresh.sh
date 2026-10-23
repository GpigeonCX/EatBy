#!/bin/sh
# Single refresh, intentionally not a suspend-aware scheduler yet.
set -u
BASE=/mnt/us/eatby
[ -d /mnt/us ] && [ -e /proc/version ] || exit 1
umask 077
mkdir -p "$BASE/cache" "$BASE/logs"
# Atomic lock in /tmp (FAT user storage does not support Unix permissions).
mkdir /tmp/eatby-refresh.lock 2>/dev/null || exit 75
if [ -n "${EATBY_AGENT_OWNER:-}" ]; then printf '%s\n' "$EATBY_AGENT_OWNER" > /tmp/eatby-refresh.lock/agent-owned;fi
trap 'rm -f "$BASE/cache/download.png" "$BASE/cache/headers.tmp" /tmp/eatby-refresh.lock/agent-owned; rmdir /tmp/eatby-refresh.lock' EXIT
trap 'exit 1' HUP INT TERM
LOGDIR=${EATBY_LOG_DIR:-$BASE/logs}
mkdir -p "$LOGDIR"
exec > "$LOGDIR/refresh.txt" 2>&1
date -u
# Manual KUAL actions need closing time; the agent is already detached and has
# restored its cached frame before networking, so do not add another delay.
if [ -z "${EATBY_AGENT_OWNER:-}" ]; then sleep 3;fi
if [ ! -s "$BASE/image-url.txt" ]; then
 echo 'Missing image-url.txt: HTTPS /api/v1/display/<token>/image.png endpoint required.'
 exit 1
fi
URL=$(head -n 1 "$BASE/image-url.txt" | tr -d '\r\n')
case "$URL" in https://*) ;; *) echo 'HTTPS required'; exit 1;; esac
battery=$(lipc-get-prop com.lab126.powerd battLevel 2>/dev/null) || battery=
case "$battery" in ''|*[!0-9]*) ;; *)
 if [ "$battery" -le 100 ]; then
  case "$URL" in *\?*) URL="$URL&battery=$battery";; *) URL="$URL?battery=$battery";; esac
 fi;;
esac
# No redirect following, no insecure TLS, bounded connect/transfer/retries.
result=0
http=$(curl --fail --silent --show-error --connect-timeout 15 --max-time 60 --retry 1 --max-filesize 2000000 -w '%{http_code}' -D "$BASE/cache/headers.tmp" -o "$BASE/cache/download.png" "$URL") || result=$?
if [ "$result" != 0 ]; then
 echo "Download failed: curl=$result HTTP=$http; retaining cache."
 # Exit 69 means temporary connectivity/service failure, not invalid credentials.
 case "$result" in
  5|6|7|18|28|35|52|55|56) exit 69;;
  22) case "$http" in 408|429|5[0-9][0-9]) exit 69;; esac;;
 esac
 exit 1
fi
signature=$(od -An -tx1 -N8 "$BASE/cache/download.png" | tr -d ' \n')
geometry=$(dd if="$BASE/cache/download.png" bs=1 skip=16 count=8 2>/dev/null | od -An -tx1 | tr -d ' \n')
if [ "$signature" != 89504e470d0a1a0a ] || [ "$geometry" != 0000025800000320 ]; then
 echo 'Invalid PNG header or panel dimensions; retaining previous screen.'; exit 1
fi
# Render before replacing the last known-good cache. Native decoder checks PNG.
if ! eips -g "$BASE/cache/download.png"; then
 echo 'Image decoder failed.'
 [ ! -s "$BASE/cache/current.png" ] || eips -g "$BASE/cache/current.png"
 exit 1
fi
# Full flashing GC16 refresh reduces ghosting without erasing image pixels.
/mnt/us/libkh/bin/fbink -f -w -W GC16 -s
mv "$BASE/cache/download.png" "$BASE/cache/current.png"
mv "$BASE/cache/headers.tmp" "$BASE/cache/headers.txt"
printf 'Successfully displayed at '; date -u
