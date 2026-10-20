"""Mock-only safety checks for awake RTC1 experiment; no host RTC access."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class RTC1Safety(unittest.TestCase):
    def run_case(self, mode):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            rtc = root / 'rtc1'
            (rtc / 'device/power').mkdir(parents=True)
            (rtc / 'name').write_text('20cc034.snvs-rtc-lp\n')
            (rtc / 'since_epoch').write_text('1000\n')
            (rtc / 'wakealarm').write_text('5000\n' if mode == 'existing' else '')
            (rtc / 'device/power/wakeup').write_text('disabled\n' if mode == 'disabled' else 'enabled\n')
            (root / 'interrupts').write_text('10: 1 snvs_rtc\n')
            (root / 'lock').mkdir()
            (root / 'bin').mkdir()
            tools = {
                'fbink': ':',
                'lipc-get-prop': 'echo active',
                'dmesg': ':',
                'sleep': 'if [ "$1" = 40 ] && [ "$MODE" = foreign ]; then echo 9999 > "$RTC_MOCK/wakealarm"; fi',
            }
            for name, body in tools.items():
                p = root / 'bin' / name
                p.write_text('#!/bin/sh\n' + body + '\n')
                p.chmod(0o755)
            source = Path(__file__).with_name('rtc1-check.sh').read_text()
            source = source.replace('RTC=/sys/class/rtc/rtc1', f'RTC="{rtc}"')
            source = source.replace('LOCK=/tmp/eatby-rtc1-check', f'LOCK="{root / "lock"}"')
            source = source.replace('[ -d /mnt/us ] && [ -e /proc/version ] || exit 1', ':')
            source = source.replace('/mnt/us/libkh/bin/fbink', str(root / 'bin/fbink'))
            source = source.replace('/proc/interrupts', str(root / 'interrupts'))
            script = root / 'check.sh'
            script.write_text(source)
            env = dict(os.environ, MODE=mode, RTC_MOCK=str(rtc), PATH=str(root / 'bin') + ':' + os.environ['PATH'])
            result = subprocess.run(['sh', str(script), 'worker'], env=env, capture_output=True, text=True, timeout=5)
            self.assertFalse((root / 'lock').exists())
            return result, (rtc / 'wakealarm').read_text().strip()

    def test_existing_alarm_not_overwritten(self):
        result, alarm = self.run_case('existing')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(alarm, '5000')

    def test_disabled_wakeup_not_enabled(self):
        result, alarm = self.run_case('disabled')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(alarm, '')

    def test_uses_rtc_epoch_and_cleans_own_alarm(self):
        result, alarm = self.run_case('normal')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('now=1000 target=1030', result.stdout)
        self.assertEqual(alarm, '0')

    def test_does_not_clear_another_alarm(self):
        result, alarm = self.run_case('foreign')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(alarm, '9999')
