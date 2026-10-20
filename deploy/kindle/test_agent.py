"""Mocked scheduled worker and recovery tests; never signals host processes."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class AgentTests(unittest.TestCase):
    def run_case(self, mode):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            base = root / 'eatby'
            run = root / 'run'
            log = root / 'logs'
            binary = root / 'bin'
            for p in (base / 'cache', base / 'logs', run, log, binary):
                p.mkdir(parents=True, exist_ok=True)
            (run / 'guard-pid').write_text(str(os.getpid()))
            (run / 'restore-statusbar').touch()
            (root / 'count').write_text('0')
            (root / 'bar').touch()
            tools = {
                'fbink': ':', 'sleep': ':', 'timeout': 'shift; exec "$@"',
                'initctl': '[ -f "$ROOT/bar" ] && echo "statusbar start/running" || echo "statusbar stop/waiting"',
                'stop': 'echo "$1" >> "$ROOT/stops"; rm -f "$ROOT/bar"',
                'start': 'touch "$ROOT/bar"',
            }
            for name, body in tools.items():
                p = binary / name
                p.write_text('#!/bin/sh\n' + body + '\n')
                p.chmod(0o755)
            (base / 'refresh.sh').write_text('''#!/bin/sh
n=$(head -n 1 "$ROOT/count"); n=$((n+1)); echo "$n" > "$ROOT/count"
[ "$MODE" != network_fail ] || exit 1
server=$(date +%s); next=$((server+1800))
[ "$n" -lt 2 ] || next=0
printf 'X-Eatby-Server-Time: %s\r\nX-Eatby-Next-Refresh: %s\r\n' "$server" "$next" > "$ROOT/eatby/cache/headers.txt"
''')
            (base / 'wake-test.sh').write_text('''#!/bin/sh
[ "$EATBY_AGENT" = 1 ] || exit 4
now=$(date +%s)
[ "$EATBY_NEXT_LOCAL" -ge "$((now+1700))" ] || exit 5
echo sleep >> "$ROOT/sleeps"
rm -rf "$ROOT/wake-lock"
''')
            source = Path(__file__).with_name('battery-agent.sh').read_text()
            source = source.replace('BASE=/mnt/us/eatby', f'BASE="{base}"')
            source = source.replace('RUN=/tmp/eatby-agent', f'RUN="{run}"')
            source = source.replace('LOG=/tmp/eatby-agent-logs', f'LOG="{log}"')
            source = source.replace('/tmp/eatby-wake-test', str(root / 'wake-lock'))
            source = source.replace('[ -d /mnt/us ] && [ -e /proc/version ] || exit 1', ':')
            source = source.replace('/mnt/us/libkh/bin/fbink', str(binary / 'fbink'))
            # Never use real /proc identities for mocked guard termination.
            source = source.replace('/proc/', str(root / 'proc') + '/')
            script = root / 'agent.sh'
            script.write_text(source)
            env = dict(os.environ, ROOT=str(root), MODE=mode, PATH=str(binary) + ':' + os.environ['PATH'])
            command = 'guard' if mode == 'guard' else 'worker'
            result = subprocess.run(['sh', str(script), command], env=env, capture_output=True, text=True, timeout=10)
            if mode == 'guard':
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertFalse(run.exists())
                self.assertTrue((root / 'bar').exists())
            else:
                self.assertEqual((root / 'count').read_text().strip(), '2')
                self.assertEqual((root / 'stops').read_text().strip(), 'statusbar')
                self.assertEqual(result.returncode, 1 if mode == 'network_fail' else 0, result.stderr)
                if mode != 'network_fail':
                    self.assertEqual((root / 'sleeps').read_text().splitlines(), ['sleep'])
                    self.assertIn('schedule paused', result.stdout)

    def test_scheduled_sleep_then_pause(self): self.run_case('normal')
    def test_network_failure_is_bounded(self): self.run_case('network_fail')
    def test_guard_restores_after_worker_exit(self): self.run_case('guard')
