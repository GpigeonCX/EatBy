#!/bin/sh
# USB-assisted configuration; KUAL itself has no text-input widget.
set -eu
BASE=/mnt/us/eatby
[ -d /mnt/us ] && [ -e /proc/version ] || exit 1
umask 077
mkdir -p "$BASE/logs"
exec > "$BASE/logs/import-link.txt" 2>&1
notify() { /mnt/us/libkh/bin/fbink -m -y 8 "$1" || true; }
if [ ! -s "$BASE/display-link.txt" ]; then
 notify 'Missing eatby/display-link.txt'; exit 1
fi
link=$(head -n 1 "$BASE/display-link.txt" | tr -d '\r\n')
# Initial pairing is deliberately restricted to the deployed application host.
case "$link" in
 https://met.xin/display/*) token=${link#https://met.xin/display/};;
 https://met.xin/d/*) token=${link#https://met.xin/d/};;
 *) notify 'Use a met.xin display link'; exit 1;;
esac
case "$token" in ''|*[!A-Za-z0-9_-]*) notify 'Invalid link'; exit 1;; esac
[ ${#token} -ge 22 ] && [ ${#token} -le 64 ] || { notify 'Invalid token length'; exit 1; }
# Verify before replacing the working configuration. Avoid printing the URL.
if ! curl --fail --silent --connect-timeout 15 --max-time 30 -o /dev/null "https://met.xin/api/v1/display/$token/config"; then
 notify 'Link check failed; old link retained'; exit 1
fi
printf 'https://met.xin/api/v1/display/%s/image.png\n' "$token" > "$BASE/image-url.txt.new"
mv "$BASE/image-url.txt.new" "$BASE/image-url.txt"
notify 'Link saved. Use Refresh dashboard.'
printf 'Link verified and saved.\n'
