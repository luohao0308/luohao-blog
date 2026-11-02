#!/usr/bin/env python3
"""Run UTC conversion acceptance only on this task's owned loopback MySQL."""
import json
import datetime
import os
from pathlib import Path
import re
import socket
import subprocess
import time
import uuid

from test_time_contract import local_docker_command
from time_restore_mysql_acceptance import run_acceptance

ROOT = Path(__file__).resolve().parents[1]
LABEL = 'utc-time-contract-integration'
PORT = 13330


def main():
    docker = local_docker_command()
    out = ROOT / 'output/time-contract/integration'
    out.mkdir(parents=True, exist_ok=True)
    (out / 'summary.json').write_text(json.dumps({'passed': False, 'status': 'running', 'startedAtUTC': datetime.datetime.now(datetime.timezone.utc).isoformat()}) + '\n')
    (out / 'restore-summary.json').write_text(json.dumps({'passed': False, 'status': 'pending'}) + '\n')
    name = 'utc-contract-' + uuid.uuid4().hex[:12]
    database = 'utc_contract_test_' + uuid.uuid4().hex[:12]
    env = {k: v for k, v in os.environ.items()
           if not k.startswith('KRATOS') and 'DSN' not in k and k not in {'GOROOT', 'DATABASE_SOURCE'}}
    env['BLOG_UTC_MYSQL_DSN'] = f'root@tcp(127.0.0.1:{PORT})/{database}?parseTime=true&multiStatements=true'

    def run(args):
        result = subprocess.run(args, capture_output=True, env=env)
        if result.returncode:
            log = out / 'fixture-failure.private.log'
            log.write_bytes(result.stdout + result.stderr)
            log.chmod(0o600)
            raise RuntimeError('owned UTC fixture command failed; private diagnostics withheld')
        return result.stdout

    try:
        run(docker + ['run', '-d', '--name', name, '--label', 'codex.task=' + LABEL,
                      '-p', f'127.0.0.1:{PORT}:3306', '-e', 'MYSQL_ALLOW_EMPTY_PASSWORD=yes',
                      '-e', 'MYSQL_DATABASE=' + database, '-e', 'TZ=Asia/Shanghai', 'mysql:8.4'])
        ready = False
        for _ in range(90):
            probe = subprocess.run(docker + ['exec', name, 'mysqladmin', 'ping', '-uroot'], capture_output=True)
            if probe.returncode == 0:
                try:
                    with socket.create_connection(('127.0.0.1', PORT), timeout=1) as connection:
                        ready = len(connection.recv(4)) == 4
                except OSError:
                    pass
            if ready:
                break
            time.sleep(1)
        assert ready, 'owned MySQL did not become ready'
        info = json.loads(run(docker + ['inspect', name]))[0]
        assert info['Config']['Labels'].get('codex.task') == LABEL
        assert info['NetworkSettings']['Ports']['3306/tcp'] == [{'HostIp': '127.0.0.1', 'HostPort': str(PORT)}]
        # Required acceptance names are updated only after the implementation exists.
        required = ["TestMySQLContract", "TestMySQLFreshInit", "TestMySQLConcurrentOperator",
                    "TestDataTimeContractMySQL", "TestContentReleaseGateMySQL",
                    "TestMySQLMixedLegacyRejected", "TestMySQLViewRejected",
                    "TestMySQLIncomingTimestampFKRejected", "TestMySQLOutgoingTimestampFKRejected"]
        pattern = '^(' + '|'.join(re.escape(name) for name in required) + ')$'
        result = subprocess.run(['go', 'test', './internal/timecontract', './internal/data', './cmd/content-release', '-run', pattern,
                                 '-count=1', '-timeout=180s', '-v'], cwd=ROOT / 'backend',
                                env=env, capture_output=True)
        log = result.stdout + result.stderr
        (out / 'go-tests.log').write_bytes(log)
        assert result.returncode == 0 and b'--- SKIP:' not in log and b'--- FAIL:' not in log
        for test in required:
            assert re.search(rb'^--- PASS: ' + test.encode() + rb'(?: \(|$)', log, re.M), test
        run_acceptance(docker, name, database, env, out)
        summary = {'passed': True, 'requiredTests': required, 'silentSkips': 0,
                   'mysqlTimezone': 'Asia/Shanghai', 'actualCLIRestoreAcceptance': True, 'productionAccess': False}
        (out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
        print(json.dumps(summary))
    finally:
        result = subprocess.run(docker + ['inspect', '--format', '{{index .Config.Labels "codex.task"}}', name], capture_output=True)
        if result.returncode == 0:
            assert result.stdout.decode().strip() == LABEL
            run(docker + ['rm', '-f', '-v', name])


if __name__ == '__main__':
    main()
