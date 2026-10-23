"""Network failures must not be confused with revoked credentials or bad TLS."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class RefreshFailureTests(unittest.TestCase):
    def test_error_classification_preserves_cache(self):
        cases = [(6, '000', 69), (7, '000', 69), (28, '000', 69),
                 (22, '503', 69), (22, '429', 69), (22, '408', 69),
                 (22, '401', 1), (22, '403', 1), (22, '404', 1),
                 (60, '000', 1)]
        for code, http, expected in cases:
            with self.subTest(code=code, http=http), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                (root / 'cache').mkdir()
                (root / 'cache/current.png').write_bytes(b'old image')
                (root / 'cache/headers.txt').write_text('old headers')
                (root / 'image-url.txt').write_text('https://example.invalid/test-image.png')
                binary = root / 'bin'
                binary.mkdir()
                for name, body in {'curl': 'printf "%s" "$HTTP"; exit "$CODE"', 'sleep': ':', 'lipc-get-prop': 'echo 50'}.items():
                    tool = binary / name
                    tool.write_text('#!/bin/sh\n' + body + '\n')
                    tool.chmod(0o755)
                source = Path(__file__).with_name('refresh.sh').read_text()
                source = source.replace('BASE=/mnt/us/eatby', f'BASE="{root}"')
                source = source.replace('/tmp/eatby-refresh.lock', str(root / 'lock'))
                source = source.replace('[ -d /mnt/us ] && [ -e /proc/version ] || exit 1', ':')
                script = root / 'refresh.sh'
                script.write_text(source)
                env = dict(os.environ, CODE=str(code), HTTP=http, PATH=str(binary)+':'+os.environ['PATH'], EATBY_LOG_DIR=str(root / 'logs'), EATBY_AGENT_OWNER='')
                result = subprocess.run(['sh', str(script)], env=env, capture_output=True, timeout=5)
                self.assertEqual(result.returncode, expected, result.stderr)
                self.assertEqual((root / 'cache/current.png').read_bytes(), b'old image')
                self.assertEqual((root / 'cache/headers.txt').read_text(), 'old headers')
                self.assertFalse((root / 'lock').exists())
