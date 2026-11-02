"""Safety regression for the isolated identity rehearsal's child environment."""
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from test_wechat_identity import fixture_environment, latest_schema_version


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


if __name__ == "__main__":
    unittest.main()
