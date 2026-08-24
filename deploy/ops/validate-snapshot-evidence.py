#!/usr/bin/env python3
"""Validate snapshot-adapter evidence against the quiesced database source."""

import json
import os
import sys
from datetime import datetime, timedelta, timezone


def fail(message: str) -> None:
    raise SystemExit(message)


if len(sys.argv) != 2:
    fail("usage: validate-snapshot-evidence.py <snapshot-evidence.json>")

with open(sys.argv[1], encoding="utf-8") as handle:
    data = json.load(handle)

if data.get("schema_version") != 1:
    fail("snapshot adapter schema_version must be 1")
if data.get("rollout_id") != os.environ["ROLLOUT_ID"]:
    fail("snapshot adapter rollout_id mismatch")
if data.get("writer_quiesced") is not True or data.get("checkpoint_completed") is not True:
    fail("snapshot adapter omitted quiescence/checkpoint proof")

expected_source = {
    "container_id": os.environ["SOURCE_CONTAINER_ID"],
    "image_id": os.environ["SOURCE_IMAGE_ID"],
    "mounts_sha256": os.environ["SOURCE_MOUNTS_SHA256"],
}
if data.get("source") != expected_source:
    fail("snapshot adapter source identity mismatch")

for key in ("provider", "snapshot_id", "target", "created_at", "retention_until"):
    if not isinstance(data.get(key), str) or not data[key].strip():
        fail(f"snapshot adapter omitted {key}")

parsed: dict[str, datetime] = {}
for key in ("created_at", "retention_until"):
    if not data[key].endswith("Z"):
        fail(f"snapshot {key} is not RFC3339 UTC")
    parsed[key] = datetime.fromisoformat(data[key][:-1] + "+00:00")

checkpoint_raw = os.environ["CHECKPOINT_AT"]
if data.get("checkpoint_at") != checkpoint_raw:
    fail("snapshot adapter checkpoint_at mismatch")
checkpoint = datetime.fromisoformat(checkpoint_raw[:-1] + "+00:00")
requested_raw = os.environ["RECOVERY_RETENTION_UNTIL"]
requested = datetime.fromisoformat(requested_raw[:-1] + "+00:00")
now = datetime.now(timezone.utc)

if parsed["created_at"] < checkpoint or parsed["created_at"] > now + timedelta(minutes=2):
    fail("snapshot created_at is stale or in the future")
if parsed["retention_until"] <= now:
    fail("snapshot retention_until must be in the future")
if parsed["retention_until"] < requested:
    fail("snapshot retention_until is shorter than RECOVERY_RETENTION_UNTIL")
