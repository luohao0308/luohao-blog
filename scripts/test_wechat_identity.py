#!/usr/bin/env python3
"""Rehearse v15→v16 and real-driver identity tests on isolated Docker fixtures.

No production configuration is loaded. The default creates disposable fixtures
on loopback ports 13316/16389 and removes only the containers it created. An
existing task-owned fixture may be supplied as private JSON for local reuse.
"""
import argparse
import gzip
import json
import os
from pathlib import Path
import re
import secrets
import subprocess
import tempfile
import time
import uuid

from test_mysql_migrations import GO_HELPER

ROOT = Path(__file__).resolve().parents[1]
LABEL = "formal-miniprogram"


def run(args, *, data=None, env=None, cwd=None):
    result = subprocess.run(args, input=data, capture_output=True, env=env, cwd=cwd)
    if result.returncode:
        # Captured output/arguments may include DSNs. Never echo them.
        raise RuntimeError("fixture command failed: " + Path(args[0]).name)
    return result.stdout


def inspect(container):
    return json.loads(run(["docker", "inspect", container]))[0]


def fixture_environment():
    # The backfill CLI supports KRATOS overrides for operator use. A rehearsal
    # must never inherit those overrides over its generated fixture config.
    return {key: value for key, value in os.environ.items()
            if key != "GOROOT" and not key.startswith("KRATOS")}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--existing-fixture", type=Path,
                        help="private JSON with task-owned mysqlContainer, redisContainer and MYSQL_ROOT_PASSWORD")
    opts = parser.parse_args()
    owned, databases = [], []
    created_source = False
    source = ROOT / "backend/cmd/.wechat-migration-check"
    env = fixture_environment()
    try:
        if opts.existing_fixture:
            fixture = json.loads(opts.existing_fixture.read_text())
        else:
            password = secrets.token_hex(24)
            suffix = uuid.uuid4().hex[:10]
            fixture = {"MYSQL_ROOT_PASSWORD": password}
            for kind, port, image, extra in [
                ("mysql", "13316:3306", "mysql:8.4", ["-e", "MYSQL_ROOT_PASSWORD=" + password]),
                ("redis", "16389:6379", "redis:7.4-alpine", []),
            ]:
                command = ["docker", "run", "-d", "--name", f"formal-mini-{kind}-{suffix}",
                           "--label", "codex.task=" + LABEL, "-p", "127.0.0.1:" + port] + extra + [image]
                if kind == "redis":
                    command += ["redis-server", "--save", "", "--appendonly", "no"]
                container = run(command).decode().strip()
                owned.append(container)
                fixture[kind + "Container"] = container
        mysql_id, redis_id = fixture["mysqlContainer"], fixture["redisContainer"]
        password = fixture["MYSQL_ROOT_PASSWORD"]
        assert re.fullmatch(r"[a-f0-9]{48}", password)
        for container, service_port, host_port in [(mysql_id, "3306/tcp", "13316"),
                                                    (redis_id, "6379/tcp", "16389")]:
            info = inspect(container)
            assert info["Config"]["Labels"].get("codex.task") == LABEL
            assert info["NetworkSettings"]["Ports"][service_port] == [{"HostIp": "127.0.0.1", "HostPort": host_port}]
            assert info["State"]["Running"]

        def sql(statement, database=None):
            if database is not None:
                assert re.fullmatch(r"website_completion_[a-f0-9_]+", database)
            command = 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot -N --default-character-set=utf8mb4'
            if database:
                command += " " + database
            return run(["docker", "exec", "-i", mysql_id, "sh", "-c", command], data=statement.encode()).decode()

        for attempt in range(60):
            try:
                sql("SELECT 1;")
                break
            except RuntimeError:
                if attempt == 59:
                    raise
                time.sleep(1)
        database = "website_completion_" + uuid.uuid4().hex[:12]
        restored = database + "_b"
        for name in (database, restored):
            sql(f"CREATE DATABASE `{name}` CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;")
            databases.append(name)
        env["MYSQL_MIGRATION_DSN"] = f"root:{password}@tcp(127.0.0.1:13316)/{database}?multiStatements=true&parseTime=true"
        assert not source.exists(), "another rehearsal owns migration helper"
        source.mkdir()
        created_source = True
        (source / "main.go").write_text(GO_HELPER)
        with tempfile.TemporaryDirectory(prefix="wechat-identity-fixture-") as temporary:
            binary = str(Path(temporary) / "migrate")
            run(["go", "build", "-o", binary, str(source)], cwd=ROOT / "backend", env=env)
            assert run([binary, "15"], env=env).decode().strip() == "15 false"
            sentinel = "00000000-0000-0000-0000-000000000002"
            sql("INSERT INTO users(id,created_at,updated_at,email,password_hash,display_name,role,avatar_url,wechat_openid,email_verified,credential_version) VALUES("
                f"'{sentinel}',NOW(),NOW(),'migration@example.invalid','fixture-hash','保留昵称',2,'/v1/assets/avatars/fixture.png','legacy-fixture',1,7);", database)
            snapshot = sql("SELECT * FROM users ORDER BY id;", database)
            sql("INSERT INTO articles(id,created_at,updated_at,slug,title,summary,content_md,content_html,status,view_count,like_count) VALUES('00000000-0000-0000-0000-000000000001',NOW(),NOW(),'identity-fixture','保留文章','摘要','# 文章','<h1>文章</h1>',2,7,3);", database)
            article_snapshot = sql("SELECT * FROM articles ORDER BY id;", database)
            assert run([binary, "16"], env=env).decode().strip() == "16 false"
            assert sql("SELECT * FROM users ORDER BY id;", database) == snapshot
            assert sql("SELECT * FROM articles ORDER BY id;", database) == article_snapshot
            assert sql("SELECT COUNT(*) FROM wechat_identities;", database).strip() == "0"
            assert run([binary, "16"], env=env).decode().strip() == "16 false"

            # Run the real command against a synthetic private config, default
            # dry-run and explicit apply; never contact WeChat or run a server.
            config = Path(temporary) / "conf"
            config.mkdir(mode=0o700)
            config_file = config / "fixture.yaml"
            config_file.write_text("data:\n  database:\n    source: " + json.dumps(env["MYSQL_MIGRATION_DSN"]) + "\n")
            config_file.chmod(0o600)
            backfill = str(Path(temporary) / "backfill")
            run(["go", "build", "-o", backfill, "./cmd/wechat-identity-backfill"], cwd=ROOT / "backend", env=env)
            base = [backfill, "-conf", str(config), "-legacy-app-id", "wx1111111111111111"]
            assert not json.loads(run(base, env=env))["Applied"]
            assert sql("SELECT COUNT(*) FROM wechat_identities;", database).strip() == "0"
            first = json.loads(run(base + ["-apply", "-writers-paused"], env=env))
            assert first["Applied"] and first["Inserted"] == 1
            repeat = json.loads(run(base + ["-apply", "-writers-paused"], env=env))
            assert repeat["Existing"] == 1 and repeat["Inserted"] == 0
            assert sql("SELECT * FROM users ORDER BY id;", database) == snapshot

            # Upgrade backup restored to a second disposable DB, including
            # scoped identity, with schema version and legacy columns intact.
            dump = run(["docker", "exec", mysql_id, "sh", "-c",
                        'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump -uroot --single-transaction --skip-comments --no-tablespaces --set-gtid-purged=OFF ' + database])
            sql(gzip.decompress(gzip.compress(dump)).decode(), restored)
            assert sql("SELECT * FROM users ORDER BY id;", restored) == snapshot
            assert sql("SELECT * FROM articles ORDER BY id;", restored) == article_snapshot
            assert sql("SELECT version,dirty FROM schema_migrations;", restored).strip() == "16\t0"
            assert sql("SELECT user_id,app_id,openid FROM wechat_identities;", restored) == sql("SELECT user_id,app_id,openid FROM wechat_identities;", database)
            # Repositories own only their generated rows; remove the rehearsal
            # sentinel before backfill tests whose fixtures count all holders.
            sql(f"DELETE FROM users WHERE id='{sentinel}';", database)
            env["BLOG_WECHAT_IDENTITY_MYSQL_DSN"] = env["MYSQL_MIGRATION_DSN"]
            env["REDIS_INTEGRATION_ADDR"] = "127.0.0.1:16389"
            output = run(["go", "test", "./internal/data", "-run", "TestWechat", "-count=1", "-v"], cwd=ROOT / "backend", env=env).decode()
            assert "--- SKIP:" not in output and "--- FAIL:" not in output
            required = ["TestWechatIdentityMySQL", "TestWechatIdentityMySQLConcurrentUnique",
                        "TestWechatIdentityMySQLPasswordLockOrdering", "TestWechatIdentityMySQLBackfill",
                        "TestWechatIdentityMySQLBackfillCommitUnknown",
                        "TestWechatBindingStoreRealRedis", "TestWechatHTTPDisabledPreservesWebsiteAuthAndRead",
                        "TestWechatHTTPDisabledMySQLPreservesWebsiteAuthAndRead",
                        "TestWechatHTTPFormalBindingPreservesLegacyAndWebsiteAccount",
                        "TestWechatHTTPBindingNegativeCases", "TestWechatHTTPPostIssueResetRealRedis"]
            # Test names are an explicit no-silent-skip gate, not merely exit 0.
            for name in required:
                assert re.search(r"^--- PASS: " + re.escape(name) + r"(?: \(|$)", output, re.MULTILINE), "required real-driver test missing: " + name
            print(json.dumps({"baseline": 15, "target": 16, "dirty": False,
                              "legacy_preserved": True, "no_automatic_backfill": True,
                              "cli_dryrun_apply_idempotent": True, "backup_restore": True,
                              "real_mysql_redis_http_tests": "passed", "silent_skips": 0}))
    finally:
        if "sql" in locals():
            for database in databases:
                sql(f"DROP DATABASE `{database}`;")
        if created_source:
            (source / "main.go").unlink()
            source.rmdir()
        for container in reversed(owned):
            assert inspect(container)["Config"]["Labels"].get("codex.task") == LABEL
            run(["docker", "rm", "-f", "-v", container])


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print("WeChat fixture verification failed: " + type(error).__name__)
        raise SystemExit(1)
