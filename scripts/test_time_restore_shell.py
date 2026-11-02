"""Restore orchestration tests; all Docker/CLI operations are synthetic."""
import gzip
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import io
import tarfile
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
 print(json.dumps({'id':'owned-backend','image':'sha256:'+'a'*64,'files':'/owned/base.yml,/owned/release.yml','project':'owned-prod','directory':'/owned/deploy','mounts':[{'Destination':'/app/data/time-contract','Type':'bind','Source':'/owned/control','RW':os.environ['RESTORE_STUB_MODE']=='runtime-state-rw'} if os.environ.get('RESTORE_STUB_STORAGE')=='bind' else {'Destination':'/app/data/time-contract','Type':'volume','Name':'owned-control','RW':os.environ['RESTORE_STUB_MODE']=='runtime-state-rw'},{'Destination':'/app/data/uploads','Type':'bind','Source':'/owned/uploads'} if os.environ.get('RESTORE_STUB_STORAGE')=='bind' else {'Destination':'/app/data/uploads','Type':'volume','Name':'owned-uploads'},{'Destination':'/data/conf','Type':'bind','Source':'/owned/conf','RW':False},{'Destination':'/owned/cache','Type':'volume','Name':'owned-cache','RW':True}]}));sys.exit(0)
if args[:2]==['image','inspect']:
 if os.environ['RESTORE_STUB_MODE']=='image-missing':sys.exit(1)
 print('sha256:'+'a'*64);sys.exit(0)
if 'run' in args and '--help' in args:
 print('Usage: compose run\n      --pull string policy' if os.environ['RESTORE_STUB_MODE']!='unsupported-help' else 'Usage: compose run');sys.exit(0)
if 'config' in args and '-q' in args:sys.exit(0)
if 'run' in args:assert '--no-build' not in args and '--build' not in args
if 'run' in args and any('/proc/self/mountinfo' in x for x in args):
 uploads=any(x.endswith('compose.uploads.yml') for x in args)
 print('/ ro,relatime\n/data/conf ro,relatime\n/owned/cache ro,relatime\n/app/data/time-contract '+('ro' if uploads or os.environ['RESTORE_STUB_MODE']=='wrong-mount' else 'rw')+',relatime\n/app/data/uploads '+('rw' if uploads else 'ro')+',relatime\n/time-operation '+('ro' if uploads else 'rw')+',relatime');sys.exit(0)
if 'run' in args and 'tar' in args:sys.stdin.buffer.read();sys.exit(0)
mode=args[args.index('-mode')+1] if '-mode' in args else ''
if mode=='restore-begin':
 (root/'pending').write_text('pending');print(json.dumps({'attempt_nonce':'b'*64}));sys.exit(0)
if mode=='snapshot':print(json.dumps({'target':{'server_uuid':'fixture','database':'blog'},'snapshot':{},'snapshot_sha256':'f'*64}));sys.exit(0)
if mode=='restore-complete':
 overlay=next(a for a in args if a.endswith('compose.operator.yml'));receipt=json.loads((pathlib.Path(overlay).parent/'verification.json').read_text());assert receipt['attempt_nonce']=='b'*64
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
    def exercise(self, mode, storage="volume", with_uploads=False):
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
            env = os.environ | {'PATH': str(root) + os.pathsep + os.environ['PATH'], 'RESTORE_STUB_ROOT': str(root), 'RESTORE_STUB_MODE': mode, 'RESTORE_STUB_STORAGE': storage, 'BLOG_RESTORE_WRITERS_PAUSED': '1', 'BLOG_RESTORE_EVIDENCE_DIR': str(root / 'evidence')}
            args=['bash', str(ROOT / 'deploy/restore.sh'), str(backup)]
            if with_uploads:
                archive=root/'uploads.tar.gz'
                with tarfile.open(archive,'w:gz') as tar:
                    info=tarfile.TarInfo('uploads/fixture.txt');info.size=7;tar.addfile(info,io.BytesIO(b'fixture'))
                args.append(str(archive))
            result = subprocess.run(args, env=env, capture_output=True, timeout=20)
            calls = [json.loads(line) for line in (root / 'calls').read_text().splitlines()]
            if mode in ('unsupported-help', 'image-missing','wrong-mount','runtime-state-rw'):
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
            self.assertNotIn('-v',operator)
            self.assertIn('/owned/release.yml', operator)
            self.assertIn('--pull', operator)
            self.assertNotIn('--no-build', operator)
            self.assertNotIn('--build', operator)
            self.assertNotIn('-writers-paused', operator)
            evidence = next((root / 'evidence').glob('restore.*')).resolve()
            overlay=json.loads((evidence/'compose.operator.yml').read_text());service=overlay['services']['backend'];mounts={v['target']:v for v in service['volumes']}
            self.assertEqual(service['image'],'sha256:'+'a'*64);self.assertTrue(service['read_only'])
            self.assertFalse(mounts['/app/data/time-contract']['read_only']);self.assertTrue(mounts['/app/data/uploads']['read_only']);self.assertFalse(mounts['/time-operation']['read_only'])
            self.assertEqual(mounts['/app/data/time-contract']['type'],storage)
            self.assertTrue(mounts['/data/conf']['read_only']);self.assertTrue(mounts['/owned/cache']['read_only'])
            self.assertTrue(any('/proc/self/mountinfo' in arg for call in calls for arg in call))
            for call in calls:
                if 'start' in call or 'stop' in call:self.assertNotIn(str(evidence/'compose.operator.yml'),call)
            if with_uploads:
                call=next(call for call in calls if 'tar' in call);self.assertIn(str(evidence/'compose.uploads.yml'),call)
                service=json.loads((evidence/'compose.uploads.yml').read_text())['services']['backend'];mounts={v['target']:v for v in service['volumes']}
                self.assertTrue(service['read_only']);self.assertTrue(mounts['/app/data/time-contract']['read_only']);self.assertFalse(mounts['/app/data/uploads']['read_only']);self.assertTrue(mounts['/time-operation']['read_only'])
            if mode == 'still-running':
                self.assertFalse(any(any('exec mysql ' in arg for arg in call) for call in calls))

    def test_writable_runtime_guard_refused_before_pending(self):
        self.exercise('runtime-state-rw')

    def test_actual_mount_mismatch_refused_before_pending(self):
        self.exercise('wrong-mount')

    def test_bind_mounts_keep_same_boundaries(self):
        self.exercise('success',storage='bind')

    def test_upload_restore_has_separate_scoped_overlay(self):
        self.exercise('success',with_uploads=True)

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
