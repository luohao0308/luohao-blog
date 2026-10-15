#!/usr/bin/env python3
"""Exercise watchdog decisions using isolated state and fake Docker; never restart real services."""
import fcntl
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]

class WatchdogTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.dir = Path(self.tmp.name)
        self.bin = self.dir / "bin"
        self.bin.mkdir()
        self.state = self.dir / "state"
        self.state.mkdir()
        self.log = self.dir / "watchdog.log"
        self.restarts = self.dir / "restarts"
        shims = {
            "docker": "#!/bin/sh\ncase \"$1\" in\ninspect) if [ \"$FAKE_STATE\" = missing ]; then exit 1; fi; printf '%s\\n' \"$FAKE_STATE\";;\nrestart) echo restart >> \"$RESTARTS\"; exit \"${RESTART_FAIL:-0}\";;\n*) exit 2;; esac\n",
            "date": "#!/bin/sh\nif [ \"$1\" = +%s ]; then echo \"$FAKE_TIME\"; else echo test-time; fi\n",
            "curl": "#!/bin/sh\nexit 7\n",
            "flock": "#!/usr/bin/env python3\nimport fcntl,sys\ntry: fcntl.flock(int(sys.argv[-1]), fcntl.LOCK_EX|fcntl.LOCK_NB)\nexcept BlockingIOError: sys.exit(1)\n",
        }
        for name, body in shims.items():
            path = self.bin / name
            path.write_text(body)
            path.chmod(0o755)
        # Redirect legacy hard-coded state as well, so the red test is safe.
        source = (ROOT / "deploy/watchdog.sh").read_text()
        source = source.replace("/var/log/blog-watchdog.log", str(self.log)).replace("/tmp/blog-health-fails", str(self.state / "health-fails"))
        self.script = self.dir / "watchdog.sh"
        self.script.write_text(source)
        self.env = os.environ | {
            "PATH": str(self.bin) + os.pathsep + os.environ["PATH"],
            "BLOG_WATCHDOG_LOG": str(self.log),
            "BLOG_WATCHDOG_STATE_DIR": str(self.state),
            "RESTARTS": str(self.restarts),
        }

    def run_check(self, state="running true healthy", now=10000, restart_fail=False):
        result = subprocess.run(["bash", str(self.script)], env=self.env | {
            "FAKE_STATE": state, "FAKE_TIME": str(now), "RESTART_FAIL": "1" if restart_fail else "0",
        }, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)

    def count(self):
        return len(self.restarts.read_text().splitlines()) if self.restarts.exists() else 0

    def test_healthy_never_restarts(self):
        for _ in range(4): self.run_check()
        self.assertEqual(self.count(), 0)

    def test_two_unhealthy_checks_restart_once(self):
        self.run_check("running true unhealthy")
        self.assertEqual(self.count(), 0)
        self.run_check("running true unhealthy", 10300)
        self.assertEqual(self.count(), 1)

    def test_health_recovery_resets_failure_count(self):
        self.run_check("running true unhealthy")
        self.run_check()
        self.run_check("running true unhealthy")
        self.assertEqual(self.count(), 0)

    def test_uncertain_container_states_do_not_restart(self):
        for state in ("running true starting", "restarting true unhealthy", "exited false unhealthy", "running true none", "missing", "broken response"):
            with self.subTest(state=state):
                self.run_check("running true unhealthy")
                self.run_check(state)
                self.run_check("running true unhealthy")
                self.run_check()
        self.assertEqual(self.count(), 0)

    def test_cooldown_prevents_restart_storm(self):
        self.run_check("running true unhealthy", 10000)
        self.run_check("running true unhealthy", 10300)
        self.run_check("running true unhealthy", 10600)
        self.run_check("running true unhealthy", 10900)
        self.assertEqual(self.count(), 1)
        self.run_check("running true unhealthy", 12100)
        self.assertEqual(self.count(), 2)

    def test_failed_restart_has_cooldown(self):
        self.run_check("running true unhealthy", 10000)
        self.run_check("running true unhealthy", 10300, True)
        self.run_check("running true unhealthy", 10600)
        self.run_check("running true unhealthy", 10900)
        self.assertEqual(self.count(), 1)
        self.assertIn("restart failed", self.log.read_text())

    def test_overlap_skips_locked_run(self):
        lock = self.state / "lock"
        with lock.open("w") as f:
            fcntl.flock(f, fcntl.LOCK_EX|fcntl.LOCK_NB)
            self.run_check("running true unhealthy")
        self.assertFalse((self.state / "health-fails").exists())
        self.assertEqual(self.count(), 0)

    def test_alert_only_mode_never_restarts(self):
        self.env["BLOG_WATCHDOG_RESTART_ENABLED"] = "0"
        for _ in range(4): self.run_check("running true unhealthy")
        self.assertEqual(self.count(), 0)
        self.assertIn("alert only", self.log.read_text())

    def test_clock_rollback_and_corrupt_timestamp_do_not_restart(self):
        for stamp in ("12000", "not-a-timestamp", "9" * 25, "1:2", "08"):
            with self.subTest(stamp=stamp):
                (self.state / "last-restart").write_text(stamp)
                self.run_check("running true unhealthy", 10000)
                self.run_check("running true unhealthy", 10300)
                self.assertEqual(self.count(), 0)
                if stamp != "12000": self.assertIn("invalid restart clock", self.log.read_text())

    def test_malformed_counter_cannot_trigger_restart(self):
        (self.state / "health-fails").write_text("bad-state\n")
        self.run_check("running true unhealthy")
        self.assertEqual(self.count(), 0)

if __name__ == "__main__": unittest.main()
