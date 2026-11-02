"""Safety regression for the isolated identity rehearsal's child environment."""
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from test_wechat_identity import fixture_environment, latest_schema_version, require_local_docker, run


class FixtureEnvironmentTest(unittest.TestCase):
    def test_application_schema_includes_migrations_after_identity_backfill(self):
        with tempfile.TemporaryDirectory() as directory:
            migrations = Path(directory)
            for name in ("000016_identity.up.sql", "000017_version.up.sql",
                         "000018_revisions.up.sql", "000019_future.down.sql"):
                (migrations / name).touch()
            self.assertEqual(latest_schema_version(migrations), 18)

    def test_missing_application_migrations_fail_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(ValueError):
                latest_schema_version(Path(directory))

    def test_operator_overrides_cannot_replace_fixture_configuration(self):
        synthetic = {
            "PATH": "/fixture/tools",
            "GOROOT": "/fixture/stale-go",
            "KRATOS_DATA_DATABASE_SOURCE": "refuse-this-synthetic-target",
            "KRATOSDATA_DATABASE_SOURCE": "refuse-this-other-synthetic-target",
            "KRATOS_WECHAT_ENABLED": "true",
            "KRATOS_OTHER_OVERRIDE": "fixture",
            "BLOG_MINI_CONTENT_MYSQL_DSN": "refuse-inherited-mini-target",
            "BLOG_WECHAT_IDENTITY_MYSQL_DSN": "refuse-inherited-identity-target",
            "MYSQL_MIGRATION_DSN": "refuse-inherited-migration-target",
        }
        with patch.dict(os.environ, synthetic, clear=True):
            self.assertEqual(fixture_environment(), {"PATH": "/fixture/tools"})


    def test_remote_daemon_is_refused_before_any_fixture_access(self):
        for host in ("ssh://fixture-refused.invalid", "tcp://192.0.2.1:2376"):
            with self.subTest(host=host), patch.dict(os.environ, {"DOCKER_HOST": host}, clear=True), patch("test_wechat_identity.subprocess.check_output") as call:
                with self.assertRaisesRegex(RuntimeError, "refusing a non-local Docker host"):
                    require_local_docker()
                call.assert_not_called()

    def test_remote_context_is_refused_and_local_context_is_returned(self):
        with patch.dict(os.environ, {}, clear=True), patch("test_wechat_identity.subprocess.check_output", side_effect=["refused\n", '\"ssh://fixture-refused.invalid\"']):
            with self.assertRaisesRegex(RuntimeError, "refusing a non-local Docker context"):
                require_local_docker()
        with patch.dict(os.environ, {}, clear=True), patch("test_wechat_identity.subprocess.check_output", side_effect=["owned-local\n", '\"unix:///fixture/docker.sock\"']):
            self.assertEqual(require_local_docker(), "owned-local")

    def test_docker_commands_are_pinned_to_the_checked_context(self):
        with patch("test_wechat_identity.DOCKER_CONTEXT", "owned-local"), patch("test_wechat_identity.subprocess.run") as call:
            call.return_value.returncode = 0
            call.return_value.stdout = b"fixture"
            self.assertEqual(run(["docker", "inspect", "owned-fixture"]), b"fixture")
            self.assertEqual(call.call_args.args[0], ["docker", "--context", "owned-local", "inspect", "owned-fixture"])


if __name__ == "__main__":
    unittest.main()
