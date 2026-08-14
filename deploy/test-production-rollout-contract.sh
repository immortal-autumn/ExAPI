#!/usr/bin/env bash
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"
fail() { printf 'production rollout contract failed: %s\n' "$*" >&2; exit 1; }

for script in \
  deploy/ops/create-recovery-set.sh \
  deploy/ops/verify-logical-restore.sh \
  deploy/ops/verify-snapshot-restore.sh \
  deploy/ops/observe-rollout.sh \
  deploy/ops/publish-rollout-manifest.sh; do
  bash -n "$script" || fail "$script has invalid shell syntax"
done
bash -n deploy/ops/validate-immutable-compose.sh || fail 'immutable Compose validator has invalid shell syntax'
sh -n deploy/ops/with-migration-report-key.sh || fail 'migration report key wrapper has invalid POSIX shell syntax'

grep -Fq '/app/with-migration-report-key.sh /protected/exapi-migration-report.key' deploy/PRODUCTION_ROLLOUT.md || \
  fail 'offline cutover does not use the protected report-key wrapper'
! grep -Fq 'export EXAPI_MIGRATION_REPORT_KEY=' deploy/PRODUCTION_ROLLOUT.md || \
  fail 'offline cutover exports its report key into the operator shell'
grep -Fq -- '--backup-dir /app/data/backups' deploy/PRODUCTION_ROLLOUT.md || \
  fail 'offline cutover example omits its mandatory managed-backup choice'
grep -Fq -- '--no-managed-backups' deploy/PRODUCTION_ROLLOUT.md || \
  fail 'offline cutover omits the verified no-managed-backups alternative'
grep -Fq '(umask 077; set -C; openssl rand -hex 32 >"$key_file")' deploy/PRODUCTION_ROLLOUT.md || \
  fail 'offline cutover does not use no-clobber report-key generation'

mkdir -p "$ROOT_DIR/tmp"
secure_tmp=
if [[ -n "${EXAPI_CONTRACT_SECURE_TMP_DIR:-}" ]]; then
  mkdir -p "$EXAPI_CONTRACT_SECURE_TMP_DIR"
  secure_tmp=$EXAPI_CONTRACT_SECURE_TMP_DIR
else
  for candidate in "$ROOT_DIR/tmp" /dev/shm "${TMPDIR:-/tmp}" /tmp; do
    [[ -d "$candidate" && -w "$candidate" ]] || continue
    mode_probe=$(mktemp "$candidate/report-key-mode-probe.XXXXXX") || continue
    chmod 0600 "$mode_probe"
    mode=$(stat -c '%a' "$mode_probe" 2>/dev/null || stat -f '%Lp' "$mode_probe" 2>/dev/null || true)
    unlink "$mode_probe"
    if [[ "$mode" == 600 ]]; then
      secure_tmp=$candidate
      break
    fi
  done
fi
[[ -n "$secure_tmp" ]] || fail 'no permission-capable temporary directory; set EXAPI_CONTRACT_SECURE_TMP_DIR'
valid_report_key=$(mktemp "$secure_tmp/report-key-valid.XXXXXX")
malformed_report_key=$(mktemp "$secure_tmp/report-key-malformed.XXXXXX")
permissive_report_key=$(mktemp "$secure_tmp/report-key-permissive.XXXXXX")
trap 'unlink "$valid_report_key" 2>/dev/null || true; unlink "$malformed_report_key" 2>/dev/null || true; unlink "$permissive_report_key" 2>/dev/null || true' EXIT
printf '%064d\n' 0 >"$valid_report_key"
chmod 0600 "$valid_report_key"
sh deploy/ops/with-migration-report-key.sh "$valid_report_key" sh -c \
  'test "${#EXAPI_MIGRATION_REPORT_KEY}" -eq 64' || fail 'valid migration report key was rejected'
printf '%064d\ntrailing' 0 >"$malformed_report_key"
chmod 0600 "$malformed_report_key"
if sh deploy/ops/with-migration-report-key.sh "$malformed_report_key" true >/dev/null 2>&1; then
  fail 'migration report key wrapper accepted trailing data'
fi
printf '%064d\n' 0 >"$permissive_report_key"
chmod 0644 "$permissive_report_key"
if sh deploy/ops/with-migration-report-key.sh "$permissive_report_key" true >/dev/null 2>&1; then
  fail 'migration report key wrapper accepted permissive mode'
fi

for compose in deploy/docker-compose.yml deploy/docker-compose.local.yml; do
  grep -Fq '${POSTGRES_IMAGE:?Set POSTGRES_IMAGE to an immutable postgres@sha256:<digest> reference}' "$compose" || fail "$compose does not require a PostgreSQL digest"
  grep -Fq '${REDIS_IMAGE:?Set REDIS_IMAGE to an immutable redis@sha256:<digest> reference}' "$compose" || fail "$compose does not require a Redis digest"
  [[ "$(grep -Fc 'no-new-privileges:true' "$compose")" -eq 3 ]] || fail "$compose does not protect every service with no-new-privileges"
  grep -Fq '${EXAPI_STOP_GRACE_PERIOD:-50s}' "$compose" || fail "$compose can kill the app before its ordered shutdown budget"
done
grep -Fq 'no-new-privileges:true' deploy/docker-compose.standalone.yml || fail 'standalone app lacks no-new-privileges'
grep -Fq '${EXAPI_STOP_GRACE_PERIOD:-50s}' deploy/docker-compose.standalone.yml || fail 'standalone shutdown grace is too short'
grep -Fq 'internal: true' deploy/docker-compose.canary-restored.yml || fail 'restored-data canary network is not internal'
grep -Fq 'SUB2API_CANARY_CLASS=restored-production-data' deploy/docker-compose.canary-restored.yml || fail 'restored-data canary class is missing'
grep -Fq 'external: true' deploy/docker-compose.canary-restored.yml || fail 'restored-data canary does not require the verified external volume'
grep -Fq 'RESTORED_POSTGRES_DATABASE' deploy/docker-compose.canary-restored.yml || fail 'restored-data canary does not require the verified restored database'
[[ "$(grep -Fc 'docker compose --env-file /protected/exapi-canary-restored.env' deploy/PRODUCTION_ROLLOUT.md)" -eq 2 ]] || \
  fail 'restored-data canary config/up must use its protected env file'
grep -Fq 'database and secret recovery objects must have disjoint age recipients' deploy/ops/create-recovery-set.sh || \
  fail 'database and secrets age recipient sets are not required to be disjoint'
grep -Fq 'snapshot retention_until is shorter than RECOVERY_RETENTION_UNTIL' deploy/ops/create-recovery-set.sh || \
  fail 'snapshot retention can be shorter than requested'
grep -Fq 'cosign verify-attestation' deploy/ops/publish-rollout-manifest.sh || fail 'OCI provenance is not verified before publication'
grep -Fq 'oci-provenance-verification.json' deploy/ops/publish-rollout-manifest.sh || fail 'OCI provenance verification evidence is not uploaded'
grep -Fq 'attest-build-provenance@0f67c3f4856b2e3261c31976d6725780e5e4c373' .github/workflows/release.yml || fail 'SLSA provenance action is missing or mutable'
grep -Fq 'format: spdx-json' .github/workflows/release.yml || fail 'SPDX JSON generation is missing'

mkdir -p tmp
hash_file=$(mktemp "$ROOT_DIR/tmp/stream-hash.XXXXXX")
stream_file=$(mktemp "$ROOT_DIR/tmp/stream-copy.XXXXXX")
trap 'unlink "$valid_report_key" 2>/dev/null || true; unlink "$malformed_report_key" 2>/dev/null || true; unlink "$permissive_report_key" 2>/dev/null || true; unlink "$hash_file" 2>/dev/null || true; unlink "$stream_file" 2>/dev/null || true' EXIT
printf abc | python3 deploy/ops/stream_hash.py "$hash_file" >"$stream_file"
[[ "$(<"$stream_file")" == abc ]] || fail 'stream hashing changed backup bytes'
[[ "$(<"$hash_file")" == sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad ]] || fail 'stream hashing returned the wrong digest'
if python3 tools/check_rollout_manifest.py deploy/ops/rollout-manifest.example.json >/dev/null 2>&1; then
  fail 'unverified rollout template passed validation'
fi

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  rendered=$(
    COMPOSE_PROJECT_NAME=exapi-canary-restored \
    EXAPI_IMAGE='ghcr.io/example/exapi@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' \
    POSTGRES_IMAGE='postgres@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb' \
    REDIS_IMAGE='redis@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc' \
    POSTGRES_PASSWORD=test REDIS_PASSWORD=test \
    RESTORED_POSTGRES_VOLUME=verified-logical-restore-volume \
    RESTORED_POSTGRES_DATABASE=exapi_restore \
    SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=data \
    SUB2API_DATA_ENCRYPTION_KEYS_JSON='{"data":"QUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUE="}' \
    SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=digest \
    SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON='{"digest":"QkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkI="}' \
    docker compose --env-file /dev/null -f deploy/docker-compose.yml -f deploy/docker-compose.canary-restored.yml config --format json
  )
  COMPOSE_CONFIG_JSON="$rendered" python3 - <<'PY'
import json, os
d = json.loads(os.environ["COMPOSE_CONFIG_JSON"])
network = d["networks"]["sub2api-network"]
if network.get("internal") is not True:
    raise SystemExit("restored canary network did not render internal")
restored_volume = d["volumes"].get("restored_postgres_data", {})
if restored_volume.get("external") is not True or restored_volume.get("name") != "verified-logical-restore-volume":
    raise SystemExit("restored canary did not render the verified external PostgreSQL volume")
postgres_mounts = [v for v in d["services"]["postgres"].get("volumes", []) if v.get("target") == "/var/lib/postgresql/data"]
if len(postgres_mounts) != 1 or postgres_mounts[0].get("source") != "restored_postgres_data":
    raise SystemExit("restored canary PostgreSQL data mount is not the verified external volume")
for service in ("sub2api", "postgres"):
    if d["services"][service]["environment"].get("DATABASE_DBNAME" if service == "sub2api" else "POSTGRES_DB") != "exapi_restore":
        raise SystemExit(f"{service} does not use the verified restored database")
for name, service in d["services"].items():
    if set(service.get("networks", {})) != {"sub2api-network"}:
        raise SystemExit(f"{name} has a second egress-capable network")
PY
  EXAPI_IMAGE='ghcr.io/example/exapi@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' \
  POSTGRES_IMAGE='postgres@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb' \
  REDIS_IMAGE='redis@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc' \
  POSTGRES_PASSWORD=test REDIS_PASSWORD=test \
  RESTORED_POSTGRES_VOLUME=verified-logical-restore-volume \
  RESTORED_POSTGRES_DATABASE=exapi_restore \
  SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=data \
  SUB2API_DATA_ENCRYPTION_KEYS_JSON='{"data":"QUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUE="}' \
  SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=digest \
  SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON='{"digest":"QkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkI="}' \
  COMPOSE_ENV_FILE=/dev/null REQUIRE_INTERNAL_NETWORK=true deploy/ops/validate-immutable-compose.sh \
    -f deploy/docker-compose.yml -f deploy/docker-compose.canary-restored.yml >/dev/null
  if EXAPI_IMAGE='ghcr.io/example/exapi:latest' \
    POSTGRES_IMAGE='postgres@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb' \
    REDIS_IMAGE='redis@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc' \
    POSTGRES_PASSWORD=test REDIS_PASSWORD=test \
    SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=data \
    SUB2API_DATA_ENCRYPTION_KEYS_JSON='{"data":"QUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUE="}' \
    SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=digest \
    SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON='{"digest":"QkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkI="}' \
    deploy/ops/validate-immutable-compose.sh -f deploy/docker-compose.yml >/dev/null 2>&1; then
    fail 'immutable Compose validator accepted a mutable application tag'
  fi
fi

python3 - <<'PY'
from copy import deepcopy
from tools.check_rollout_manifest import ContractError, validate

digest = "sha256:" + "a" * 64
commit = "b" * 40
observation = {
    "duration_minutes": 60, "readiness_interval_seconds": 30, "readiness_checks": 120,
    "readiness_failures": 0, "restarts": 0, "new_p0_p1": 0,
    "request_count": 100, "unexpected_5xx": 0, "error_rate": 0,
    "latency_sample_count": 100, "p95_ms": 110, "baseline_p95_ms": 100,
    "image_digest": digest,
}
manifest = {
    "schema_version": 1, "rollout_id": "rollout-test", "generated_at": "2026-08-05T12:00:00Z",
    "source": {"commit": commit, "repository": "immortal-autumn/Sub2API2Personal", "workflow": ".github/workflows/release.yml"},
    "artifact": {
        "image": "ghcr.io/immortal-autumn/sub2api2personal@" + digest,
        "manifest_digest": digest,
        "platform_digests": [{"platform": "linux/amd64", "digest": "sha256:" + "c" * 64}],
        "oci_labels": {"revision": commit, "source": "https://github.com/immortal-autumn/Sub2API2Personal", "version": "1.0.0"},
        "sbom": {"format": "SPDX-JSON", "sha256": "sha256:" + "d" * 64},
        "provenance": {"verified": True, "issuer": "https://token.actions.githubusercontent.com", "repository": "immortal-autumn/Sub2API2Personal", "workflow": ".github/workflows/release.yml", "subject_digest": digest},
    },
    "recovery": {
        "logical": {
            "backup": {"object_uri": "s3://backup/db", "version_id": "v1", "sha256": "sha256:" + "e" * 64, "encrypted": True, "retention_until": "2099-09-05T12:00:00Z"},
            "disposable_target": "logical-test", "volume": "logical-test-data", "database": "exapi_restore",
            "restored_at": "2026-08-05T12:10:00Z", "verified": True, "network_mode": "none", "backup_sha256": "sha256:" + "e" * 64,
        },
        "snapshot": {"provider": "oci", "snapshot_id": "snap-1", "retention_until": "2099-09-05T12:00:00Z", "writer_quiesced": True, "checkpoint_completed": True, "disposable_target": "snapshot-test", "restored_at": "2026-08-05T12:15:00Z", "verified": True, "egress_denied": True, "integrity_verified": True, "decryption_verified": True},
        "secrets": {"object_uri": "s3://keyroots/env", "version_id": "v2", "sha256": "sha256:" + "f" * 64, "encrypted": True, "retention_until": "2099-09-05T12:00:00Z", "key_ids": ["data-v1", "digest-v1", "backup-v1"]},
        "independent_restore_paths": True,
    },
    "security": {"private_live_restore_disabled": True, "no_new_privileges": True, "immutable_runtime_images": True},
    "monitoring": {"external_readiness_configured": True, "provider": "oci-health-check", "alert_probe": {"delivered": True, "delivered_at": "2026-08-05T12:20:00Z", "delivery_id": "alert-1"}},
    "canaries": {
        "restored_data": {
            **observation, "duration_minutes": 30, "readiness_checks": 60,
            "production_data": True, "egress_denied": True, "integrity_verified": True,
            "decryption_verified": True,
            "source": {
                "logical_restore_target": "logical-test", "postgres_volume": "logical-test-data",
                "database": "exapi_restore", "backup_sha256": "sha256:" + "e" * 64,
            },
        },
        "synthetic_provider": {**observation, "duration_minutes": 30, "readiness_checks": 60, "production_data": False, "canary_only_credentials": True, "egress_allowlist_verified": True, "provider_smoke_verified": True},
    },
    "production": observation,
    "migration": {"version": "001", "ciphertext_only_cutover": False},
    "rollback": {"mode": "compatible-image-or-snapshot"},
}
validate(manifest)
for path, value in (
    (("canaries", "restored_data", "egress_denied"), False),
    (("canaries", "synthetic_provider", "duration_minutes"), 29),
    (("production", "error_rate"), 0.01),
    (("recovery", "logical", "backup", "encrypted"), False),
    (("security", "private_live_restore_disabled"), False),
    (("source", "repository"), "someone/example"),
    (("source", "workflow"), ".github/workflows/other.yml"),
    (("artifact", "image"), "ghcr.io/someone/example@" + digest),
    (("artifact", "provenance", "issuer"), "https://example.invalid"),
    (("recovery", "logical", "backup", "retention_until"), "2020-01-01T00:00:00Z"),
    (("recovery", "snapshot", "retention_until"), "2020-01-01T00:00:00Z"),
    (("recovery", "secrets", "retention_until"), "2020-01-01T00:00:00Z"),
    (("canaries", "restored_data", "source", "logical_restore_target"), "other-target"),
    (("canaries", "restored_data", "source", "postgres_volume"), "other-volume"),
    (("canaries", "restored_data", "source", "database"), "other-database"),
    (("canaries", "restored_data", "source", "backup_sha256"), "sha256:" + "9" * 64),
):
    broken = deepcopy(manifest)
    target = broken
    for key in path[:-1]: target = target[key]
    target[path[-1]] = value
    try: validate(broken)
    except ContractError: pass
    else: raise SystemExit(f"manifest validator accepted unsafe mutation {path}")
broken = deepcopy(manifest)
broken["recovery"]["snapshot"]["retention_until"] = "2099-08-20T12:00:00Z"
try: validate(broken)
except ContractError: pass
else: raise SystemExit("manifest validator accepted snapshot retention shorter than requested logical retention")
PY

unlink "$hash_file"
unlink "$stream_file"
unlink "$valid_report_key"
unlink "$malformed_report_key"
unlink "$permissive_report_key"
trap - EXIT
printf 'production rollout contracts: pass\n'
