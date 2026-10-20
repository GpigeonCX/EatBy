"""Host-side state-machine tests. Uses mocks; never stops host services.
Run: python3 -m unittest discover -s deploy/kindle -p 'test_*.py'
Actual Kindle watchdog survival and UI recovery still require physical testing.
"""
import os
from pathlib import Path
import subprocess
import tempfile
import time
import unittest


class ExclusiveRecovery(unittest.TestCase):
    def run_case(self, mode):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            base = root / 'eatby'
            (base / 'cache').mkdir(parents=True)
            (base / 'cache/current.png').write_bytes(b'mock')
            state = root / 'framework'
            state.touch()
            statusbar = root / 'statusbar'
            if mode != 'statusbar_off':
                statusbar.touch()
            if mode == 'completed_old_lock':
                (root / 'lock').mkdir()
                (root / 'lock/restored').touch()
                (root / 'lock/watchdog.pid').write_text('99999999')
            binary = root / 'bin'
            binary.mkdir()
            scripts = {
                'initctl': '[ -f "$STATE_DIR/$2" ] && echo "$2 start/running, process 1" || echo "$2 stop/waiting"',
                'start': 'touch "$STATE_DIR/$1"',
                'stop': 'if [ "$TEST_MODE" = stop_statusbar_fail ] && [ "$1" = statusbar ]; then exit 1; fi; rm -f "$STATE_DIR/$1"',
                'timeout': 'shift; exec "$@"',
                'sleep': 'case "$1" in 150|600) /bin/sleep 3;; 120) /bin/sleep 0.05;; *) /bin/sleep 0.01;; esac',
                'setsid': 'exec "$@"',  # process survival itself is a hardware test
                'eips': 'if [ -f "$STATE_DIR/framework" ] || [ -f "$STATE_DIR/statusbar" ]; then exit 2; fi; case "$TEST_MODE" in fail) exit 1;; crash) kill -KILL "$PPID"; exit 1;; esac',
                'fbink': 'exit 0',
            }
            for name, body in scripts.items():
                p = binary / name
                p.write_text('#!/bin/sh\n' + body + '\n')
                p.chmod(0o755)
            source = Path(__file__).with_name('exclusive-test.sh').read_text()
            source = source.replace('/tmp/eatby-wake-test', str(root / 'wake-lock'))
            source = source.replace('RAMLOG=/tmp/eatby-cycle-logs', f'RAMLOG="{root / "ramlogs"}"')
            (base / 'wake-test.sh').write_text('#!/bin/sh\n[ -f "$STATE_DIR/framework" ] || exit 8\nrmdir "$STATE_DIR/wake-lock"\n[ "$TEST_MODE" != cycle_fail ] || exit 1\nsh "$(dirname "$0")/exclusive-test.sh" take-display || exit 9\n[ -f "$STATE_DIR/framework" ] && [ ! -f "$STATE_DIR/statusbar" ]\n')
            source = source.replace('BASE=/mnt/us/eatby', f'BASE="{base}"')
            source = source.replace('LOCK=/tmp/eatby-exclusive-test', f'LOCK="{root / "lock"}"')
            source = source.replace('[ -d /mnt/us ] && [ -e /proc/version ] || exit 1', ': # mocked device check')
            source = source.replace('/mnt/us/libkh/bin/fbink', str(binary / 'fbink'))
            script = base / 'exclusive-test.sh'
            script.write_text(source)
            env = dict(os.environ, PATH=str(binary) + ':' + os.environ['PATH'], STATE_DIR=str(root), TEST_MODE=mode, EATBY_FULL_CYCLE='1' if mode.startswith('cycle_') else '0')
            subprocess.run(['sh', str(script)], env=env, check=True, timeout=5)
            deadline = time.monotonic() + 10
            while (root / 'lock').exists() and time.monotonic() < deadline:
                time.sleep(0.05)
            self.assertFalse((root / 'lock').exists(), 'watchdog did not finish')
            self.assertTrue(state.exists(), 'framework was not restored')
            self.assertEqual(statusbar.exists(), mode != 'statusbar_off', 'statusbar original state not preserved')
            log = (base / 'logs/exclusive-test.txt').read_text()
            if mode.startswith('cycle_'):
                self.assertIn('Framework remains running throughout', log)
            else:
                self.assertIn('Stopping framework temporarily', log)
            return log

    def test_integrated_cycle_success_restores_services(self):
        self.assertIn('Integrated cycle returned: 0', self.run_case('cycle_ok'))

    def test_integrated_cycle_failure_restores_services(self):
        self.assertIn('Integrated cycle returned: 1', self.run_case('cycle_fail'))

    def test_completed_old_lock_allows_retry(self):
        self.assertIn('Test complete', self.run_case('completed_old_lock'))

    def test_normal_exit_restores(self):
        self.assertIn('Test complete', self.run_case('normal'))

    def test_render_failure_restores(self):
        self.assertIn('Render failed', self.run_case('fail'))

    def test_killed_worker_watchdog_restores(self):
        self.run_case('crash')

    def test_initially_stopped_statusbar_stays_stopped(self):
        self.assertIn('Test complete', self.run_case('statusbar_off'))

    def test_statusbar_stop_failure_restores_framework(self):
        self.assertIn('Statusbar stop failed', self.run_case('stop_statusbar_fail'))
