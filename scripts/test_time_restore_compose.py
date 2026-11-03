#!/usr/bin/env python3
"""Real Compose/CLI/MySQL restore acceptance on owned local synthetic resources."""
import argparse
import gzip
import importlib.util
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import tarfile
import tempfile
import time
import uuid

from test_time_contract import local_docker_command
from time_restore_mysql_acceptance import DUMP_FLAGS

ROOT = Path(__file__).resolve().parents[1]
LABEL = 'utc-restore-compose'


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--bind-state', action='store_true', help='also exercise host bind control storage')
    bind_state = parser.parse_args().bind_state
    docker = local_docker_command()
    env = {k: v for k, v in os.environ.items() if not k.startswith('KRATOS') and 'DSN' not in k and k not in {'GOROOT', 'DATABASE_SOURCE'}}
    env['DOCKER_CONTEXT'] = docker[2]
    out = ROOT / 'output/time-contract/compose-restore' / ('bind' if bind_state else 'volume')
    out.mkdir(parents=True, exist_ok=True)
    (out / 'summary.json').write_text('{"passed":false,"status":"running"}\n')

    def run(args, data=None, *, cwd=None, check=True, timeout=180):
        r = subprocess.run(args, input=data, capture_output=True, env=env, cwd=cwd, timeout=timeout)
        if check and r.returncode:
            (out / 'failure.private.log').write_bytes(r.stdout + r.stderr)
            (out / 'failure.private.log').chmod(0o600)
            raise RuntimeError('owned Compose acceptance failed; private diagnostics withheld')
        return r.stdout if check else r

    help_text = run(docker + ['compose', 'run', '--help']).decode()
    assert '--pull' in help_text, 'Compose run must support --pull; no fixtures created'
    compose_version = run(docker + ['compose', 'version', '--short']).decode().strip()
    project = 'utc-restore-' + uuid.uuid4().hex[:12]
    image = project + ':fixture'
    backend_name = project + '-backend'
    assert run(docker + ['inspect', backend_name], check=False).returncode != 0
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    spec = importlib.util.spec_from_file_location('owned_manifest', ROOT / 'deploy/time_backup_manifest.py')
    manifest = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(manifest)
    with tempfile.TemporaryDirectory(prefix='utc-compose-') as directory:
        root = Path(directory).resolve()
        deploy = root / 'deploy'
        deploy.mkdir()
        for name in ['restore.sh', 'time_backup_manifest.py']:
            shutil.copyfile(ROOT / 'deploy' / name, deploy / name)
        # Only the fixed container identifier changes in this test copy; commands and lifecycle remain real.
        restore = deploy / 'restore.sh'
        original = restore.read_text()
        assert 'blog-backend' in original
        restore.write_text(original.replace('blog-backend', backend_name))
        (deploy / '.env.prod').write_text('')
        conf = deploy / 'conf'
        conf.mkdir()
        (conf / 'fixture.yaml').write_text('data:\n  database:\n    source: "root@tcp(mysql:3306)/blog?parseTime=true&multiStatements=true"\n')
        state, uploads = root / 'state', root / 'uploads'
        state.mkdir(mode=0o700)
        uploads.mkdir()
        (uploads / 'sentinel.txt').write_text('backed-up synthetic upload\n')
        arch = run(docker + ['info', '--format', '{{.Architecture}}']).decode().strip()
        assert arch in ('x86_64', 'aarch64', 'arm64')
        build_env = env | {'CGO_ENABLED': '0', 'GOOS': 'linux', 'GOARCH': 'amd64' if arch == 'x86_64' else 'arm64'}
        r = subprocess.run(['go', 'build', '-o', str(root / 'time-contract'), './cmd/time-contract'], cwd=ROOT / 'backend', env=build_env, capture_output=True)
        assert r.returncode == 0, 'local operator build failed'
        (root / 'Dockerfile').write_text('FROM mysql:8.4\nCOPY time-contract /app/time-contract\nLABEL codex.task="'+LABEL+'"\n')
        run(docker + ['build', '--pull=false', '-t', image, str(root)], timeout=240)
        config = {'services': {
            'mysql': {'image': 'mysql:8.4', 'environment': {'MYSQL_ALLOW_EMPTY_PASSWORD': 'yes', 'MYSQL_DATABASE': 'blog'}, 'ports': [f'127.0.0.1:{port}:3306'], 'labels': {'codex.task': LABEL}},
            'backend': {'image': image, 'container_name': backend_name, 'labels': {'codex.task': LABEL}, 'entrypoint': [],
                'command': ['/bin/sh', '-ec', '/app/time-contract -conf /data/conf -mode status | grep -q \'"ready":true\'; exec sleep infinity'],
                'volumes': [str(conf)+':/data/conf:ro', (str(state) if bind_state else 'fixture-state')+':/app/data/time-contract:ro', str(uploads)+':/app/data/uploads:rw'],
                'healthcheck': {'test': ['CMD-SHELL', '/app/time-contract -conf /data/conf -mode status | grep -q \'"ready":true\''], 'interval': '1s', 'timeout': '5s', 'retries': 3}}}}
        if not bind_state:
            config['volumes'] = {'fixture-state': {'labels': {'codex.task': LABEL}}}
        compose_file = deploy / 'compose.prod.yml'
        compose_file.write_text(json.dumps(config))
        compose = docker + ['compose', '-p', project, '--project-directory', str(deploy), '-f', str(compose_file), '--env-file', str(deploy / '.env.prod')]

        def sql(statement):
            return run(compose + ['exec', '-T', 'mysql', 'mysql', '-uroot', '-N', 'blog'], statement.encode()).decode().strip()

        def info():
            return json.loads(run(docker + ['inspect', backend_name]))[0]

        def backup(path, duplicate=False):
            raw = run(compose + ['exec', '-T', 'mysql', 'mysqldump', '-uroot', *DUMP_FLAGS, 'blog'])
            if duplicate:
                lines = raw.splitlines(keepends=True)
                i = next(i for i, line in enumerate(lines) if line.startswith(b'INSERT INTO `users` '))
                lines.insert(i + 1, lines[i])
                raw = b''.join(lines)
            path.write_bytes(gzip.compress(raw, mtime=0))
            Path(str(path)+'.manifest.json').write_text(json.dumps(manifest.inspect_dump(path), indent=2)+'\n')

        try:
            run(compose + ['up', '-d', '--no-build', '--pull', 'never', 'mysql'])
            ready = False
            for _ in range(90):
                if run(compose + ['exec', '-T', 'mysql', 'mysqladmin', 'ping', '-uroot'], check=False).returncode == 0:
                    try:
                        with socket.create_connection(('127.0.0.1', port), timeout=1) as connection:
                            ready = len(connection.recv(4)) == 4
                    except OSError:
                        ready = False
                    if ready:
                        break
                time.sleep(1)
            assert ready, 'owned MySQL not ready'
            mysql_id = run(compose + ['ps', '-q', 'mysql']).decode().strip()
            mysql_info = json.loads(run(docker + ['inspect', mysql_id]))[0]
            assert mysql_info['Config']['Labels']['codex.task'] == LABEL
            assert mysql_info['NetworkSettings']['Ports']['3306/tcp'] == [{'HostIp': '127.0.0.1', 'HostPort': str(port)}]
            # Initialization gets an explicit service overlay, exactly as the repair requires.
            init = root / 'init.yml'
            init.write_text(json.dumps({'services': {'backend': {'volumes': [{'type': 'bind' if bind_state else 'volume', 'source': str(state) if bind_state else 'fixture-state', 'target': '/app/data/time-contract', 'read_only': False}]}}}))
            run(compose + ['-f', str(init), 'run', '--rm', '--no-deps', '--pull', 'never', '-T', 'backend', '/app/time-contract', '-conf', '/data/conf', '-mode', 'fresh-init', '-operation-id', project, '-writers-paused'])
            sql("INSERT INTO users(id,created_at,updated_at,email,password_hash,display_name,role,email_verified,credential_version) VALUES('00000000-0000-7000-8000-000000008001',UTC_TIMESTAMP(),UTC_TIMESTAMP(),'fixture@example.invalid','synthetic','Original',1,1,1);")
            run(compose + ['up', '-d', '--no-build', '--pull', 'never', 'backend'])
            for _ in range(30):
                if info()['State'].get('Health', {}).get('Status') == 'healthy':
                    break
                time.sleep(1)
            assert info()['State']['Health']['Status'] == 'healthy'
            assert next(m for m in info()['Mounts'] if m['Destination'] == '/app/data/time-contract')['RW'] is False
            good, bad = root / 'good.sql.gz', root / 'duplicate.sql.gz'
            backup(good)
            backup(bad, duplicate=True)
            upload_archive = root / 'uploads.tar.gz'
            with tarfile.open(upload_archive, 'w:gz') as archive:
                archive.add(uploads, arcname='uploads')
            sql("UPDATE users SET display_name='Changed';")
            (uploads / 'sentinel.txt').write_text('changed\n')
            shell_env = env | {'BLOG_RESTORE_WRITERS_PAUSED': '1', 'BLOG_RESTORE_EVIDENCE_DIR': str(root / 'evidence')}
            r = subprocess.run(['bash', str(deploy / 'restore.sh'), str(good), str(upload_archive)], env=shell_env, capture_output=True, timeout=180)
            (out / 'success.log').write_bytes(r.stdout+r.stderr)
            assert r.returncode == 0, 'real Compose restore failed; inspect success.log'
            assert sql('SELECT display_name FROM users;') == 'Original'
            assert (uploads / 'sentinel.txt').read_text() == 'backed-up synthetic upload\n'
            assert info()['State']['Running'] and info()['State']['Health']['Status'] == 'healthy'
            assert next(m for m in info()['Mounts'] if m['Destination'] == '/app/data/time-contract')['RW'] is False
            assert json.loads(run(compose + ['run', '--rm', '--no-deps', '--pull', 'never', '-T', 'backend', 'cat', '/app/data/time-contract/state.json']))['status'] == 'ready'
            original_id = info()['Id']
            sql("UPDATE users SET display_name='Changed before failure';")
            r = subprocess.run(['bash', str(deploy / 'restore.sh'), str(bad)], env=shell_env, capture_output=True, timeout=180)
            (out / 'failure.log').write_bytes(r.stdout+r.stderr)
            assert r.returncode != 0
            assert info()['Id'] == original_id and not info()['State']['Running']
            assert json.loads(run(compose + ['run', '--rm', '--no-deps', '--pull', 'never', '-T', 'backend', 'cat', '/app/data/time-contract/state.json']))['status'] == 'pending'
            assert sql('SELECT COUNT(*) FROM time_contract;') == '1', 'failure did not reach imported marker'
            assert sql('SELECT display_name FROM users;') == 'Original', 'failure did not reach users import after marker'
            for case in (root / 'evidence').iterdir():
                for evidence in case.glob('*mount*.json'):
                    shutil.copyfile(evidence, out / (case.name+'-'+evidence.name))
            result = {'passed': True, 'controlStorage': 'bind' if bind_state else 'named-volume', 'composeVersion': compose_version, 'actualDockerCompose': True, 'actualOperatorCLI': True, 'actualMySQL': True,
                'normalizedBackupAndUploadsRestored': True, 'runtimeStateMountReadOnly': True,
                'failureAfterMarkerLeavesPendingAndStopped': True, 'productionAccess': False, 'lifecycleShim': False, 'testOnlyContainerNameSubstitution': True}
            (out / 'summary.json').write_text(json.dumps(result, indent=2)+'\n')
            print(json.dumps(result))
        finally:
            ids = run(docker + ['ps', '-aq', '--filter', 'label=com.docker.compose.project='+project]).decode().split()
            for identifier in ids:
                labels = json.loads(run(docker + ['inspect', identifier]))[0]['Config']['Labels']
                assert labels.get('codex.task') == LABEL and labels.get('com.docker.compose.project') == project
            run(compose + ['down', '--volumes', '--remove-orphans'])
            image_meta = json.loads(run(docker + ['image', 'inspect', image]))[0]
            assert image_meta['Config']['Labels'].get('codex.task') == LABEL
            run(docker + ['image', 'rm', image])


if __name__ == '__main__':
    main()
