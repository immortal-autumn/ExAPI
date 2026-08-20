#!/usr/bin/env python3
"""Validate the fail-closed ExAPI production rollout evidence contract."""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


SHA256 = re.compile(r"^sha256:[0-9a-f]{64}$")
COMMIT = re.compile(r"^[0-9a-f]{40}$")
CANONICAL_REPOSITORY = "immortal-autumn/ExAPI"
CANONICAL_WORKFLOW = ".github/workflows/release.yml"
CANONICAL_OIDC_ISSUER = "https://token.actions.githubusercontent.com"
CANONICAL_IMAGE_REPOSITORY = "ghcr.io/immortal-autumn/sub2api2personal"
CANONICAL_WORKFLOW_IDENTITY_REGEXP = (
    r"^https://github\.com/immortal-autumn/ExAPI/\.github/workflows/release\.yml@"
)


class ContractError(ValueError):
    pass


def fail(message: str) -> None:
    raise ContractError(message)


def obj(value: Any, path: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        fail(f"{path} must be an object")
    return value


def field(parent: dict[str, Any], name: str, path: str) -> Any:
    if name not in parent:
        fail(f"{path}.{name} is required")
    return parent[name]


def text(parent: dict[str, Any], name: str, path: str) -> str:
    value = field(parent, name, path)
    if not isinstance(value, str) or not value.strip():
        fail(f"{path}.{name} must be a non-empty string")
    return value


def true(parent: dict[str, Any], name: str, path: str) -> None:
    if field(parent, name, path) is not True:
        fail(f"{path}.{name} must be true")


def number(parent: dict[str, Any], name: str, path: str) -> float:
    value = field(parent, name, path)
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        fail(f"{path}.{name} must be numeric")
    return float(value)


def timestamp(parent: dict[str, Any], name: str, path: str) -> datetime:
    raw = text(parent, name, path)
    if not raw.endswith("Z"):
        fail(f"{path}.{name} must be RFC3339 UTC with a Z suffix")
    try:
        parsed = datetime.fromisoformat(raw[:-1] + "+00:00")
    except ValueError as exc:
        raise ContractError(f"{path}.{name} is not RFC3339: {exc}") from exc
    if parsed.tzinfo != timezone.utc:
        fail(f"{path}.{name} must be UTC")
    return parsed


def future_timestamp(parent: dict[str, Any], name: str, path: str) -> datetime:
    parsed = timestamp(parent, name, path)
    if parsed <= datetime.now(timezone.utc):
        fail(f"{path}.{name} must be in the future")
    return parsed


def digest(parent: dict[str, Any], name: str, path: str) -> str:
    value = text(parent, name, path)
    if not SHA256.fullmatch(value):
        fail(f"{path}.{name} must be a lowercase sha256 digest")
    return value


def encrypted_object(value: Any, path: str) -> None:
    item = obj(value, path)
    uri = text(item, "object_uri", path)
    if not uri.startswith("s3://"):
        fail(f"{path}.object_uri must be an off-host s3:// URI")
    text(item, "version_id", path)
    digest(item, "sha256", path)
    true(item, "encrypted", path)
    future_timestamp(item, "retention_until", path)


def restore_evidence(value: Any, path: str, snapshot: bool = False) -> None:
    item = obj(value, path)
    text(item, "disposable_target", path)
    timestamp(item, "restored_at", path)
    true(item, "verified", path)
    if snapshot:
        text(item, "provider", path)
        text(item, "snapshot_id", path)
        future_timestamp(item, "retention_until", path)
        true(item, "writer_quiesced", path)
        true(item, "checkpoint_completed", path)


def observation(value: Any, path: str, minimum_minutes: float) -> None:
    item = obj(value, path)
    duration = number(item, "duration_minutes", path)
    if duration < minimum_minutes:
        fail(f"{path}.duration_minutes must be at least {minimum_minutes:g}")
    interval = number(item, "readiness_interval_seconds", path)
    if interval > 30 or interval <= 0:
        fail(f"{path}.readiness_interval_seconds must be in (0, 30]")
    checks = number(item, "readiness_checks", path)
    if checks < int(duration * 60 // interval):
        fail(f"{path}.readiness_checks does not cover the observation window")
    if number(item, "readiness_failures", path) != 0:
        fail(f"{path}.readiness_failures must be zero")
    if number(item, "restarts", path) != 0:
        fail(f"{path}.restarts must be zero")
    if number(item, "new_p0_p1", path) != 0:
        fail(f"{path}.new_p0_p1 must be zero")
    for metric in ("new_p0_p1", "readiness_failures", "restarts"):
        if number(item, metric, path) < 0:
            fail(f"{path}.{metric} must be non-negative")
    requests = number(item, "request_count", path)
    unexpected_5xx = number(item, "unexpected_5xx", path)
    error_rate = number(item, "error_rate", path)
    if requests < 0 or unexpected_5xx < 0 or error_rate < 0 or error_rate > 1:
        fail(f"{path} contains an invalid request/error metric")
    if requests >= 100:
        if error_rate >= 0.01:
            fail(f"{path}.error_rate must be below 0.01 with at least 100 requests")
    elif unexpected_5xx != 0:
        fail(f"{path}.unexpected_5xx must be zero with fewer than 100 requests")
    sample_count = number(item, "latency_sample_count", path)
    if sample_count < 0:
        fail(f"{path}.latency_sample_count must be non-negative")
    p95 = number(item, "p95_ms", path)
    baseline = number(item, "baseline_p95_ms", path)
    if p95 < 0 or baseline < 0:
        fail(f"{path} contains an invalid latency metric")
    if sample_count >= 100:
        if baseline <= 0 or p95 > baseline * 1.2:
            fail(f"{path}.p95_ms exceeds the baseline by more than 20%")


def validate(document: Any) -> None:
    root = obj(document, "manifest")
    if field(root, "schema_version", "manifest") != 1:
        fail("manifest.schema_version must equal 1")
    text(root, "rollout_id", "manifest")
    timestamp(root, "generated_at", "manifest")

    source = obj(field(root, "source", "manifest"), "manifest.source")
    commit = text(source, "commit", "manifest.source")
    if not COMMIT.fullmatch(commit):
        fail("manifest.source.commit must be a lowercase 40-character Git commit")
    source_repository = text(source, "repository", "manifest.source")
    source_workflow = text(source, "workflow", "manifest.source")
    if source_repository != CANONICAL_REPOSITORY:
        fail(f"manifest.source.repository must equal {CANONICAL_REPOSITORY!r}")
    if source_workflow != CANONICAL_WORKFLOW:
        fail(f"manifest.source.workflow must equal {CANONICAL_WORKFLOW!r}")

    artifact = obj(field(root, "artifact", "manifest"), "manifest.artifact")
    image = text(artifact, "image", "manifest.artifact")
    image_digest = digest(artifact, "manifest_digest", "manifest.artifact")
    if image != CANONICAL_IMAGE_REPOSITORY + "@" + image_digest:
        fail("manifest.artifact.image must be the canonical OCI image at manifest.artifact.manifest_digest")
    platforms = field(artifact, "platform_digests", "manifest.artifact")
    if not isinstance(platforms, list) or not platforms:
        fail("manifest.artifact.platform_digests must be a non-empty list")
    for index, platform in enumerate(platforms):
        item = obj(platform, f"manifest.artifact.platform_digests[{index}]")
        text(item, "platform", f"manifest.artifact.platform_digests[{index}]")
        digest(item, "digest", f"manifest.artifact.platform_digests[{index}]")
    labels = obj(field(artifact, "oci_labels", "manifest.artifact"), "manifest.artifact.oci_labels")
    if text(labels, "revision", "manifest.artifact.oci_labels") != commit:
        fail("OCI revision label must equal source.commit")
    if text(labels, "source", "manifest.artifact.oci_labels") != "https://github.com/" + source_repository:
        fail("OCI source label must identify source.repository")
    text(labels, "version", "manifest.artifact.oci_labels")
    sbom = obj(field(artifact, "sbom", "manifest.artifact"), "manifest.artifact.sbom")
    if text(sbom, "format", "manifest.artifact.sbom") != "SPDX-JSON":
        fail("manifest.artifact.sbom.format must equal SPDX-JSON")
    digest(sbom, "sha256", "manifest.artifact.sbom")
    provenance = obj(field(artifact, "provenance", "manifest.artifact"), "manifest.artifact.provenance")
    true(provenance, "verified", "manifest.artifact.provenance")
    if text(provenance, "issuer", "manifest.artifact.provenance") != CANONICAL_OIDC_ISSUER:
        fail(f"provenance issuer must equal {CANONICAL_OIDC_ISSUER!r}")
    if text(provenance, "repository", "manifest.artifact.provenance") != source_repository:
        fail("provenance repository must equal source.repository")
    if text(provenance, "workflow", "manifest.artifact.provenance") != source_workflow:
        fail("provenance workflow must equal source.workflow")
    if digest(provenance, "subject_digest", "manifest.artifact.provenance") != image_digest:
        fail("provenance subject digest must equal the image manifest digest")

    recovery = obj(field(root, "recovery", "manifest"), "manifest.recovery")
    logical = obj(field(recovery, "logical", "manifest.recovery"), "manifest.recovery.logical")
    backup = obj(field(logical, "backup", "manifest.recovery.logical"), "manifest.recovery.logical.backup")
    encrypted_object(backup, "manifest.recovery.logical.backup")
    restore_evidence(logical, "manifest.recovery.logical")
    if text(logical, "network_mode", "manifest.recovery.logical") != "none":
        fail("logical restore target must have network_mode='none'")
    if digest(logical, "backup_sha256", "manifest.recovery.logical") != digest(backup, "sha256", "manifest.recovery.logical.backup"):
        fail("logical restore checksum must equal the encrypted backup checksum")
    logical_volume = text(logical, "volume", "manifest.recovery.logical")
    logical_database = text(logical, "database", "manifest.recovery.logical")
    snapshot = obj(field(recovery, "snapshot", "manifest.recovery"), "manifest.recovery.snapshot")
    restore_evidence(snapshot, "manifest.recovery.snapshot", snapshot=True)
    if timestamp(snapshot, "retention_until", "manifest.recovery.snapshot") < timestamp(backup, "retention_until", "manifest.recovery.logical.backup"):
        fail("snapshot retention must not be shorter than logical backup retention")
    for proof in ("egress_denied", "integrity_verified", "decryption_verified"):
        true(snapshot, proof, "manifest.recovery.snapshot")
    if text(logical, "disposable_target", "manifest.recovery.logical") == text(snapshot, "disposable_target", "manifest.recovery.snapshot"):
        fail("logical and snapshot restores must use independent disposable targets")
    secrets = obj(field(recovery, "secrets", "manifest.recovery"), "manifest.recovery.secrets")
    encrypted_object(secrets, "manifest.recovery.secrets")
    key_ids = field(secrets, "key_ids", "manifest.recovery.secrets")
    if not isinstance(key_ids, list) or len(key_ids) < 3 or not all(isinstance(item, str) and item for item in key_ids):
        fail("manifest.recovery.secrets.key_ids must name all independently protected keyrings")
    if len(set(key_ids)) != len(key_ids):
        fail("manifest.recovery.secrets.key_ids must be distinct")
    if text(backup, "object_uri", "manifest.recovery.logical.backup") == text(secrets, "object_uri", "manifest.recovery.secrets"):
        fail("database and keyroot recovery objects must use separate locations")
    true(recovery, "independent_restore_paths", "manifest.recovery")

    security = obj(field(root, "security", "manifest"), "manifest.security")
    true(security, "private_live_restore_disabled", "manifest.security")
    true(security, "no_new_privileges", "manifest.security")
    true(security, "immutable_runtime_images", "manifest.security")

    monitoring = obj(field(root, "monitoring", "manifest"), "manifest.monitoring")
    true(monitoring, "external_readiness_configured", "manifest.monitoring")
    text(monitoring, "provider", "manifest.monitoring")
    alert = obj(field(monitoring, "alert_probe", "manifest.monitoring"), "manifest.monitoring.alert_probe")
    true(alert, "delivered", "manifest.monitoring.alert_probe")
    timestamp(alert, "delivered_at", "manifest.monitoring.alert_probe")
    text(alert, "delivery_id", "manifest.monitoring.alert_probe")

    canaries = obj(field(root, "canaries", "manifest"), "manifest.canaries")
    restored = obj(field(canaries, "restored_data", "manifest.canaries"), "manifest.canaries.restored_data")
    observation(restored, "manifest.canaries.restored_data", 30)
    true(restored, "production_data", "manifest.canaries.restored_data")
    true(restored, "egress_denied", "manifest.canaries.restored_data")
    true(restored, "integrity_verified", "manifest.canaries.restored_data")
    true(restored, "decryption_verified", "manifest.canaries.restored_data")
    if digest(restored, "image_digest", "manifest.canaries.restored_data") != image_digest:
        fail("restored-data canary must use the release digest")
    restored_source = obj(field(restored, "source", "manifest.canaries.restored_data"), "manifest.canaries.restored_data.source")
    if text(restored_source, "logical_restore_target", "manifest.canaries.restored_data.source") != text(logical, "disposable_target", "manifest.recovery.logical"):
        fail("restored-data canary source target must equal the verified logical restore target")
    if text(restored_source, "postgres_volume", "manifest.canaries.restored_data.source") != logical_volume:
        fail("restored-data canary source volume must equal the verified logical restore volume")
    if text(restored_source, "database", "manifest.canaries.restored_data.source") != logical_database:
        fail("restored-data canary source database must equal the verified logical restore database")
    if digest(restored_source, "backup_sha256", "manifest.canaries.restored_data.source") != digest(logical, "backup_sha256", "manifest.recovery.logical"):
        fail("restored-data canary source checksum must equal the verified logical restore checksum")

    synthetic = obj(field(canaries, "synthetic_provider", "manifest.canaries"), "manifest.canaries.synthetic_provider")
    observation(synthetic, "manifest.canaries.synthetic_provider", 30)
    if field(synthetic, "production_data", "manifest.canaries.synthetic_provider") is not False:
        fail("synthetic provider canary production_data must be false")
    true(synthetic, "canary_only_credentials", "manifest.canaries.synthetic_provider")
    true(synthetic, "egress_allowlist_verified", "manifest.canaries.synthetic_provider")
    true(synthetic, "provider_smoke_verified", "manifest.canaries.synthetic_provider")
    if digest(synthetic, "image_digest", "manifest.canaries.synthetic_provider") != image_digest:
        fail("synthetic provider canary must use the release digest")

    production = obj(field(root, "production", "manifest"), "manifest.production")
    observation(production, "manifest.production", 60)
    if digest(production, "image_digest", "manifest.production") != image_digest:
        fail("production must use the canary-tested release digest")

    migration = obj(field(root, "migration", "manifest"), "manifest.migration")
    text(migration, "version", "manifest.migration")
    cutover = field(migration, "ciphertext_only_cutover", "manifest.migration")
    if not isinstance(cutover, bool):
        fail("manifest.migration.ciphertext_only_cutover must be boolean")
    rollback = obj(field(root, "rollback", "manifest"), "manifest.rollback")
    mode = text(rollback, "mode", "manifest.rollback")
    expected = "snapshot-and-keyroots" if cutover else "compatible-image-or-snapshot"
    if mode != expected:
        fail(f"manifest.rollback.mode must equal {expected!r}")
    if cutover:
        text(rollback, "snapshot_id", "manifest.rollback")
        digest(rollback, "previous_image_digest", "manifest.rollback")
        roots = field(rollback, "previous_key_ids", "manifest.rollback")
        if not isinstance(roots, list) or not roots:
            fail("post-cutover rollback requires previous_key_ids")


def verify_signature(args: argparse.Namespace) -> None:
    if not args.bundle:
        return
    command = ["cosign", "verify-blob", "--bundle", str(args.bundle)]
    if args.key:
        command += ["--key", args.key]
    else:
        if not args.certificate_identity_regexp or not args.certificate_oidc_issuer:
            fail("keyless signature verification requires certificate identity and OIDC issuer")
        if args.certificate_identity_regexp != CANONICAL_WORKFLOW_IDENTITY_REGEXP:
            fail("keyless signature verification must require the canonical release workflow identity")
        if args.certificate_oidc_issuer != CANONICAL_OIDC_ISSUER:
            fail("keyless signature verification must require the canonical GitHub OIDC issuer")
        command += [
            "--certificate-identity-regexp", args.certificate_identity_regexp,
            "--certificate-oidc-issuer", args.certificate_oidc_issuer,
        ]
    command.append(str(args.manifest))
    try:
        subprocess.run(command, check=True)
    except FileNotFoundError as exc:
        raise ContractError("cosign is required to verify the rollout signature") from exc
    except subprocess.CalledProcessError as exc:
        raise ContractError(f"rollout manifest signature verification failed: {exc}") from exc


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--bundle", type=Path)
    parser.add_argument("--key")
    parser.add_argument("--certificate-identity-regexp")
    parser.add_argument("--certificate-oidc-issuer")
    args = parser.parse_args()
    if args.key and (args.certificate_identity_regexp or args.certificate_oidc_issuer):
        parser.error("--key cannot be combined with keyless certificate options")
    try:
        document = json.loads(args.manifest.read_text(encoding="utf-8"))
        validate(document)
        verify_signature(args)
    except (OSError, json.JSONDecodeError, ContractError) as exc:
        print(f"rollout manifest rejected: {exc}", file=sys.stderr)
        return 1
    print("ExAPI rollout manifest accepted.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
