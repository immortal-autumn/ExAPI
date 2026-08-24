#!/usr/bin/env python3

import copy
import json
import os
import subprocess
import sys
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path


VALIDATOR = Path(__file__).with_name("validate-snapshot-evidence.py")


class SnapshotEvidenceValidationTest(unittest.TestCase):
    def setUp(self) -> None:
        now = datetime.now(timezone.utc).replace(microsecond=0)
        self.checkpoint = now - timedelta(seconds=2)
        self.retention = now + timedelta(days=30)
        self.env = os.environ.copy()
        self.env.update(
            {
                "ROLLOUT_ID": "rollout-test",
                "CHECKPOINT_AT": self._format(self.checkpoint),
                "RECOVERY_RETENTION_UNTIL": self._format(self.retention),
                "SOURCE_CONTAINER_ID": "container-id",
                "SOURCE_IMAGE_ID": "sha256:image-id",
                "SOURCE_MOUNTS_SHA256": "a" * 64,
            }
        )
        self.evidence = {
            "schema_version": 1,
            "rollout_id": "rollout-test",
            "writer_quiesced": True,
            "checkpoint_completed": True,
            "checkpoint_at": self._format(self.checkpoint),
            "source": {
                "container_id": "container-id",
                "image_id": "sha256:image-id",
                "mounts_sha256": "a" * 64,
            },
            "provider": "synthetic",
            "snapshot_id": "snapshot-1",
            "target": "volume-1",
            "created_at": self._format(self.checkpoint + timedelta(seconds=1)),
            "retention_until": self._format(self.retention),
        }

    @staticmethod
    def _format(value: datetime) -> str:
        return value.isoformat().replace("+00:00", "Z")

    def run_validator(self, evidence: dict) -> subprocess.CompletedProcess[str]:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "evidence.json"
            path.write_text(json.dumps(evidence), encoding="utf-8")
            return subprocess.run(
                [sys.executable, str(VALIDATOR), str(path)],
                env=self.env,
                text=True,
                capture_output=True,
                check=False,
            )

    def test_accepts_evidence_bound_to_current_source(self) -> None:
        result = self.run_validator(self.evidence)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_rejects_wrong_rollout(self) -> None:
        evidence = copy.deepcopy(self.evidence)
        evidence["rollout_id"] = "stale-rollout"
        result = self.run_validator(evidence)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("rollout_id mismatch", result.stderr)

    def test_rejects_wrong_source(self) -> None:
        evidence = copy.deepcopy(self.evidence)
        evidence["source"]["container_id"] = "other-container"
        result = self.run_validator(evidence)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("source identity mismatch", result.stderr)

    def test_rejects_snapshot_created_before_checkpoint(self) -> None:
        evidence = copy.deepcopy(self.evidence)
        evidence["created_at"] = self._format(self.checkpoint - timedelta(seconds=1))
        result = self.run_validator(evidence)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("created_at is stale", result.stderr)

    def test_rejects_missing_quiescence_proof(self) -> None:
        evidence = copy.deepcopy(self.evidence)
        evidence["writer_quiesced"] = False
        result = self.run_validator(evidence)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("quiescence/checkpoint proof", result.stderr)


if __name__ == "__main__":
    unittest.main()
