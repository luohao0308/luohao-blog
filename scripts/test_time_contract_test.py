"""The reproduction runner must refuse remote Docker targets before fixture writes."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / 'scripts/test_time_contract.py'
HELPER = ROOT / 'backend/cmd/.time-baseline-check'


class LocalDockerGateTests(unittest.TestCase):
    def run_refused(self, env, reason):
        self.assertFalse(HELPER.exists(), 'run separately from the actual fixture')
        result = subprocess.run(['python3', str(SCRIPT)], cwd=ROOT, env=env, capture_output=True, text=True, timeout=20)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(reason, result.stderr)
        self.assertFalse(HELPER.exists())

    def test_remote_host_overrides_are_refused(self):
        for host in ['ssh://fixture-refused.invalid', 'tcp://192.0.2.1:2376']:
            with self.subTest(host=host):
                self.run_refused(os.environ | {'DOCKER_HOST': host}, 'refusing a non-local Docker host')

    def test_selected_remote_context_is_refused(self):
        with tempfile.TemporaryDirectory(prefix='case-docker-gate-') as tmp:
            stub = Path(tmp) / 'docker'
            stub.write_text('#!/usr/bin/env python3\nimport sys,json\nif sys.argv[1:]==["context","show"]:print("remote-fixture");sys.exit(0)\nif sys.argv[1:3]==["context","inspect"]:print(json.dumps("ssh://fixture-refused.invalid"));sys.exit(0)\nraise SystemExit(90)\n')
            stub.chmod(0o755)
            env = {k: v for k, v in os.environ.items() if k != 'DOCKER_HOST'}
            env['PATH'] = tmp + os.pathsep + env['PATH']
            self.run_refused(env, 'refusing a non-local Docker context')


if __name__ == '__main__':
    unittest.main()
