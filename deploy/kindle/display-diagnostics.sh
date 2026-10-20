#!/bin/sh
# Read-only snapshot. Never prints process command-line arguments or credentials.
set -u
[ -d /mnt/us ] && [ -e /proc/version ] || exit 1
date -u
printf '\nService states\n'
initctl list 2>/dev/null || true
printf '\nDisplay-related process names and framebuffer holders\n'
for proc in /proc/[0-9]*; do
 [ -r "$proc/comm" ] || continue
 name=$(head -n 1 "$proc/comm" 2>/dev/null)
 case "$name" in
  *pillow*|*awesome*|*cvm*|*framework*|*chrome*|*lipc*|*waf*|*window*|*status*|*blanket*|X|Xorg|Xfbdev)
   printf 'pid=%s name=%s\n' "${proc##*/}" "$name";;
 esac
 for fd in "$proc"/fd/*; do
  target=$(readlink "$fd" 2>/dev/null) || continue
  case "$target" in /dev/fb*|/dev/graphics/*)
   printf 'framebuffer-holder pid=%s name=%s device=%s\n' "${proc##*/}" "$name" "$target";;
  esac
 done
done
printf '\nDisplay tools\n'
for tool in xwininfo xprop wmctrl awesome-client; do command -v "$tool" || true; done
# Record X window classes rather than titles (titles may contain private data).
if command -v xprop >/dev/null 2>&1; then
 DISPLAY=:0 timeout 2 xprop -root _NET_CLIENT_LIST _NET_ACTIVE_WINDOW 2>/dev/null || true
fi
printf '\nRelevant system job names\n'
for file in /etc/upstart/*.conf /etc/init/*.conf; do
 [ -r "$file" ] || continue
 if grep -q -E 'pillow|awesome|Xorg|Xfbdev|cvm|framework|statusbar' "$file"; then
  printf '%s\n' "$file"
  awk '/^[[:space:]]*(start on|stop on|respawn)/ {print} /^[[:space:]]*exec / {print "exec-command:", $2}' "$file"
 fi
done
printf '\nEnd snapshot\n'
