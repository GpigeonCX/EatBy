#!/bin/sh
# Explicit opt-in hardware experiment: arm SNVS RTC1 in its own time base
# after readyToSuspend, then let powerd perform normal suspend.
# No changes to RTC clocks, wakeup-enable flags, USB, or startup configuration.
export EATBY_RTC1_TEST=1
exec sh /mnt/us/eatby/wake-test.sh "$@"
