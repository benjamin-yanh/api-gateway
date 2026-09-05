"""Exercise activation and rollback using temporary hosts and fake services."""
import hashlib
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('activate.sh').resolve()
RELEASE = '1-1-' + 'a' * 40


class ActivationTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.release = self.root / 'releases' / RELEASE
        self.release.mkdir(parents=True)
        (self.root / 'bin').mkdir()
        (self.root / 'web').mkdir()
        (self.root / 'web/index.html').write_text('old web')
        (self.root / 'bin/new-api-control').write_text('old binary')
        self.mocks = self.root / 'mocks'
        self.mocks.mkdir()
        for name, body in {
            'flock': 'exit 0',
            'nginx': 'exit 0',
            'systemctl': 'test "${FAIL_SERVICE:-0}" != 1',
            'curl': '''
if [ "${FAIL_HTTP:-0}" = 1 ]; then exit 1; fi
while [ "$#" -gt 0 ]; do
  if [ "$1" = -o ]; then cp "$DEPLOY_ROOT/web/index.html" "$2"; exit 0; fi
  shift
done
''',
        }.items():
            path = self.mocks / name
            path.write_text('#!/bin/sh\n' + body + '\n')
            path.chmod(0o755)
        self.env = dict(os.environ, DEPLOY_ROOT=str(self.root),
                        PUBLIC_URL='https://example.com',
                        PATH=str(self.mocks) + ':' + os.environ['PATH'])

    def artifact(self, name, content):
        path = self.release / name
        path.write_bytes(content)
        digest = hashlib.sha256(content).hexdigest()
        (self.release / 'SHA256SUMS').write_text(f'{digest}  {name}\n')

    def run_phase(self, role, success=True, **env):
        result = subprocess.run(['bash', str(SCRIPT), role, RELEASE],
                                env=dict(self.env, **env), capture_output=True, text=True)
        self.assertEqual(result.returncode == 0, success, result.stdout + result.stderr)

    def web_artifact(self):
        new = self.root / 'index.html'
        new.write_text('new web')
        archive = self.root / 'frontend.tar.gz'
        with tarfile.open(archive, 'w:gz') as tar:
            tar.add(new, arcname='index.html')
        self.artifact('frontend.tar.gz', archive.read_bytes())

    def test_frontend_activation_preserves_backup(self):
        self.web_artifact()
        self.run_phase('frontend')
        self.assertEqual((self.root / 'web/index.html').read_text(), 'new web')
        self.assertEqual((self.release / 'backups/frontend/index.html').read_text(), 'old web')
        self.assertEqual((self.root / 'web').stat().st_mode & 0o777, 0o755)

    def test_frontend_http_failure_restores_old_site(self):
        self.web_artifact()
        self.run_phase('frontend', success=False, FAIL_HTTP='1')
        self.assertEqual((self.root / 'web/index.html').read_text(), 'old web')

    def test_checksum_failure_leaves_live_binary_untouched(self):
        self.artifact('new-api-control', b'new binary')
        (self.release / 'new-api-control').write_bytes(b'corrupted')
        self.run_phase('control', success=False)
        self.assertEqual((self.root / 'bin/new-api-control').read_text(), 'old binary')

    def test_service_failure_restores_old_binary(self):
        self.artifact('new-api-control', b'new binary')
        self.run_phase('control', success=False, FAIL_SERVICE='1')
        self.assertEqual((self.root / 'bin/new-api-control').read_text(), 'old binary')

    def test_successful_control_activation_keeps_frontend_unchanged(self):
        self.artifact('new-api-control', b'new binary')
        self.run_phase('control')
        self.assertEqual((self.root / 'bin/new-api-control').read_text(), 'new binary')
        self.assertEqual((self.root / 'web/index.html').read_text(), 'old web')


if __name__ == '__main__':
    unittest.main()
