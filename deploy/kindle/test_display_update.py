"""Local installer tests using temporary files and mocked device operations."""
import hashlib
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class DisplayUpdateTests(unittest.TestCase):
    def run_case(self, mode):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            base = root / 'eatby'
            stage = base / 'display-update'
            stage.mkdir(parents=True)
            (root / 'lock').mkdir()
            binary = root / 'bin'
            binary.mkdir()
            for name, body in {'fbink': ':', 'sleep': 'if [ "$MODE" = running ] && [ "$1" = 2 ]; then rm -rf "$ROOT/agent"; fi'}.items():
                p = binary / name
                p.write_text('#!/bin/sh\n' + body + '\n')
                p.chmod(0o755)
            manifest = []
            files = ('wake-test.sh', 'refresh.sh', 'battery-agent.sh')
            for name in files:
                (base / name).write_text('#!/bin/sh\n# old\n')
                data = b'#!/bin/sh\n# new\n'
                if name == 'battery-agent.sh': data += b'touch "$ROOT/restarted"\n'
                (stage / name).write_bytes(data)
                manifest.append(hashlib.sha256(data).hexdigest() + '  ' + name)
            (stage / 'SHA256SUMS').write_text('\n'.join(manifest) + '\n')
            if mode == 'corrupt':
                (stage / 'wake-test.sh').write_text('# changed after checksum\n')
            if mode in ('running', 'stuck'):
                (root / 'agent').mkdir()
            (base / 'image-url.txt').write_text('private-placeholder')
            source = Path(__file__).with_name('apply-display-update.sh').read_text()
            source = source.replace('BASE=/mnt/us/eatby', f'BASE="{base}"')
            source = source.replace('LOCK=/tmp/eatby-display-update', f'LOCK="{root / "lock"}"')
            source = source.replace('[ -d /mnt/us ] && [ -e /proc/version ] || exit 1', ':')
            source = source.replace('/mnt/us/libkh/bin/fbink', str(binary / 'fbink'))
            for name in ('agent', 'exclusive-test', 'wake-test', 'refresh.lock'):
                source = source.replace('/tmp/eatby-' + name, str(root / name))
            script = root / 'apply.sh'
            script.write_text(source)
            result = subprocess.run(['sh', str(script), 'worker'], env=dict(os.environ, ROOT=str(root), MODE=mode, PATH=str(binary)+':'+os.environ['PATH']), capture_output=True, text=True, timeout=5)
            success = mode in ('stopped', 'running')
            self.assertEqual(result.returncode == 0, success, result.stderr)
            self.assertEqual((base / 'image-url.txt').read_text(), 'private-placeholder')
            self.assertEqual((root / 'restarted').exists(), mode == 'running')
            self.assertFalse((root / 'lock').exists())
            for name in files:
                self.assertIn('# new' if success else '# old', (base / name).read_text())
                if success:
                    self.assertIn('# old', (base / 'backup-network-v2' / name).read_text())

    def test_stopped_agent_stays_stopped(self): self.run_case('stopped')
    def test_running_agent_stops_before_update_and_restarts(self): self.run_case('running')
    def test_checksum_failure_preserves_files(self): self.run_case('corrupt')
    def test_stop_timeout_preserves_files(self): self.run_case('stuck')
