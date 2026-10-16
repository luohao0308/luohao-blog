"""Safety regression for the isolated identity rehearsal's child environment."""
import os
import unittest
from unittest.mock import patch

from test_wechat_identity import fixture_environment


class FixtureEnvironmentTest(unittest.TestCase):
    def test_operator_overrides_cannot_replace_fixture_configuration(self):
        synthetic = {
            "PATH": "/fixture/tools",
            "GOROOT": "/fixture/stale-go",
            "KRATOS_DATA_DATABASE_SOURCE": "refuse-this-synthetic-target",
            "KRATOSDATA_DATABASE_SOURCE": "refuse-this-other-synthetic-target",
            "KRATOS_WECHAT_ENABLED": "true",
            "KRATOS_OTHER_OVERRIDE": "fixture",
        }
        with patch.dict(os.environ, synthetic, clear=True):
            self.assertEqual(fixture_environment(), {"PATH": "/fixture/tools"})


if __name__ == "__main__":
    unittest.main()
