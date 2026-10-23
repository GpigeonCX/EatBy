# Kindle 8 integration

## Offline recovery staged after observed DNS-exit failure (2026-10-21)

Device confirms prior cached-frame update installed. Latest wake: estimated
3459s suspended, RTC1 IRQ 22→23, deadline +1s, cached repaint before suspend
and after wake. At 13:00:15 UTC refresh failed with curl (6) DNS resolution;
13:00:17 guard restored statusbar after worker exited. Thus sustained native
screensaver can result from intentionally leaving dashboard mode on transient
network failure, not from failure to enter deep sleep. Earlier cycles show the
same failure; USB attachment also previously caused stale file handles.

Refresh now returns 69 only for selected transient curl failures, HTTP 408/429
or 5xx. Auth/not-found/bad TLS/invalid PNG remain terminal. Agent retries once,
then if a decoded cached frame is available, retains it and uses the existing
RTC sleep path to retry in 30 minutes. Does not reuse expired schedule headers,
busy-poll networking, inhibit suspend or touch blanket/framework. Offline retries
may fall outside normal scheduled windows; schedule resumes on next good response.
Stop and watchdog restoration unchanged. Cache timestamp remains old, honestly
indicating data age. No cache/invalid image still exits instead of a blank kiosk.

36 mocks pass: transient/permanent classification, cache/header preservation,
multiple offline sleep cycles, recovery, Stop and missing/bad cache, plus existing
RTC and guard cases. Three-file package staged via existing safe installer, KUAL
`APPLY offline recovery update`; backups in backup-network-v2. No active device
scripts overwritten; new offline behavior awaits physical installation/test.

## Cached-frame transition optimization staged

User observed two automatic cycles working, but native screensaver remained
visible during ~60s sleep preparation and while awaiting fresh data on wake.
Agent log confirms cycles at 08:05/08:10 UTC; one cycle estimated 171s mem sleep.

Updated wake helper in agent mode paints cached PNG once two seconds after the
sleep request, retains the existing readyToSuspend redraw, then paints cache
again one second after wake handling. Each extra redraw is bounded to five
seconds and uses native partial update, not an additional flashing GC16 update.
No display/IPC polling during sleep; no screen-saver-inhibition property changes.
Missing cache or redraw failure falls back to prior behavior, not a failed wake.
Agent refresh skips the manual KUAL-closing three-second delay. Manual refresh
still keeps that delay. Both orientations reuse the existing panel-ready cache.
This aims to shorten screensaver visibility; zero native transition flash is
NOT guaranteed. Physical visual quality and continued deep sleep await checking.

Live files were NOT overwritten while agent may be running. Updated wake-test.sh
and refresh.sh + SHA256SUMS staged in eatby/display-update. KUAL action
`EatBy: APPLY display optimization` validates package, asks running agent to stop,
waits up to 60 seconds, aborts if any test/refresh remains, backs up previous
files to eatby/backup-display-v1, installs via per-file rename, and restarts only
if agent was running before update. Device URL/schedule unchanged. Repeated
installation keeps the first backup. Thirty mocked tests pass, including exactly
two best-effort extra redraws, redraw failure fallback, checksum failure, stop
timeout, backups and conditional restart. Device installation still awaits click.

## Battery agent v1 staged (not yet hardware-qualified)

KUAL: `EatBy: START battery dashboard` / `EatBy: STOP battery dashboard`.
`battery-agent.sh` keeps framework alive, stops only initially-running statusbar,
refreshes once on explicit start, then reads next-refresh/server-time HTTP
headers from the real image endpoint. Uses server-relative timing conservatively
late by transfer time, not an interval that drifts from each completed refresh.
No config/network polling while waiting. New config applies on next connection.
Next=0 (paused schedule) exits mode at next response. Starting explicitly may
refresh once even outside the scheduled windows.

For slots >=150s away, wake helper subscribes to readyToSuspend, requests normal
sleep via powerd powerButton=1 (new automatic step, needs physical validation),
then programs RTC1 using its own epoch + remaining delay and preserves cached
image. Shorter gaps are waited awake because normal suspend preparation is too
slow. Only silent sleep/date reads while waiting. Normal repeated cycles retain
statusbar suppression. No framework shutdown, clock changes or startup hooks.

Guard uses worker PID plus /proc start-time identity, separate setsid process,
and a lease through next deadline +600s. On Stop, worker exit or lease expiry,
it terminates only the recorded worker/session, clears only matching agent RTC
alarm/owned temporary locks, restores statusbar if originally running, exports
RAM logs and removes state. Stop is a KUAL action: wake normally, open KUAL,
select Stop, wait for stopped message BEFORE reconnecting USB. Hardware stop
responsiveness/session termination and long sleep retention still need testing.

Download failure: two bounded refresh rounds (each curl may internally retry
once), then exit mode safely instead of continuous offline wake/retry. This v1
requires manual Start after sustained network failure or reboot; it is NOT
unattended reboot recovery. RAM logs lost on reboot; exported logs retained.
No claim of measured battery life or multi-day stability yet.

Logs: agent.txt, agent-guard.txt, agent-wake.txt, agent-wake.previous.txt,
refresh.txt. `inspect last cycle` also exports agent RAM logs. Mock tests cover
schedule-driven sleep then pause, bounded failures, guard restoration, automatic
sleep request, RTC timing/ownership and previous safety cases (25 total passed).
Current paired server schedule verified: weekdays 18:00–24:00, weekends
09:00–24:00 every 30 minutes, Asia/Shanghai; next slot 2026-10-20 18:00.

## Integrated battery cycle verified on hardware (2026-10-20 15:40)

Latest run succeeded with FRAMEWORK LEFT RUNNING and only statusbar stopped.
- readyToSuspend received, RTC1 own epoch 4852→4942 verified.
- Kernel entered mem sleep and resumed; clock difference estimates 35s asleep.
- RTC1 IRQ 4→5; wait completed one second after target.
- Real HTTPS dashboard downloaded and rendered at 07:40:55 UTC.
- Integrated child exit=0, recovery=0; statusbar restored.
- Watchdog finished and removed lock; RAM-first logs exported successfully.

Do not regress to stopping framework for the integrated battery mode. Native
framework is retained for power-key and suspend/resume lifecycle handling.
This validates ONE manually-initiated 90-second alarm cycle, not the weekly
schedule or automatic repeated suspend. Remaining production work: automatic
sleep initiation, schedule-driven RTC deadlines, repeated loop, user Stop menu,
bounded offline recovery, long-session watchdog leases, and reboot policy.
No indefinite mode or startup hook has been enabled.

## Integrated cycle revised to statusbar-only; RAM-first traces

Runtime inspection found normal services running, no test/wake locks and no
matching workers. Previous logs still lack child completion/recovery; no new
refresh happened. Cannot establish cause of exit or count integrated success.

New integrated take-display stops ONLY statusbar, keeping framework alive for
normal input/suspend/resume. Generic standalone exclusive test still stops both.
Statusbar-only integrated screen retention/overlay behavior is unverified.
Worker/guard/refresh traces now go to /tmp/eatby-cycle-logs first; worker/guard
export them to USB storage on completion, and cycle-inspect explicitly exports
remaining RAM traces. RAM data is lost on reboot; this is diagnostic logging,
not persistent crash recovery. No claim USB caused missing traces is made.

At exit, worker now shows explicit `EatBy cycle finished. Reconnect USB.` or
`EatBy test failed. Reconnect USB.` after restoration/export. Returning to the
native desktop alone is NOT a completion indicator. Eight exclusive-control
mocked tests pass, including framework staying alive in integrated take-display.
Updated scripts staged, physical integrated test still pending.

## Revised integrated run: automatic desktop, incomplete trace

User confirms desktop returned without power-button or USB interaction. Logs:
readyToSuspend received, RTC1 target installed, then framework/statusbar stopped
and cached image submitted. Trace ends at FBInk flashing refresh; no quiet-wait,
post-wake download, child return or service-restoration records. Refresh log is
still from a previous run. Do NOT count this as integrated success or assume
FBInk blocked solely from the last line; runtime evidence is needed.

Staged `EatBy: inspect last cycle (read only)`, bounded to 25 seconds. Captures
service states, non-secret test markers, selected process states/wait channels/
kernel stacks (not argv), RTC1 and recent lifecycle/PM logs. Does not signal
processes or modify services/alarms. User must not repeat suspend test before
reviewing this snapshot. Existing timing/retention tests remain experimental.

## Integrated ordering and retry fix

First integrated run failed: framework/statusbar stopped BEFORE asking user to
press power; no readyToSuspend event arrived within timeout. Child returned 1;
both services restored. User reports power button had no visible effect.
Suspected ordering interference, not proven firmware causation. No RTC alarm
was armed in that run. Subsequent taps were silently rejected because old
watchdog retained the mutex for the full 600-second guard period.

Updated cycle leaves framework running through user power press and event wait.
Only after readyToSuspend and verified RTC1 alarm does authenticated internal
`take-display` action stop the two services and redraw. Recovery remains
independent. Worker always signals completion; guard then restores and removes
lock promptly instead of waiting full ten minutes. Busy launches show a message.
Legacy completed locks (restored marker, no run-id) are reclaimed only when
framework and any recorded statusbar are running, killing only a matching old
watchdog command. New guards use run identity; new-format lock cleanup remains
owned by the guard, avoiding launch/cleanup races. No reboot should be needed
for the previously observed completed legacy lock.

Twenty-one mocked tests pass, including UI-running-at-event-wait ordering,
restoration on integrated failure, and retry after completed legacy lock.
Updated scripts staged on Kindle; revised integrated hardware cycle unverified.

## Physical deep sleep + RTC1 wake verified

Quiet RTC1 test: requested epoch 1792479050, completion observed +1 second.
Monotonic elapsed ~56s vs boot-time elapsed 91s, estimated suspend 35s.
Kernel logged Preparing/Entering mem sleep and device resume/Finishing wakeup.
RTC1 IRQ increased 2→3; user observed automatic completion. This validates one
normal-UI deep-sleep/RTC1 wake, not yet the full exclusive dashboard loop.

Staged integrated test: `EatBy: TEST one battery dashboard cycle`.
Stops previously-running framework/statusbar with independent recovery guard,
asks user to press power once, waits for readyToSuspend, arms RTC1, redraws
cached dashboard once after screensaver preparation, quietly waits for wake,
then runs actual network refresh. After ten seconds restores both services.
Normal duration around three minutes plus network wait. Child command bounded
to 360 active seconds, independent recovery guard at 600 active seconds; those
timers can exclude suspend time. Emergency recovery remains normal power reboot.
No autostart or permanent suspend-inhibition changes. Twenty mocked tests pass,
including integrated-child success/failure restoration. Physical integrated
screen retention, post-wake Wi-Fi and refresh remain to be validated. This is
still a one-cycle test, NOT the recurring weekly-scheduled agent.

## RTC1 prompt appeared, but deep sleep NOT established

User observed automatic completion. Detailed log shows every sample stayed
readyToSuspend, maximum sample gap six seconds, and no kernel system-suspend
entry/resume evidence. RTC1 interrupt increased 1→2 and deadline was met within
one second. This confirms alarm timing and prompt only, NOT battery-efficient
suspend/wake. Do not mark RTC integration complete.

Updated wake test removes all powerd IPC, RTC reads and log writes from its wait
loop; only sleep and date run there. Frequent powerd queries are a suspected
interference source, not a proven cause. Captures optional timer_list monotonic
and /proc/uptime boot-time clocks before/after and kernel PM messages to assess
suspend. Nine mock tests pass, including a limit on total powerd reads to prevent
polling regression. Quiet test staged; still needs physical verification.

## RTC1 awake alarm verified; sleep test staged

Physical awake test passed: RTC1 epoch 951, alarm 981, after 40 seconds epoch
991 and empty wakealarm. IRQ 51 (`GIC rtc alarm`) incremented 0→1; PMIC RTC
counter unchanged. Device stayed active. This proves SNVS alarm firing while
awake, NOT wake from suspend or successful powerd alarm handling.

New menu: `EatBy: TEST RTC1 sleep wake (90 sec)` invokes wrapper enabling
EATBY_RTC1_TEST in wake-test.sh. Waits for the observed payload-only
readyToSuspend event, checks SNVS identity/wakeup flag/existing alarm, then
sets RTC1 alarm to its OWN current epoch +90, verifies read-back and logs IRQs.
Does not call powerd rtcWakeup, change RTC clocks, enable flags or USB settings.
Cleanup clears only an alarm still matching its own target. No autostart.
Nine wake-script mocked tests pass, including RTC1 own-clock conversion,
existing-alarm preservation and rejecting disabled wakeup. Physical sleep wake
still awaits test. If no automatic completion after three minutes, manually
wake and collect logs. Previous USB issue remains unresolved; no reliability
claim should be made until sleep/resume/USB are all tested.

## Battery operation selected; awake RTC1 check staged

User explicitly requires primarily battery operation. Always-awake mode is not
an acceptable substitute. Production scheduling remains blocked on validated
suspend/wake and safe exit handling.

New KUAL experiment: `EatBy: TEST RTC1 while awake (45 sec)`. Device stays awake;
no framework, powerd, USB, suspend or clock-setting changes. It writes ONE
30-second rtc1/wakealarm using rtc1/since_epoch (not system epoch), verifies
read-back, records IRQ counters before/after and clears only its own remaining
alarm. Existing alarm, disabled wakeup, wrong RTC name or non-active state aborts.
No RTC0 writes, no clock synchronization and no wakeup-flag changes.
Four mocked tests cover existing alarms, disabled wakeup, clock-base conversion,
and preserving another alarm during cleanup. Physical test still pending.
This only checks alarm programming/IRQ behavior while awake, not deep sleep wake.
The earlier powerd suspend test remains disabled in the KUAL menu.

## Read-only post-reboot RTC findings (2026-10-20)

- rtc0: bd7181x-rtc, hctosys=1, correct wall time; device/power/wakeup=disabled.
- rtc1: SNVS low-power RTC, hctosys=0, epoch near boot (458 seconds),
  device/power/wakeup=enabled. Do NOT use system epoch timestamps for it without
  correct conversion; its 1970 date does not by itself imply broken hardware.
- Both wakealarm files empty; rtc0 alarm IRQ disabled at observation time.
- Supported suspend states: freeze, standby, mem.
- Root ext3 mounted read-only after reboot (rootfs pseudo-mount rw is normal).
- Current-boot USB logs contain pinctrl/VBUS setup errors and later successful
  mass-storage enumeration. These do not establish cause of previous USB issue.
- Current logs do not show which RTC powerd programmed for the prior request;
  do NOT conclude rtc0 wakeup=disabled is proven cause of the missed wake.

No RTC clock/wakeup flags or USB settings changed. Further suspend testing remains
paused. Stable always-awake/externally-powered scheduling is a potential interim
solution but requires explicit disclosure of battery cost and does not satisfy
battery-efficient deep-sleep scheduling. Production autonomous mode still pending.

## RTC test paused after late wake / USB report

Latest physical log: readyToSuspend event received; rtcWakeup=90 accepted.
Requested deadline epoch 1792476669; script next saw active at 1792476894
(225 seconds late), following a 264-second sampling gap and suspended state.
This does NOT validate automatic wake. wakeUp was also rejected because device
was already active; script now skips wakeUp when active and avoids unverified
power states. User reports USB only charged until reboot. Causation unknown;
do not dismiss or continue uncontrolled suspend tests.

Removed wake-test action from KUAL menu pending investigation. Replaced with
`EatBy: collect wake diagnostics (read only)`: collects RTC identities, supported
power states and recent power/USB logs without changing alarms/services/settings.
Post-reboot diagnostics may not retain the earlier failure. No new wake test
should be run until reviewed. Existing manual refresh/pairing remain available.

## Wake-event payload correction

Second hardware run received `10 ` from the readyToSuspend subscription about
one minute after user pressed power. The script incorrectly required the event
name in stdout and aborted before arming. Stock lipc-wait-event on this device
returns payload only. Updated validation accepts bounded numeric payloads from
the explicitly named subscription, as well as named event output; empty timeout
or unexpected diagnostic output still aborts. Tests now reproduce the observed
`10 ` output, and cover named/empty/invalid/oversized variants. All 12 mocked
safety tests pass. Updated script copied to Kindle; actual wake still unverified.

## Wake-test state correction

First hardware wake test failed before arming: powerd state=active,
`rtcWakeup` returned `0x100 lipcPropErrInvalidState`. No future kernel RTC alarm
was visible and preventScreenSaver=0. This is a rejected request, not evidence
that RTC hardware is unsupported.
Updated script subscribes to readyToSuspend before asking user to press power,
requires the actual event output, then requests a 90-second wake. Subscription
is bounded to 120 seconds; no event means no RTC request. The 90 seconds start
at preparation for suspend, not when the KUAL menu was tapped. Allow about
three minutes after pressing power. If request is rejected after the event,
record it and ask normal powerd wakeUp before showing error.
Ten host-side safety tests pass; actual state-gated wake still needs hardware
verification. No persistent power settings are modified.

## One-shot wake test staged

`EatBy: TEST wake in 90 seconds` requests a one-time powerd rtcWakeup=90,
then asks user to press power once to enter normal sleep. It does not stop
framework/statusbar or change sleep-inhibition properties. It refuses to
replace a visible future kernel RTC alarm or override preventScreenSaver.
At the deadline it requests normal wakeUp and shows a completion message.
No production scheduling or autostart is installed.

Logs: eatby/logs/wake-test.txt. User must report whether wake was automatic.
Timestamp gaps and powerd states are diagnostic, not alone proof of deep sleep.
If still asleep after three minutes, user manually wakes it and reconnects USB;
late observation is recorded as unproven automatic wake. Clock-backward cases
are bounded by a sample limit. Existing RTC/blocked/rejected/deadline paths
covered by four mocked tests. Physical wake behavior remains untested.

## Verified hardware result: overlay suppression

User confirmed no native time/battery overlay throughout the 90-second test,
and normal desktop recovery. Device log independently confirms framework AND
statusbar stayed stop/waiting at 30/60/90 seconds, then both restored to
start/running. Independent watchdog completed and removed the lock.
The tested effective combination is framework + statusbar; do not infer that
stopping statusbar alone is sufficient (not tested).

This is still a bounded experiment, not permanent kiosk mode. Remaining work:
independent user exit, crash recovery for long sessions, screen retention across
sleep, RTC wake and network-ready handling, and the scheduled refresh loop.
Do not simply remove the recovery timeout to enable permanent display.

## Targeted statusbar test

Diagnostic hardware run confirmed framework stayed stop/waiting at all three
samples, while statusbar stayed start/running with unchanged PID 12545.
Pillow/Xorg/awesome/blanket also remained active. This excludes framework
respawn during the sampled interval; statusbar is the next targeted candidate,
not yet a proven fix.

Staged KUAL action: `EatBy: TEST hide system bar (90 sec)`. Temporarily stops
framework and (only if initially running) statusbar. Recovery attempts BOTH
services independently even if one fails; no powerd, network or X changes.
Five mocked tests pass: normal, renderer failure, killed worker, initially
stopped statusbar, and failure to stop statusbar. Physical validation passed.

## Overlay diagnosis follow-up

First physical exclusive test: framework stop/waiting confirmed; worker restored
framework successfully and watchdog completed. User confirms clock/battery still
overlaid DURING the two-minute observation, not only after restoration.
Do not assume framework shutdown is sufficient. Possible independent compositor
or framework restart remains unconfirmed.

Updated KUAL action: `EatBy: diagnose overlay (90 sec)`. Same stopped service
and recovery guard, plus read-only service/process/framebuffer snapshots before
stop and at 30/60/90 seconds. Each diagnostic snapshot has a five-second timeout.
No process command-line arguments or window titles are collected. Diagnostic
version is staged, not yet run. No additional system services are stopped.

## Bounded exclusive-display experiment

Pairing and paired device refresh succeeded on hardware. Available recovery
commands: setsid, timeout, initctl, start, stop. Framework was start/running.
`exclusive-test.sh` is staged with KUAL `EatBy: TEST exclusive display (2 min)`.
It launches a detached worker and an independent detached recovery watchdog,
checks the watchdog is live, temporarily stops framework, and shows the last
cached PNG. Worker restores after a 120-second observation; watchdog attempts
restoration after 150 seconds if worker failed. Start commands have timeouts
and retries. A failed restore retains the lock for diagnosis.
No root files, boot hooks, RTC alarms or powerd properties are changed. This
is NOT yet a validated permanent kiosk mode, nor a suspend-aware scheduler.
Home/KUAL are unavailable while framework is stopped. Do not reconnect USB,
press power or interact with the device during the observation. Wait for the
normal interface to return before reconnecting. If it fails to return after
four minutes, use normal Kindle long-press power restart recovery; no autostart
hook was installed, so this test is not automatically repeated on boot.

Logs: eatby/logs/exclusive-test.txt and exclusive-watchdog.txt.
Host-side mocked tests cover normal completion, image failure, and worker
SIGKILL recovery. They do not establish that watchdog survives real framework
shutdown or prove physical UI restoration. Both require the device test.

## Pairing and status footer (2026-10-20 12:43)

Deployed short-code device pairing and configurable status footer.
KUAL `EatBy: pair device` starts a background, bounded polling worker. Phone:
Settings → Kindle → 使用配对码绑定 Kindle. The eight-character base32 code
expires in ten minutes; only a logged-in household owner can claim it once.
A separate 256-bit device secret is used to poll and becomes the read-only
credential on successful binding. Server persists hashes, not plaintext secrets.
Start/claim/poll endpoints have bounded global rate buckets (10/20/120 per minute).
Failed pairing retains previous device configuration; success does not revoke
old credentials automatically. Remove obsolete devices from owner settings.

Status footer optionally shows image update time and sampled battery percentage;
rotates with content. Existing stored settings default to hidden when the new
field is absent; new default schedules show the footer. No real-time clock.
The native status bar can STILL overwrite the screen: exclusive mode, reliable
exit and suspend-aware scheduling remain unimplemented. Pairing also collects
read-only framework/recovery command availability for that work.

Go integration tests and frontend build/tests pass. Live pairing start and
pending poll verified; physical device pairing awaits user test.
Latest backup: /opt/eatby-backups/kindle-20261020-124300/.
Latest rollback image: eatby:before-kindle-20261020-124300.


## Minimal layout and short links (2026-10-20 12:19)

Deployed four-panel-only PNG layout. Live HTTP 200 and image visually verified.
New display tokens: 16 cryptographically random bytes, 22 URL-safe characters.
New web links: `/d/<token>`; old `/display/<token>` and old tokens remain valid.
Device API paths stay unchanged. No third-party URL shortener is used.
Latest backup: /opt/eatby-backups/kindle-20261020-121925/.
Latest rollback image: eatby:before-kindle-20261020-121925.

Device link configuration (USB-assisted, not an on-screen keyboard UI):
1. Edit the first line of `/mnt/us/eatby/display-link.txt` with a met.xin
   `/d/<token>` or `/display/<token>` link. Never commit this secret file.
2. Safely eject USB; open KUAL → EatBy: import display link.
3. Import validates the link against the config endpoint before atomically
   replacing `/mnt/us/eatby/image-url.txt`; failures keep old configuration.
4. Use EatBy: refresh dashboard. No change needed for the existing paired link.
Server stores token hashes, not recoverable plaintext. Revoke in owner settings.
Kindle USB storage is FAT; do not rely on chmod for protecting its token files.
The system clock may still overwrite the framebuffer. Exclusive display mode,
exit/restore and automatic suspend-aware scheduling are still pending.

## Initial deployment status (2026-10-20)

Image/config/settings implementation deployed to met.xin; app healthy.
NotoSansCJK-Regular.ttc tested on server; SHA256
`b76b0433203017ca80401b2ee0dd69350349871c4b19d504c34dbdd80541690a`.
Verified live authenticated endpoint returns 600×800 PNG with household data.
Kindle image-url.txt paired locally (secret intentionally not included here).
Manual on-device live refresh remains to be tested. Automatic sleep/wake is NOT enabled.

Deployment used locally cross-compiled linux/amd64 binary and verified font
layer on previous runtime image; normal Dockerfile also includes Noto CJK.
Source snapshot synchronized to /opt/eatby (uncommitted, do not git-reset).
Release materials: /opt/eatby-kindle-release/.
SQLite online backup + old source: /opt/eatby-backups/kindle-20261020-112559/.
Rollback image: eatby:before-kindle-20261020-112559.
Rollback: tag that image as eatby:local, then from /opt/eatby run
`docker compose up -d --no-deps --no-build app`.
The new database table is additive; do not restore DB over subsequent user edits.


Target: Kindle 8, portrait 600×800 (6 inches, 167 ppi). Confirm framebuffer
geometry with the device probe before rendering. Observed firmware file:
5.16.2.1.1. Existing FBInk: `/mnt/us/libkh/bin/fbink`.

## Implemented

- `internal/kindle`: validated weekly schedule and next-slot calculation.
- Default: Asia/Shanghai, Monday–Friday 18:00–24:00, Saturday/Sunday
  09:00–24:00, every 30 minutes. End is exclusive; no midnight refresh.
- Embedded timezone database; independent of host timezone.
- Disabled schedule returns no next wake time.
- Tests for boundaries, weekday transitions, UTC input, slot counts and invalid
  input.
- Per-device SQLite schedule storage and owner-only settings UI implemented locally.
  GET/PUT `/api/v1/display-tokens/{id}/schedule`; read-only device endpoint
  GET `/api/v1/display/{token}/config` returns schedule, server time and next slot.
  Revocation and household isolation covered by integration tests. Deployed.
- Read-only device probe and KUAL menu. No power settings changed.
- Probe executed successfully: armv7l, curl 7.86.0, powerd and RTC devices present.
  Jailbreak FBInk is a minimal build with Image=No; preserve it unchanged.
  Root filesystem is currently rw after KUAL installation; not changed by EatBy.
- One-shot `EatBy: screen test (600x800)` uses native eips PNG rendering and
  records actual FBInk geometry/power capabilities. Executed successfully:
  screenWidth=600, screenHeight=800, DPI=167, eips PNG exit status 0.
  User photo confirms correct geometry; residual ghosting needs full refresh.
  Test image is synthetic calibration data, not household data. Reopen Home to
  restore normal UI. Do not infer physical resolution from virtual_size.

## Device probe

Copy `probe.sh` to `/mnt/us/eatby/probe.sh`; copy `config.xml` and `menu.json`
to `/mnt/us/extensions/eatby/`. These are staged on the attached Kindle.

A compatible KUAL installation or existing shell access is required. Copying
an extension does NOT install KUAL or provide command execution by itself.

1. Safely eject Kindle from the computer.
2. If KUAL is available, open it and select `EatBy: device capability probe`.
3. Otherwise install a verified, firmware-compatible KUAL using the appropriate
   jailbreak documentation, or establish USBNetwork/SSH. Do not install random
   firmware/update packages or rerun the jailbreak.
4. With shell access, run `sh /mnt/us/eatby/probe.sh` instead.
5. Reconnect USB and inspect `/Volumes/Kindle/eatby/logs/probe.txt`.

Probe output includes architecture, framebuffer geometry, available commands,
FBInk help, RTC presence and read-only power state. It does not collect serial
numbers, tokens or network addresses; it does not render or schedule alarms.

## Orientation

Settings now support `portrait`, `landscape_cw`, and `landscape_ccw`. Old settings without orientation default to portrait.
Logical page size is 600×800 or 800×600; device PNG size remains 600×800.
`internal/kindle/orientation.go` rotates a fully laid-out page into panel
coordinates. Rotation names describe image rotation, not device rotation.
No framebuffer/system rotation is changed. Tests cover corner mapping,
non-zero source origins, invalid dimensions and API persistence.
The Go dashboard renderer now calls this conversion. Authenticated endpoint:
`GET /api/v1/display/<token>/image.png`. It includes X-Eatby-Next-Refresh and
X-Eatby-Server-Time epoch-second headers (next=0 means paused).

## Current image/client implementation

- `internal/kindle/render.go` uses Go x/image, Chinese fonts, four panels,
  clipped long lines and overflow counts. No Chromium dependency.
- `internal/pantry/kindle_image.go` reuses household dashboard data and checks
  token validity/household active state before rendering.
- `refresh.sh` is staged on Kindle with a KUAL refresh menu. It needs
  `/mnt/us/eatby/image-url.txt` containing the HTTPS image endpoint URL.
  Treat that URL as a secret; do not commit it. Paired on attached Kindle.
- Client checks PNG signature and 600×800 geometry, retains previous cache on
  failure, and requests a flashing GC16 refresh after decoding.
- Integration tests passed using local Arial Unicode. Alpine wqy-zenhei is
  incompatible with the Go sfnt parser (invalid table offset); do not use it.
  Noto CJK verified successfully in production; image endpoint deployed.
- No automatic scheduler/RTC writes yet.

## Remaining implementation

- Test real household data/Chinese rendering/full refresh on Kindle.
- Device-tested RTC/powerd wake integration; plain cron/sleep is insufficient
  during deep sleep. Preserve dashboard against screensaver replacement.
- Pause, exit/restore, offline recovery, restart recovery and status reporting.
- Integration tests, backup and production deployment.

While fully asleep, the device cannot receive changed schedules. Changes apply
on next connection or manual sync. Weekdays currently mean Monday–Friday, not
China's holiday/compensating-workday calendar.

## Rollback of staged files

Remove only `/mnt/us/eatby/probe.sh` and `/mnt/us/extensions/eatby/`.
No autostart hook, firmware file or power setting was installed or modified.
