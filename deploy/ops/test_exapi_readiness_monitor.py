#!/usr/bin/env python3
"""Isolated regression tests for the off-host ExAPI readiness monitor.

These tests deliberately do not contact ExAPI or the desktop notification
daemon.  Every test uses a disposable directory below the checkout-local
``tmp/`` folder and, where a process boundary is useful, an executable fake
``curl`` or ``notify-send`` program.

Run with:

    PYTHONDONTWRITEBYTECODE=1 python3 -m unittest deploy/ops/test_exapi_readiness_monitor.py
"""

from __future__ import annotations

import datetime as dt
import importlib.util
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import unittest


sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[2]
MODULE_PATH = ROOT / "deploy" / "ops" / "exapi_readiness_monitor.py"
ADAPTER_DIR = ROOT / "deploy" / "ops" / "adapters"
TMP_ROOT = ROOT / "tmp"


def load_monitor_module():
    spec = importlib.util.spec_from_file_location("exapi_readiness_monitor_under_test", MODULE_PATH)
    if spec is None or spec.loader is None:  # pragma: no cover - a broken checkout
        raise RuntimeError(f"cannot load {MODULE_PATH}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


MONITOR = load_monitor_module()
UTC = dt.timezone.utc
EMPTY_SHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"


class FakeNotifier:
    """A deterministic in-process notification sink."""

    def __init__(self, results: list[tuple[bool, int]] | None = None) -> None:
        self.calls: list[tuple[str, dict[str, object]]] = []
        self.results = list(results or [])

    def __call__(self, kind, probe, _config):
        self.calls.append((kind, dict(probe)))
        return self.results.pop(0) if self.results else (True, 0)


class ReadinessMonitorTestCase(unittest.TestCase):
    def setUp(self) -> None:
        TMP_ROOT.mkdir(exist_ok=True)
        self.temporary = tempfile.TemporaryDirectory(prefix="readiness-monitor-test-", dir=TMP_ROOT)
        self.work = Path(self.temporary.name)
        self.state_dir = self.work / "state"
        self.base_time = dt.datetime(2026, 9, 18, 20, 0, tzinfo=UTC)

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def config(self, **overrides):
        values = {
            "target_url": "https://sub2api.research.for-immortal.cn/ready",
            "state_dir": self.state_dir,
            "failure_threshold": 3,
            "recovery_threshold": 2,
            "notification_cooldown_seconds": 10 * 60,
            "delivery_retry_seconds": 5 * 60,
            "reminder_seconds": 6 * 60 * 60,
            "max_event_bytes": 16 * 1024,
            "max_event_lines": 100,
            "max_response_bytes": 1024,
            "connect_timeout_seconds": 1,
            "request_timeout_seconds": 2,
            # Unit-test timestamps are sometimes advanced by more than the
            # production 30-second cadence to exercise cooldowns/retries.
            # Keep gap-reset behavior out of those focused assertions.
            "probe_gap_reset_seconds": 3600,
        }
        values.update(overrides)
        return MONITOR.MonitorConfig(**values)

    @staticmethod
    def healthy_probe() -> dict[str, object]:
        return {
            "status": "healthy",
            "http_status": 200,
            "curl_exit_code": 0,
            "body_sha256": EMPTY_SHA256,
            "body_size": 18,
            "failure_reason": None,
        }

    @staticmethod
    def unhealthy_probe(*, http_status: int = 503, curl_exit_code: int = 0, reason: str = "upstream_unavailable") -> dict[str, object]:
        return {
            "status": "unhealthy",
            "http_status": http_status,
            "curl_exit_code": curl_exit_code,
            "body_sha256": EMPTY_SHA256,
            "body_size": 0,
            "failure_reason": reason,
        }

    def at(self, seconds: int) -> dt.datetime:
        return self.base_time + dt.timedelta(seconds=seconds)

    def run_probe(self, config, notifier, probe, seconds: int):
        return MONITOR.run_once(config, now=self.at(seconds), probe=probe, notifier=notifier)

    def read_state(self) -> dict[str, object]:
        return json.loads(self.config().state_path.read_text(encoding="utf-8"))

    def read_proof(self) -> dict[str, object]:
        return json.loads(self.config().proof_path.read_text(encoding="utf-8"))

    def filesystem_honors_private_modes(self) -> bool:
        """The checkout may be on exFAT/NTFS, which ignores chmod()."""

        probe = self.work / ".mode-probe"
        probe.write_bytes(b"x")
        try:
            probe.chmod(0o600)
            return stat.S_IMODE(probe.stat().st_mode) == 0o600
        finally:
            probe.unlink(missing_ok=True)

    def establish_healthy(self, config, notifier) -> None:
        self.run_probe(config, notifier, self.healthy_probe(), 0)
        self.run_probe(config, notifier, self.healthy_probe(), 30)

    def test_initial_healthy_requires_stabilization_without_recovery_notice(self) -> None:
        """Startup cannot manufacture a recovery notification from unknown state."""

        config = self.config()
        notifier = FakeNotifier()

        first = self.run_probe(config, notifier, self.healthy_probe(), 0)
        second = self.run_probe(config, notifier, self.healthy_probe(), 30)

        self.assertEqual("unknown", first["status"])
        self.assertEqual(1, first["success_streak"])
        self.assertEqual("healthy", second["status"])
        self.assertEqual(2, second["success_streak"])
        self.assertEqual([], notifier.calls)

    def test_one_or_two_failed_probes_are_logged_but_do_not_notify(self) -> None:
        config = self.config()
        notifier = FakeNotifier()
        self.establish_healthy(config, notifier)

        first = self.run_probe(config, notifier, self.unhealthy_probe(), 60)
        second = self.run_probe(config, notifier, self.unhealthy_probe(), 90)
        recovered_raw_probe = self.run_probe(config, notifier, self.healthy_probe(), 120)

        self.assertEqual("healthy", first["status"])
        self.assertEqual("healthy", second["status"])
        self.assertEqual(2, second["failure_streak"])
        self.assertEqual("healthy", recovered_raw_probe["status"])
        self.assertEqual(0, recovered_raw_probe["failure_streak"])
        self.assertEqual([], notifier.calls)

    def test_three_failures_alert_once_and_two_successes_recover_once(self) -> None:
        """The normal 30s cadence gives a 90s failure and 60s recovery gate."""

        config = self.config()
        notifier = FakeNotifier()
        self.establish_healthy(config, notifier)

        self.run_probe(config, notifier, self.unhealthy_probe(), 60)
        self.run_probe(config, notifier, self.unhealthy_probe(), 90)
        alert = self.run_probe(config, notifier, self.unhealthy_probe(), 120)
        self.run_probe(config, notifier, self.unhealthy_probe(), 150)
        first_success = self.run_probe(config, notifier, self.healthy_probe(), 180)
        recovery = self.run_probe(config, notifier, self.healthy_probe(), 210)

        self.assertEqual("unhealthy", alert["status"])
        self.assertTrue(alert["alert_sent"])
        self.assertEqual("unhealthy", first_success["status"])
        self.assertEqual("healthy", recovery["status"])
        self.assertTrue(recovery["alert_sent"])
        self.assertEqual(["unhealthy", "healthy"], [kind for kind, _probe in notifier.calls])

        proof = self.read_proof()
        self.assertEqual(1, proof["schema_version"])
        self.assertEqual("unhealthy", proof["last_alert"]["status"])
        self.assertEqual("healthy", proof["last_recovery"]["status"])

    def test_failed_delivery_retries_only_after_delivery_backoff(self) -> None:
        config = self.config(failure_threshold=1, recovery_threshold=1)
        notifier = FakeNotifier(results=[(False, 1), (True, 0)])
        self.establish_healthy(config, notifier)

        failed_delivery = self.run_probe(config, notifier, self.unhealthy_probe(), 60)
        before_backoff = self.run_probe(config, notifier, self.unhealthy_probe(), 120)
        retry = self.run_probe(config, notifier, self.unhealthy_probe(), 60 + config.delivery_retry_seconds)

        self.assertFalse(failed_delivery["alert_sent"])
        self.assertEqual("delivery_retry_backoff", before_backoff["suppression_reason"])
        self.assertTrue(retry["alert_sent"])
        self.assertEqual(["unhealthy", "unhealthy"], [kind for kind, _probe in notifier.calls])

    def test_failed_recovery_delivery_has_its_own_retry_backoff(self) -> None:
        """A critical alert must not throttle or spam recovery delivery."""

        config = self.config(failure_threshold=1, recovery_threshold=1)
        notifier = FakeNotifier(results=[(True, 0), (False, 1), (True, 0)])
        self.establish_healthy(config, notifier)

        self.run_probe(config, notifier, self.unhealthy_probe(), 60)
        failed_recovery = self.run_probe(config, notifier, self.healthy_probe(), 90)
        before_backoff = self.run_probe(config, notifier, self.healthy_probe(), 120)
        retry = self.run_probe(config, notifier, self.healthy_probe(), 90 + config.delivery_retry_seconds)

        self.assertFalse(failed_recovery["alert_sent"])
        self.assertEqual("delivery_retry_backoff", before_backoff["suppression_reason"])
        self.assertTrue(retry["alert_sent"])
        self.assertEqual(["unhealthy", "healthy", "healthy"], [kind for kind, _probe in notifier.calls])

    def test_long_incident_reminder_is_rate_limited(self) -> None:
        config = self.config(failure_threshold=1, recovery_threshold=1, reminder_seconds=3600)
        notifier = FakeNotifier()
        self.establish_healthy(config, notifier)

        self.run_probe(config, notifier, self.unhealthy_probe(), 60)
        reminder = self.run_probe(config, notifier, self.unhealthy_probe(), 3660)
        after_reminder = self.run_probe(config, notifier, self.unhealthy_probe(), 3690)

        self.assertTrue(reminder["alert_sent"])
        self.assertFalse(after_reminder["alert_sent"])
        self.assertEqual(["unhealthy", "unhealthy"], [kind for kind, _probe in notifier.calls])

    def test_cooldown_begins_at_confirmed_recovery_and_defers_a_persistent_realert(self) -> None:
        """A flap just after recovery must not create a second alert storm."""

        config = self.config(failure_threshold=1, recovery_threshold=1, delivery_retry_seconds=0)
        notifier = FakeNotifier()
        self.establish_healthy(config, notifier)

        self.run_probe(config, notifier, self.unhealthy_probe(), 60)       # first alert
        self.run_probe(config, notifier, self.unhealthy_probe(), 1200)     # outage remains open
        self.run_probe(config, notifier, self.healthy_probe(), 1230)       # recovered
        suppressed = self.run_probe(config, notifier, self.unhealthy_probe(), 1260)
        self.run_probe(config, notifier, self.unhealthy_probe(), 1860)     # recovery + 10m (plus cadence margin)

        self.assertEqual(["unhealthy", "healthy", "unhealthy"], [kind for kind, _probe in notifier.calls])
        self.assertEqual("notification_cooldown", suppressed["suppression_reason"])

    def test_cooldown_suppressed_flap_never_emits_a_recovery(self) -> None:
        config = self.config(failure_threshold=1, recovery_threshold=1, delivery_retry_seconds=0)
        notifier = FakeNotifier()
        self.establish_healthy(config, notifier)
        self.run_probe(config, notifier, self.unhealthy_probe(), 60)
        self.run_probe(config, notifier, self.healthy_probe(), 90)
        self.run_probe(config, notifier, self.unhealthy_probe(), 120)
        self.run_probe(config, notifier, self.healthy_probe(), 150)

        self.assertEqual(["unhealthy", "healthy"], [kind for kind, _probe in notifier.calls])

    def test_restart_preserves_streaks_and_opens_only_one_incident(self) -> None:
        config = self.config()
        notifier = FakeNotifier()
        self.establish_healthy(config, notifier)
        self.run_probe(config, notifier, self.unhealthy_probe(), 60)
        self.run_probe(config, notifier, self.unhealthy_probe(), 90)

        restarted_config = self.config()
        alert = self.run_probe(restarted_config, notifier, self.unhealthy_probe(), 120)
        repeat = self.run_probe(restarted_config, notifier, self.unhealthy_probe(), 150)

        self.assertTrue(alert["alert_sent"])
        self.assertFalse(repeat["alert_sent"])
        self.assertEqual(["unhealthy"], [kind for kind, _probe in notifier.calls])

    def test_v1_unhealthy_state_and_malformed_counters_cannot_create_an_immediate_alert_or_crash(self) -> None:
        config = self.config()
        MONITOR.ensure_state_dir(config.state_dir)
        config.state_path.write_text(
            json.dumps(
                {
                    "status": "unhealthy",
                    "observed_at": MONITOR.timestamp(self.at(0)),
                    "target_url": config.target_url,
                    "failure_streak": "not-a-number",
                }
            ),
            encoding="utf-8",
        )
        notifier = FakeNotifier()

        event = self.run_probe(config, notifier, self.unhealthy_probe(), 30)

        self.assertIn(event["status"], {"unknown", "unhealthy"})
        self.assertEqual([], notifier.calls)
        state = self.read_state()
        self.assertIsInstance(state["failure_streak"], int)
        self.assertEqual(2, state["schema_version"])

    def test_target_change_resets_hysteresis_without_a_synthetic_alert(self) -> None:
        original = self.config()
        notifier = FakeNotifier()
        self.establish_healthy(original, notifier)
        self.run_probe(original, notifier, self.unhealthy_probe(), 60)
        changed = self.config(target_url="https://different.example.test/ready")

        event = self.run_probe(changed, notifier, self.unhealthy_probe(), 90)

        self.assertEqual("unknown", event["status"])
        self.assertEqual([], notifier.calls)

    def test_state_proof_and_event_log_are_private(self) -> None:
        if not self.filesystem_honors_private_modes():
            self.skipTest("checkout filesystem does not preserve chmod modes")
        config = self.config()
        notifier = FakeNotifier()
        self.run_probe(config, notifier, self.healthy_probe(), 0)

        self.assertEqual(0o700, stat.S_IMODE(config.state_dir.stat().st_mode))
        for path in (config.state_path, config.proof_path, config.events_path, config.lock_path):
            self.assertTrue(path.exists(), path)
            self.assertEqual(0o600, stat.S_IMODE(path.stat().st_mode), path)

    def test_event_retention_honors_line_and_byte_bounds_and_keeps_ndjson_valid(self) -> None:
        config = self.config()
        MONITOR.ensure_state_dir(config.state_dir)
        for index in range(101):
            MONITOR.append_event(config.events_path, {"index": index}, config)
        lines = config.events_path.read_text(encoding="utf-8").splitlines()
        self.assertEqual(100, len(lines))
        self.assertEqual(1, json.loads(lines[0])["index"])
        if self.filesystem_honors_private_modes():
            self.assertEqual(0o600, stat.S_IMODE(config.events_path.stat().st_mode))

        for index in range(100):
            MONITOR.append_event(config.events_path, {"index": index, "padding": "x" * 1000}, config)
        payload = config.events_path.read_bytes()
        self.assertLessEqual(len(payload), config.max_event_bytes)
        for line in payload.splitlines():
            self.assertIsInstance(json.loads(line), dict)

    def write_fake_curl(self) -> Path:
        binary = self.work / "fake-curl"
        binary.write_text(
            "#!/usr/bin/env python3\n"
            "import os, pathlib, sys\n"
            "args = sys.argv[1:]\n"
            "def value(flag): return args[args.index(flag) + 1]\n"
            "pathlib.Path(value('--dump-header')).write_text(os.environ.get('FAKE_HEADERS', 'Content-Type: application/json\\r\\n'), encoding='utf-8')\n"
            "if os.environ.get('FAKE_WRITE_BODY', '1') == '1':\n"
            "    pathlib.Path(value('--output')).write_text(os.environ.get('FAKE_BODY', '{\\\"status\\\":\\\"ready\\\"}'), encoding='utf-8')\n"
            "print(os.environ.get('FAKE_HTTP_STATUS', '200'))\n"
            "raise SystemExit(int(os.environ.get('FAKE_CURL_EXIT', '0')))\n",
            encoding="utf-8",
        )
        binary.chmod(0o700)
        return binary

    def test_probe_contract_classification_and_stale_body_isolation(self) -> None:
        fake_curl = self.write_fake_curl()
        config = self.config(curl_bin=str(fake_curl))
        original_environment = dict(os.environ)
        try:
            fixtures = [
                ({"FAKE_BODY": '{"status":"ready"}'}, "healthy", None),
                ({"FAKE_BODY": '{"status":"ok"}'}, "healthy", None),
                ({"FAKE_HEADERS": "Content-Type: text/plain\\r\\n"}, "unhealthy", "readiness_contract"),
                ({"FAKE_BODY": "not json"}, "unhealthy", "readiness_contract"),
                ({"FAKE_BODY": '{"status":"not-ready"}'}, "unhealthy", "readiness_contract"),
                ({"FAKE_HTTP_STATUS": "503"}, "unhealthy", "upstream_unavailable"),
                ({"FAKE_CURL_EXIT": "6", "FAKE_HTTP_STATUS": "0", "FAKE_WRITE_BODY": "0"}, "unhealthy", "dns_resolution"),
                ({"FAKE_CURL_EXIT": "28", "FAKE_HTTP_STATUS": "0", "FAKE_WRITE_BODY": "0"}, "unhealthy", "timeout"),
                ({"FAKE_BODY": "x" * 1025}, "unhealthy", "response_too_large_or_invalid"),
            ]
            for values, expected_status, expected_reason in fixtures:
                for key in ("FAKE_HEADERS", "FAKE_BODY", "FAKE_HTTP_STATUS", "FAKE_CURL_EXIT", "FAKE_WRITE_BODY"):
                    os.environ.pop(key, None)
                os.environ.update(values)
                with self.subTest(values=values):
                    result = MONITOR.probe_readiness(config)
                    self.assertEqual(expected_status, result["status"])
                    self.assertEqual(expected_reason, result["failure_reason"])
                    if values.get("FAKE_WRITE_BODY") == "0":
                        self.assertEqual(EMPTY_SHA256, result["body_sha256"])
        finally:
            os.environ.clear()
            os.environ.update(original_environment)

    def test_notification_subprocess_never_receives_target_query_secret(self) -> None:
        notification_log = self.work / "notifications.jsonl"
        fake_notify = self.work / "fake-notify"
        fake_notify.write_text(
            "#!/usr/bin/env python3\n"
            "import json, os, pathlib, sys\n"
            "with pathlib.Path(os.environ['NOTIFY_LOG']).open('a', encoding='utf-8') as handle:\n"
            "    handle.write(json.dumps(sys.argv[1:]) + '\\n')\n",
            encoding="utf-8",
        )
        fake_notify.chmod(0o700)
        config = self.config(notify_send_bin=str(fake_notify))
        # Simulate a legacy/in-memory caller that bypassed configuration
        # validation; notification text must still redact query material.
        config.target_url = "https://sub2api.research.for-immortal.cn/ready?token=never-disclose-this"
        old = os.environ.get("NOTIFY_LOG")
        os.environ["NOTIFY_LOG"] = str(notification_log)
        try:
            delivered, exit_code = MONITOR.send_notification("unhealthy", self.unhealthy_probe(), config)
        finally:
            if old is None:
                os.environ.pop("NOTIFY_LOG", None)
            else:
                os.environ["NOTIFY_LOG"] = old
        self.assertTrue(delivered)
        self.assertEqual(0, exit_code)
        self.assertNotIn("never-disclose-this", notification_log.read_text(encoding="utf-8"))

    def test_target_url_rejects_query_and_fragment_material(self) -> None:
        """Probe state/proof must never become a credential side channel."""

        for target in (
            "https://sub2api.research.for-immortal.cn/ready?token=secret",
            "https://sub2api.research.for-immortal.cn/ready#fragment",
        ):
            with self.subTest(target=target):
                with self.assertRaises(ValueError):
                    self.config(target_url=target)

    def test_dry_run_does_not_consume_notification_retry_budget(self) -> None:
        fake_curl = self.write_fake_curl()
        state_dir = self.work / "dry-run-state"
        environment = dict(os.environ)
        environment.update(
            {
                "EXAPI_MONITOR_STATE_DIR": str(state_dir),
                "EXAPI_MONITOR_CURL_BIN": str(fake_curl),
                "EXAPI_MONITOR_FAILURE_THRESHOLD": "1",
                "EXAPI_MONITOR_RECOVERY_THRESHOLD": "1",
                "FAKE_HTTP_STATUS": "503",
                "FAKE_BODY": '{"status":"not-ready"}',
            }
        )
        completed = subprocess.run(
            [sys.executable, str(MODULE_PATH), "--dry-run"],
            cwd=ROOT,
            env=environment,
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(0, completed.returncode, completed.stderr)
        state = json.loads((state_dir / "state.json").read_text(encoding="utf-8"))
        self.assertIsNone(state["last_notification_attempt_at"])
        proof = json.loads((state_dir / "alert-delivery-evidence.json").read_text(encoding="utf-8"))
        self.assertNotIn("delivery_accepted", proof)

    def test_second_process_skips_while_the_monitor_lock_is_held(self) -> None:
        config = self.config()
        MONITOR.ensure_state_dir(config.state_dir)
        environment = dict(os.environ)
        environment.update({"EXAPI_MONITOR_STATE_DIR": str(config.state_dir)})

        with MONITOR.monitor_lock(config.lock_path) as acquired:
            self.assertTrue(acquired)
            completed = subprocess.run(
                [sys.executable, str(MODULE_PATH), "--dry-run"],
                cwd=ROOT,
                env=environment,
                text=True,
                capture_output=True,
                check=False,
            )
        self.assertEqual(0, completed.returncode, completed.stderr)
        self.assertFalse(config.events_path.exists())

    def test_current_proof_remains_compatible_with_both_rollout_adapters(self) -> None:
        """Proof format remains v1 even when monitor event/state formats evolve."""

        config = self.config(delivery_retry_seconds=0)
        notifier = FakeNotifier()
        now = MONITOR.utc_now() - dt.timedelta(minutes=1)
        times = [now + dt.timedelta(seconds=offset) for offset in (0, 5, 10, 15, 20, 25, 30)]
        for current in times[:2]:
            MONITOR.run_once(config, now=current, probe=self.healthy_probe(), notifier=notifier)
        for current in times[2:5]:
            MONITOR.run_once(config, now=current, probe=self.unhealthy_probe(), notifier=notifier)
        for current in times[5:]:
            MONITOR.run_once(config, now=current, probe=self.healthy_probe(), notifier=notifier)

        proof = self.read_proof()
        proof["monitor_host"] = "independent-exapi-monitor"
        for key in ("last_probe", "last_alert", "last_recovery"):
            proof[key]["monitor_host"] = proof["monitor_host"]
        config.proof_path.write_text(json.dumps(proof), encoding="utf-8")
        environment = dict(os.environ)
        environment.update(
            {
                "EXAPI_OFFHOST_MONITOR_PROOF_FILE": str(config.proof_path),
                "EXAPI_REQUIRE_MONITOR_RECOVERY": "true",
            }
        )
        for adapter in ("configure-external-readiness", "verify-alert-delivery"):
            with self.subTest(adapter=adapter):
                completed = subprocess.run(
                    [str(ADAPTER_DIR / adapter)],
                    cwd=ROOT,
                    env=environment,
                    text=True,
                    capture_output=True,
                    check=False,
                )
                self.assertEqual(0, completed.returncode, completed.stderr)
                self.assertTrue(json.loads(completed.stdout)["verified"] if adapter == "verify-alert-delivery" else json.loads(completed.stdout)["configured"])


if __name__ == "__main__":  # pragma: no cover
    unittest.main(verbosity=2)
