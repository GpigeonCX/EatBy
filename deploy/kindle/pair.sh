#!/bin/sh
set -u
BASE=/mnt/us/eatby
[ -d /mnt/us ] && [ -e /proc/version ] || exit 1
if [ "${1:-}" != worker ]; then
 nohup sh "$BASE/pair.sh" worker >/dev/null 2>&1 </dev/null &
 exit 0
fi
umask 077
mkdir /tmp/eatby-pair.lock 2>/dev/null || exit 0
trap 'rm -rf /tmp/eatby-pair.lock' EXIT
mkdir -p "$BASE/logs"
exec > "$BASE/logs/pair.txt" 2>&1
# Read-only information for designing a recoverable exclusive display mode.
printf 'Display recovery capabilities\n'
for command in setsid start stop initctl timeout; do command -v "$command" || true; done
if command -v initctl >/dev/null 2>&1; then initctl status framework || true; fi
sleep 3
FB=/mnt/us/libkh/bin/fbink
say() { "$FB" -c -m -y 14 "$1"; }
if ! curl -fsS --connect-timeout 15 --max-time 30 -X POST https://met.xin/api/v1/kindle-pairings -o /tmp/eatby-pair.lock/response; then
 say 'Pairing unavailable. Old link retained.';exit 1
fi
code=$(head -n 1 /tmp/eatby-pair.lock/response)
secret=$(tail -n 1 /tmp/eatby-pair.lock/response)
case "$secret" in ''|*[!A-Za-z0-9_-]*) say 'Invalid pairing response';exit 1;; esac
[ ${#secret} -eq 43 ] && [ ${#code} -eq 9 ] || exit 1
say 'EatBy device pairing'
"$FB" -m -y 18 "$code"
"$FB" -m -y 22 'Phone: met.xin > Settings > Kindle'
"$FB" -m -y 25 'Code valid for 10 minutes'
printf 'header = "Authorization: Bearer %s"\n' "$secret" > /tmp/eatby-pair.lock/curl.conf
n=0
while [ "$n" -lt 110 ]; do
 sleep 5;n=$((n+1))
 status=$(curl -s --config /tmp/eatby-pair.lock/curl.conf --connect-timeout 5 --max-time 10 -o /dev/null -w '%{http_code}' https://met.xin/api/v1/kindle-pairings/status) || continue
 case "$status" in
 200)
  printf 'https://met.xin/api/v1/display/%s/image.png\n' "$secret" > "$BASE/image-url.txt.new"
  mv "$BASE/image-url.txt.new" "$BASE/image-url.txt"
  printf 'https://met.xin/display/%s\n' "$secret" > "$BASE/display-link.txt"
  say 'Paired. Loading dashboard...'
  sh "$BASE/refresh.sh"
  exit $?;;
 410) break;;
 429) sleep 30;;
 esac
done
say 'Pairing expired. Old link retained.'
