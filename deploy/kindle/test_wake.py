"""Mock-only tests for one-shot wake experiment safety checks."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class WakeSafety(unittest.TestCase):
    def run_case(self, mode):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'bin').mkdir()
            (root / 'eatby').mkdir()
            (root / 'lock').mkdir()
            (root / 'agent').mkdir()
            if mode.startswith('rtc1_agent'):
                (root / 'lock/agent-owned').touch()
                (root / 'eatby/cache').mkdir()
                if mode != 'rtc1_agent_nocache':
                    (root / 'eatby/cache/current.png').write_bytes(b'mock image')
            (root / 'rtc/rtc0').mkdir(parents=True)
            (root / 'rtc/rtc0/wakealarm').write_text('99999' if mode == 'alarm' else '')
            rtc1 = root / 'rtc/rtc1'
            (rtc1 / 'device/power').mkdir(parents=True)
            (rtc1 / 'wakealarm').write_text('99999' if mode == 'rtc1_existing' else '')
            (rtc1 / 'name').write_text('20cc034.snvs-rtc-lp')
            (rtc1 / 'device/power/wakeup').write_text('disabled' if mode == 'rtc1_disabled' else 'enabled')
            (rtc1 / 'since_epoch').write_text('500')
            (root / 'interrupts').write_text('51: 0 rtc alarm\n')
            (root / 'clock').write_text('1000')
            (root / 'uptime').write_text('2000.00 0.00\n')
            tools = {
                'sleep': ':',
                'timeout': 'shift; exec "$@"',
                'lipc-wait-event': 'echo event-listener >> "$ROOT/writes"; case "$MODE" in noevent) exit 0;; named) echo "readyToSuspend 1";; invalid) echo "publisher unavailable";; oversized) echo "1234567890";; *) printf "10 \\n";; esac',
                'date': 'if [ "$1" = +%s ]; then n=$(head -n 1 "$ROOT/clock"); n=$((n+10)); echo "$n" > "$ROOT/clock"; echo "$n"; else echo MOCK_DATE; fi',
                'fbink': ':',
                'eips': 'echo cached-paint >> "$ROOT/draws"; [ "$MODE" != rtc1_agent_drawfail ]',
                'dmesg': ':',
                'lipc-get-prop': 'echo "$2" >> "$ROOT/get_calls"; case "$2" in preventScreenSaver) if [ "$MODE" = blocked ]; then echo 1; else echo 0; fi;; *) echo active;; esac',
                'lipc-set-prop': 'echo "$*" >> "$ROOT/writes"; if [ "$MODE" = rejected ] && [ "$3" = rtcWakeup ]; then exit 1; fi',
            }
            for name, body in tools.items():
                p = root / 'bin' / name
                p.write_text('#!/bin/sh\n' + body + '\n')
                p.chmod(0o755)
            source = Path(__file__).with_name('wake-test.sh').read_text()
            source = source.replace('BASE=/mnt/us/eatby', f'BASE="{root / "eatby"}"')
            source = source.replace('LOCK=/tmp/eatby-wake-test', f'LOCK="{root / "lock"}"')
            source = source.replace('[ -d /mnt/us ] && [ -e /proc/version ] || exit 1', ':')
            source = source.replace('/sys/class/rtc', str(root / 'rtc'))
            source = source.replace('/proc/interrupts', str(root / 'interrupts'))
            source = source.replace('/tmp/eatby-agent/rtc-target', str(root / 'agent/rtc-target'))
            source = source.replace('/proc/uptime', str(root / 'uptime'))
            source = source.replace('/proc/timer_list', str(root / 'timer_list'))
            source = source.replace('/mnt/us/libkh/bin/fbink', str(root / 'bin/fbink'))
            script = root / 'wake-test.sh'
            script.write_text(source)
            env = dict(os.environ, ROOT=str(root), MODE=mode, EATBY_AGENT='1' if mode.startswith('rtc1_agent') else '0', EATBY_NEXT_LOCAL='1210', EATBY_RTC1_TEST='1' if mode.startswith('rtc1_') else '0', PATH=str(root / 'bin') + ':' + os.environ['PATH'])
            result = subprocess.run(['sh', str(script), 'worker'], env=env, capture_output=True, text=True, timeout=5)
            writes = (root / 'writes').read_text() if (root / 'writes').exists() else ''
            self.assertFalse((root / 'lock').exists())
            # No powerd IPC in the wait loop, regardless of number of samples.
            self.assertLessEqual(len((root / 'get_calls').read_text().splitlines()), 8 if mode.startswith('rtc1_agent') else 7)
            if mode.startswith('rtc1_agent'):
                draws = (root / 'draws').read_text().splitlines() if (root / 'draws').exists() else []
                self.assertEqual(len(draws), 0 if mode == 'rtc1_agent_nocache' else 2)
            if mode == 'rtc1_normal':
                self.assertEqual((rtc1 / 'wakealarm').read_text().strip(), '0')
            elif mode == 'rtc1_existing':
                self.assertEqual((rtc1 / 'wakealarm').read_text().strip(), '99999')
            return result, writes

    def test_existing_alarm_is_not_overwritten(self):
        result, writes = self.run_case('alarm')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(writes, '')

    def test_sleep_inhibition_is_not_overridden(self):
        result, writes = self.run_case('blocked')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(writes, '')

    def test_rejected_alarm_aborts(self):
        result, writes = self.run_case('rejected')
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('Requested wake in 90 seconds', result.stdout)
        self.assertTrue(writes.startswith('event-listener\n'))

    def test_successful_event_timeout_does_not_arm_alarm(self):
        result, writes = self.run_case('noevent')
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('rtcWakeup', writes)

    def test_cached_paint_failures_do_not_break_wake_cycle(self):
        for mode in ('rtc1_agent_nocache', 'rtc1_agent_drawfail'):
            with self.subTest(mode=mode):
                result, writes = self.run_case(mode)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(writes.count('powerButton 1'), 1)

    def test_agent_requests_sleep_and_uses_schedule_deadline(self):
        result, writes = self.run_case('rtc1_agent')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(writes.count('powerButton 1'), 1)
        self.assertNotIn('rtcWakeup', writes)
        self.assertNotIn('Requested wake in 90 seconds', result.stdout)

    def test_rtc1_uses_own_time_and_skips_powerd_alarm(self):
        result, writes = self.run_case('rtc1_normal')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('epoch=500 target=590', result.stdout)
        self.assertNotIn('rtcWakeup', writes)

    def test_rtc1_refuses_existing_alarm_or_disabled_wakeup(self):
        for mode in ('rtc1_existing', 'rtc1_disabled'):
            with self.subTest(mode=mode):
                result, writes = self.run_case(mode)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn('rtcWakeup', writes)

    def test_named_event_is_also_supported(self):
        result, writes = self.run_case('named')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('rtcWakeup 90', writes)

    def test_unexpected_event_output_never_arms_alarm(self):
        for mode in ('invalid', 'oversized'):
            with self.subTest(mode=mode):
                result, writes = self.run_case(mode)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn('rtcWakeup', writes)

    def test_deadline_path_does_not_claim_proof_of_sleep(self):
        result, writes = self.run_case('normal')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('rtcWakeup 90', writes)
        self.assertTrue(writes.startswith('event-listener\n'))
        self.assertIn('Need user observation', result.stdout)
        self.assertNotIn('wakeUp', writes)
        self.assertIn('Already active; not issuing wakeUp.', result.stdout)
