"""Restore orchestration tests; all Docker/CLI operations are synthetic."""
import gzip
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('dump_fixture', ROOT / 'scripts/test_time_backup_manifest.py')
f = importlib.util.module_from_spec(spec)
spec.loader.exec_module(f)

STUB = r'''#!/usr/bin/env python3
import json,os,pathlib,sys
args=sys.argv[1:];root=pathlib.Path(os.environ['RESTORE_STUB_ROOT'])
with (root/'calls').open('a') as out:out.write(json.dumps(args)+'\n')
if args[0]=='inspect' and 'running' in args[2]:
 print(json.dumps({'id':'owned-backend','running':os.environ['RESTORE_STUB_MODE']=='still-running','status':'running' if os.environ['RESTORE_STUB_MODE']=='still-running' else 'exited'}));sys.exit(0)
if args[0]=='inspect':
 print(json.dumps({'id':'owned-backend','image':'sha256:'+'a'*64,'files':'/owned/base.yml,/owned/release.yml','project':'owned-prod','directory':'/owned/deploy','mounts':[{'Destination':'/app/data/time-contract','Type':'volume','Name':'owned-control'}]}));sys.exit(0)
if args[:2]==['image','inspect']:
 if os.environ['RESTORE_STUB_MODE']=='image-missing':sys.exit(1)
 print('sha256:'+'a'*64);sys.exit(0)
if 'run' in args and '--help' in args:
 print('Usage: compose run\n      --pull string policy' if os.environ['RESTORE_STUB_MODE']!='unsupported-help' else 'Usage: compose run');sys.exit(0)
if 'config' in args and '-q' in args:sys.exit(0)
if 'run' in args:assert '--no-build' not in args and '--build' not in args
mode=args[args.index('-mode')+1] if '-mode' in args else ''
if mode=='restore-begin':
 (root/'pending').write_text('pending');print(json.dumps({'attempt_nonce':'b'*64}));sys.exit(0)
if mode=='snapshot':print(json.dumps({'target':{'server_uuid':'fixture','database':'blog'},'snapshot':{},'snapshot_sha256':'f'*64}));sys.exit(0)
if mode=='restore-complete':
 mount=next(a for a in args if a.endswith(':/time-operation:rw'));receipt=json.loads((pathlib.Path(mount.removesuffix(':/time-operation:rw'))/'verification.json').read_text());assert receipt['attempt_nonce']=='b'*64
 print(json.dumps({'ready':os.environ['RESTORE_STUB_MODE']=='success'}));sys.exit(0)
if 'stop' in args:sys.exit(0)
if 'start' in args:sys.exit(0)
if 'ps' in args:print('blog-backend healthy');sys.exit(0)
if any('mysqldump ' in x for x in args):sys.stdout.buffer.write((root/'sql').read_bytes());sys.exit(0)
if any('exec mysql ' in x for x in args):
 sys.stdin.buffer.read();sys.exit(7 if os.environ['RESTORE_STUB_MODE']=='import-fail' else 0)
raise SystemExit(90)
'''


class RestoreShellTests(unittest.TestCase):
    def exercise(self, mode):
        with tempfile.TemporaryDirectory(prefix='utc-restore-stub-') as directory:
            root = Path(directory)
            docker = root / 'docker'
            docker.write_text(STUB)
            docker.chmod(0o755)
            sql = f.dump(b'CREATE TABLE `time_contract` (\n `id` int\n) ENGINE=InnoDB;\nINSERT INTO `time_contract` (`id`) VALUES (1);\n', version=21)
            (root / 'sql').write_bytes(sql)
            backup = root / 'backup.gz'
            backup.write_bytes(gzip.compress(sql))
            manifest = Path(str(backup) + '.manifest.json')
            manifest.write_text(json.dumps(f.m.inspect_dump(backup)))
            env = os.environ | {'PATH': str(root) + os.pathsep + os.environ['PATH'], 'RESTORE_STUB_ROOT': str(root), 'RESTORE_STUB_MODE': mode, 'BLOG_RESTORE_WRITERS_PAUSED': '1', 'BLOG_RESTORE_EVIDENCE_DIR': str(root / 'evidence')}
            result = subprocess.run(['bash', str(ROOT / 'deploy/restore.sh'), str(backup)], env=env, capture_output=True, timeout=20)
            calls = [json.loads(line) for line in (root / 'calls').read_text().splitlines()]
            if mode in ('unsupported-help', 'image-missing'):
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse((root / 'pending').exists())
                self.assertFalse(any('stop' in call or '-mode' in call for call in calls))
                return
            self.assertTrue((root / 'pending').exists())
            starts = [call for call in calls if 'start' in call]
            if mode == 'success':
                self.assertEqual(result.returncode, 0, result.stderr.decode())
                self.assertEqual(len(starts), 1)
            else:
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(starts, [])
            operator = next(call for call in calls if 'restore-begin' in call)
            self.assertIn('owned-control:/app/data/time-contract:rw', operator)
            self.assertIn('/owned/release.yml', operator)
            self.assertIn('--pull', operator)
            self.assertNotIn('--no-build', operator)
            self.assertNotIn('--build', operator)
            self.assertNotIn('-writers-paused', operator)
            evidence = next((root / 'evidence').glob('restore.*'))
            self.assertIn('sha256:' + 'a'*64, (evidence / 'compose.operator.yml').read_text())
            if mode == 'still-running':
                self.assertFalse(any(any('exec mysql ' in arg for arg in call) for call in calls))

    def test_unsupported_compose_run_refused_before_pending(self):
        self.exercise('unsupported-help')

    def test_missing_cached_image_refused_before_pending(self):
        self.exercise('image-missing')

    def test_failed_import_never_restarts(self):
        self.exercise('import-fail')

    def test_stop_must_be_observed_before_import(self):
        self.exercise('still-running')

    def test_incomplete_receipt_never_restarts(self):
        self.exercise('not-ready')

    def test_verified_ready_starts_only_after_completion(self):
        self.exercise('success')


if __name__ == '__main__':
    unittest.main()
