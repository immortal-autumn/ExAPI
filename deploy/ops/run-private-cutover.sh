#!/usr/bin/env bash
set -euo pipefail

# Run the destructive private-only cutover from a reviewed image while the
# online application is stopped. This script deliberately does not restart
# the application: the operator must inspect the archive evidence and start
# the exact image explicitly after the evidence gate passes.

umask 077

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT_DIR"

die() { printf 'private cutover orchestration failed: %s\n' "$*" >&2; exit 1; }
require_env() { [[ -n "${!1:-}" ]] || die "$1 is required"; }
require_command() { command -v "$1" >/dev/null 2>&1 || die "$1 is required"; }

for command_name in docker python3; do require_command "$command_name"; done
docker compose version >/dev/null 2>&1 || die 'Docker Compose v2 is required'

for name in EXAPI_IMAGE COMPOSE_ENV_FILE EXAPI_MIGRATION_REPORT_KEY_FILE \
  EXAPI_CONFIRMATION EXAPI_REVIEWED_COMMIT EXAPI_REPORT_ARCHIVE_COMMAND \
  EXAPI_REPORT_ARCHIVE_VERIFY_COMMAND EXAPI_MAINTENANCE_VERIFY_COMMAND EXAPI_CONTROL_BIND_HOST \
  EXAPI_CONTROL_LISTEN_ADDR EXAPI_CONTROL_HOSTS EXAPI_OPERATOR_PEER_IPS \
  EXAPI_WIREGUARD_INTERFACE EXAPI_BATCH_CLEANUP_VERIFY_COMMAND; do
  require_env "$name"
done

[[ "$EXAPI_IMAGE" =~ @sha256:[0-9a-f]{64}$ ]] || die 'EXAPI_IMAGE must be immutable by digest'
[[ "$EXAPI_REVIEWED_COMMIT" =~ ^[0-9a-f]{40}$ ]] || die 'EXAPI_REVIEWED_COMMIT must be a full lowercase Git commit'
[[ -r "$COMPOSE_ENV_FILE" ]] || die 'COMPOSE_ENV_FILE must be readable'
[[ "$EXAPI_MIGRATION_REPORT_KEY_FILE" = /* ]] || \
  die 'EXAPI_MIGRATION_REPORT_KEY_FILE must be an absolute path'
[[ -x "$EXAPI_REPORT_ARCHIVE_COMMAND" ]] || die 'EXAPI_REPORT_ARCHIVE_COMMAND must be executable'
[[ -x "$EXAPI_REPORT_ARCHIVE_VERIFY_COMMAND" ]] || die 'EXAPI_REPORT_ARCHIVE_VERIFY_COMMAND must be executable'
[[ -x "$EXAPI_MAINTENANCE_VERIFY_COMMAND" ]] || die 'EXAPI_MAINTENANCE_VERIFY_COMMAND must be executable'
[[ -x "$EXAPI_BATCH_CLEANUP_VERIFY_COMMAND" ]] || die 'EXAPI_BATCH_CLEANUP_VERIFY_COMMAND must be executable'
resume=${EXAPI_RESUME_PRIVATE_CUTOVER:-false}
[[ "$resume" == true || "$resume" == false ]] || die 'EXAPI_RESUME_PRIVATE_CUTOVER must be true or false'
if [[ "$resume" == true && -z "${EXAPI_ROLLOUT_ID:-}" ]]; then
  die 'EXAPI_ROLLOUT_ID is required when EXAPI_RESUME_PRIVATE_CUTOVER=true'
fi
CONTROL_BIND_HOST="$EXAPI_CONTROL_BIND_HOST" CONTROL_HOSTS="$EXAPI_CONTROL_HOSTS" \
OPERATOR_PEER_IPS="$EXAPI_OPERATOR_PEER_IPS" python3 - <<'PY'
import ipaddress
import os

bind = ipaddress.ip_address(os.environ["CONTROL_BIND_HOST"])
if bind.is_loopback or bind.is_unspecified:
    raise SystemExit("EXAPI_CONTROL_BIND_HOST must be a private/WireGuard address")
hosts = {value.strip() for value in os.environ["CONTROL_HOSTS"].split(",") if value.strip()}
if str(bind) not in hosts:
    raise SystemExit("EXAPI_CONTROL_HOSTS must include EXAPI_CONTROL_BIND_HOST")
peers = [value.strip() for value in os.environ["OPERATOR_PEER_IPS"].split(",") if value.strip()]
if not peers:
    raise SystemExit("EXAPI_OPERATOR_PEER_IPS must contain a WireGuard peer")
for value in peers:
    peer = ipaddress.ip_address(value)
    if peer.is_loopback or peer.is_unspecified:
        raise SystemExit("EXAPI_OPERATOR_PEER_IPS must contain non-loopback peer addresses")
PY

if command -v ip >/dev/null 2>&1; then
  ip -o addr show dev "$EXAPI_WIREGUARD_INTERFACE" | awk '{print $4}' | cut -d/ -f1 | \
    grep -Fxq "$EXAPI_CONTROL_BIND_HOST" || die 'EXAPI_CONTROL_BIND_HOST is not assigned to EXAPI_WIREGUARD_INTERFACE'
elif command -v ifconfig >/dev/null 2>&1; then
  ifconfig "$EXAPI_WIREGUARD_INTERFACE" | grep -Fq "$EXAPI_CONTROL_BIND_HOST" || \
    die 'EXAPI_CONTROL_BIND_HOST is not assigned to EXAPI_WIREGUARD_INTERFACE'
else
  die 'ip or ifconfig is required to verify the WireGuard bind address'
fi

compose_project=${COMPOSE_PROJECT_NAME:-exapi-production}
compose_file=${COMPOSE_FILE:-deploy/docker-compose.yml}
app_service=${EXAPI_APP_SERVICE:-sub2api}
db_service=${EXAPI_DB_SERVICE:-postgres}
report_path=${EXAPI_REPORT_FILE:-/app/data/private-migration-report.json}
verification_path=${EXAPI_REPORT_EVIDENCE_FILE:-/app/data/private-migration-evidence.json}
key_container_path=/run/exapi-private-cutover/report.key
verification_command=${EXAPI_VERIFY_REPORT_COMMAND:-/app/verify-private-cutover-report}
rollout_id=${EXAPI_ROLLOUT_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$(git rev-parse --short=12 HEAD 2>/dev/null || printf manual)}

[[ "$rollout_id" =~ ^[A-Za-z0-9._-]+$ ]] || die 'EXAPI_ROLLOUT_ID contains unsafe characters'

# Bind provider-side batch cleanup to the signed migration report. The adapter
# must inspect every configured provider and publish its detailed evidence to
# an immutable/versioned S3 object before returning this compact attestation.
evidence_dir=${EXAPI_EVIDENCE_DIR:-"$ROOT_DIR/tmp/rollouts/$rollout_id/private-cutover"}
mkdir -p "$evidence_dir"
chmod 700 "$evidence_dir"
batch_cleanup_evidence=$(mktemp "$evidence_dir/batch-cleanup-evidence.XXXXXX") || \
  die 'cannot create protected batch cleanup evidence file'
batch_cleanup_json=$(EXAPI_ROLLOUT_ID="$rollout_id" \
  "$EXAPI_BATCH_CLEANUP_VERIFY_COMMAND") || die 'batch cleanup verification adapter failed'
printf '%s\n' "$batch_cleanup_json" >"$batch_cleanup_evidence"
chmod 600 "$batch_cleanup_evidence"
BATCH_CLEANUP_JSON="$batch_cleanup_json" python3 - <<'PY'
import datetime
import json
import os
import re
from urllib.parse import urlparse

data = json.loads(os.environ["BATCH_CLEANUP_JSON"])
expected = {
    "schema_version", "verified", "verified_at", "sql_rows_remaining",
    "provider_jobs_remaining", "provider_inputs_remaining",
    "provider_outputs_remaining", "evidence_uri", "evidence_version_id",
    "evidence_sha256",
}
if set(data) != expected:
    raise SystemExit("batch cleanup attestation has unexpected fields")
if data["schema_version"] != 1 or data["verified"] is not True:
    raise SystemExit("batch cleanup attestation is not verified schema version 1")
for field in (
    "sql_rows_remaining", "provider_jobs_remaining",
    "provider_inputs_remaining", "provider_outputs_remaining",
):
    if type(data[field]) is not int or data[field] != 0:
        raise SystemExit(f"batch cleanup attestation requires {field}=0")
try:
    verified_at = datetime.datetime.fromisoformat(
        data["verified_at"].removesuffix("Z") + "+00:00"
    )
except (AttributeError, TypeError, ValueError) as exc:
    raise SystemExit("batch cleanup verified_at must be RFC3339 UTC") from exc
if not data["verified_at"].endswith("Z") or verified_at.tzinfo is None:
    raise SystemExit("batch cleanup verified_at must be RFC3339 UTC")
now = datetime.datetime.now(datetime.timezone.utc)
if verified_at < now - datetime.timedelta(minutes=15) or verified_at > now + datetime.timedelta(minutes=5):
    raise SystemExit("batch cleanup attestation is stale or from the future")

def valid_s3_object_uri(value):
    if not isinstance(value, str):
        return False
    parsed = urlparse(value)
    try:
        port = parsed.port
    except ValueError:
        return False
    return (
        parsed.scheme == "s3"
        and bool(parsed.netloc)
        and bool(parsed.path.strip("/"))
        and parsed.username is None
        and parsed.password is None
        and port is None
        and not parsed.query
        and not parsed.fragment
    )

if not valid_s3_object_uri(data["evidence_uri"]):
    raise SystemExit("batch cleanup evidence_uri must be an off-host s3:// URI")
if not isinstance(data["evidence_version_id"], str) or not data["evidence_version_id"].strip():
    raise SystemExit("batch cleanup evidence_version_id is required")
if not isinstance(data["evidence_sha256"], str) or re.fullmatch(r"[0-9a-f]{64}", data["evidence_sha256"]) is None:
    raise SystemExit("batch cleanup evidence_sha256 must be a lowercase SHA-256 digest")
PY

# Read the operator-supplied path exactly once through descriptor-relative,
# no-symlink traversal. All later consumers use this protected single-link
# copy, so a changed host source cannot split the migration and archive keys.
mkdir -p "$ROOT_DIR/tmp"
key_stage_dir=$(mktemp -d "$ROOT_DIR/tmp/private-cutover-key.${rollout_id}.XXXXXX") || \
  die 'cannot create protected key staging directory'
chmod 700 "$key_stage_dir"
staged_key="$key_stage_dir/report.key"
secret_volume=
cleanup_private_cutover_secret() {
  if [[ -n "$secret_volume" ]]; then
    docker volume rm -f "$secret_volume" >/dev/null 2>&1 || true
  fi
  unlink "$staged_key" >/dev/null 2>&1 || true
  rmdir "$key_stage_dir" >/dev/null 2>&1 || true
}
trap cleanup_private_cutover_secret EXIT HUP INT TERM
key_stage_json=$(python3 deploy/ops/stage-private-cutover-report-key.py \
  "$EXAPI_MIGRATION_REPORT_KEY_FILE" "$staged_key") || die 'cannot stage protected report key'
# Ensure every subsequently launched process sees only the staged path. The
# operator-supplied mutable path is never opened again during this run.
EXAPI_MIGRATION_REPORT_KEY_FILE=$staged_key
export EXAPI_MIGRATION_REPORT_KEY_FILE
key_digest_values=$(KEY_STAGE_JSON="$key_stage_json" python3 - <<'PY'
import json
import os
import re

data = json.loads(os.environ["KEY_STAGE_JSON"])
fields = ("key_file_sha256", "report_key_sha256")
if set(data) != set(fields):
    raise SystemExit("key staging metadata has unexpected fields")
for field in fields:
    if not isinstance(data[field], str) or re.fullmatch(r"[0-9a-f]{64}", data[field]) is None:
        raise SystemExit(f"key staging metadata has invalid {field}")
print(data["key_file_sha256"], data["report_key_sha256"])
PY
)
read -r key_file_sha256 report_key_sha256 extra_digest <<<"$key_digest_values"
[[ -z "${extra_digest:-}" && "$key_file_sha256" =~ ^[0-9a-f]{64}$ && "$report_key_sha256" =~ ^[0-9a-f]{64}$ ]] || \
  die 'cannot parse protected key fingerprints'

[[ "$report_path" = /* && "$verification_path" = /* ]] || die 'report paths must be absolute container paths'
[[ "$key_container_path" = /* ]] || die 'key container path must be absolute'
[[ "$report_path" != "$verification_path" ]] || die 'report and verification paths must differ'

COMPOSE_ENV_FILE="$COMPOSE_ENV_FILE" EXAPI_IMAGE="$EXAPI_IMAGE" \
  deploy/ops/validate-immutable-compose.sh -f "$compose_file" >/dev/null || \
  die 'Compose image references are not immutable or the configuration is invalid'

# Exactly one explicit local-backup policy is required. S3 backup_records are
# deliberately not inspected here: those objects are retained by cutover and
# are independent from any legacy local backup files this flag may purge.
backup_args=()
if [[ -n "${EXAPI_LEGACY_LOCAL_BACKUP_DIR:-}" && "${EXAPI_NO_LOCAL_BACKUPS:-false}" == true ]]; then
  die 'EXAPI_LEGACY_LOCAL_BACKUP_DIR and EXAPI_NO_LOCAL_BACKUPS are mutually exclusive'
elif [[ -n "${EXAPI_LEGACY_LOCAL_BACKUP_DIR:-}" ]]; then
  [[ "$EXAPI_LEGACY_LOCAL_BACKUP_DIR" = /* ]] || die 'EXAPI_LEGACY_LOCAL_BACKUP_DIR must be an absolute container path'
  backup_args=(--local-backup-dir "$EXAPI_LEGACY_LOCAL_BACKUP_DIR")
elif [[ "${EXAPI_NO_LOCAL_BACKUPS:-false}" == true ]]; then
  backup_args=(--no-local-backups)
else
  die 'set exactly one of EXAPI_LEGACY_LOCAL_BACKUP_DIR or EXAPI_NO_LOCAL_BACKUPS=true'
fi

compose() {
  COMPOSE_PROJECT_NAME="$compose_project" \
  EXAPI_IMAGE="$EXAPI_IMAGE" \
  EXAPI_CONTROL_BIND_HOST="$EXAPI_CONTROL_BIND_HOST" \
  EXAPI_CONTROL_LISTEN_ADDR="$EXAPI_CONTROL_LISTEN_ADDR" \
  EXAPI_CONTROL_HOSTS="$EXAPI_CONTROL_HOSTS" \
  EXAPI_OPERATOR_PEER_IPS="$EXAPI_OPERATOR_PEER_IPS" \
  docker compose --env-file "$COMPOSE_ENV_FILE" -f "$compose_file" "$@"
}

# Resolve and inspect the immutable candidate before taking downtime.
docker pull "$EXAPI_IMAGE" >/dev/null
image_revision=$(docker image inspect "$EXAPI_IMAGE" --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}')
[[ "$image_revision" == "$EXAPI_REVIEWED_COMMIT" ]] || die 'image revision does not match EXAPI_REVIEWED_COMMIT'

# A bind-mounted operator-owned 0600 staged key is intentionally unreadable
# after the image entrypoint drops to UID 1000. Copy the already-staged bytes
# into an ephemeral Docker volume as UID 1000 and always destroy that volume.
secret_volume=$(docker volume create)
docker run --rm --network none --entrypoint /bin/sh \
  -v "$staged_key:/source/report.key:ro" \
  -v "$batch_cleanup_evidence:/source/batch-cleanup-evidence.json:ro" \
  -v "$secret_volume:/target" \
  "$EXAPI_IMAGE" -ec '
    cp /source/report.key /target/report.key
    cp /source/batch-cleanup-evidence.json /target/batch-cleanup-evidence.json
    chown 1000:1000 /target/report.key /target/batch-cleanup-evidence.json
    chmod 0600 /target/report.key /target/batch-cleanup-evidence.json
  '

run_image_command() {
  compose run --rm --no-deps -T \
    -e "EXAPI_PREFLIGHT_LOCAL_BACKUP_DIR=${EXAPI_LEGACY_LOCAL_BACKUP_DIR:-}" \
    -v "$secret_volume:/run/exapi-private-cutover:ro" \
    "$app_service" "$@"
}

# Exercise the exact entrypoint, key wrapper, binaries, production data volume,
# and database network while the current application is still available.
run_image_command /app/with-migration-report-key.sh "$key_container_path" /bin/sh -ec '
  test -x /app/migrate-private-only
  test -x /app/verify-private-cutover-report
  test -d /app/data
  if [ -n "${EXAPI_PREFLIGHT_LOCAL_BACKUP_DIR:-}" ]; then test -d "$EXAPI_PREFLIGHT_LOCAL_BACKUP_DIR"; fi
  PGPASSWORD="$DATABASE_PASSWORD" psql -h "$DATABASE_HOST" -p "$DATABASE_PORT" \
    -U "$DATABASE_USER" -d "$DATABASE_DBNAME" -Atqc "SELECT 1" | grep -Fxq 1
  PGPASSWORD="$DATABASE_PASSWORD" psql -h "$DATABASE_HOST" -p "$DATABASE_PORT" \
    -U "$DATABASE_USER" -d "$DATABASE_DBNAME" -Atqc "SELECT COUNT(*) FROM batch_image_jobs" | grep -Fxq 0
'

service_running() {
  compose ps --status running --services | grep -Fxq "$1"
}

service_count() {
  compose ps --all -q "$1" | awk 'NF { count++ } END { print count + 0 }'
}

service_healthy() {
  local container_id health
  container_id=$(compose ps -q "$1" | head -n 1)
  [[ -n "$container_id" ]] || return 1
  health=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container_id")
  [[ "$health" == healthy ]]
}

service_running "$db_service" || die "$db_service must remain running for the offline cutover"
# The private readiness gate is intentionally false before cutover, so the
# application healthcheck may report unhealthy even while its process is alive.
# PostgreSQL, which remains online for the migration, must be healthy.
service_healthy "$db_service" || die "$db_service must be healthy before the offline cutover"
[[ "$(service_count "$app_service")" -eq 1 ]] || die "expected exactly one retained $app_service container"
if [[ "$resume" == true ]]; then
  ! service_running "$app_service" || die "$app_service must remain stopped in resume mode"
else
  service_running "$app_service" || die "$app_service must be running before the initial offline cutover"
fi

maintenance_expected_replicas=1
if [[ "$resume" == true ]]; then
  maintenance_expected_replicas=0
fi
EXAPI_ROLLOUT_ID="$rollout_id" EXAPI_EXPECTED_APP_REPLICAS="$maintenance_expected_replicas" \
  "$EXAPI_MAINTENANCE_VERIFY_COMMAND" || die 'maintenance/ingress/replica gate failed'

# The application is intentionally never restarted by this script. A failed
# migration, verification, archive, or operator interruption therefore leaves
# admission stopped instead of silently returning to a partially cut-over DB.
if [[ "$resume" == false ]]; then
  compose stop -t "${EXAPI_STOP_TIMEOUT_SECONDS:-60}" "$app_service" >/dev/null
fi
if service_running "$app_service"; then
  die "$app_service is still running after stop"
fi
service_running "$db_service" || die "$db_service stopped unexpectedly; refusing cutover"
service_healthy "$db_service" || die "$db_service is not healthy after application stop"
EXAPI_ROLLOUT_ID="$rollout_id" EXAPI_EXPECTED_APP_REPLICAS=0 \
  "$EXAPI_MAINTENANCE_VERIFY_COMMAND" || die 'post-stop maintenance/ingress/replica gate failed'

database_clients=$(compose exec -T "$db_service" sh -ec \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT COUNT(*) FROM pg_stat_activity WHERE datname = current_database() AND pid <> pg_backend_pid() AND backend_type = '\''client backend'\''"')
[[ "$database_clients" == 0 ]] || die "unexpected PostgreSQL client sessions remain after app stop: $database_clients"

run_image_command /app/with-migration-report-key.sh "$key_container_path" \
  /app/migrate-private-only \
  --confirm "$EXAPI_CONFIRMATION" \
  "${backup_args[@]}" \
  --batch-cleanup-evidence-file /run/exapi-private-cutover/batch-cleanup-evidence.json \
  --report-file "$report_path"

# Verification is a separate one-shot invocation so the signed report is
# checked against the durable database marker before any restart is permitted.
run_image_command /app/with-migration-report-key.sh "$key_container_path" \
  "$verification_command" \
  --report-file "$report_path" \
  --evidence-file "$verification_path"

staged_report="$evidence_dir/private-migration-report.json"
staged_verification="$evidence_dir/private-migration-evidence.json"
archive_result="$evidence_dir/private-migration-archive.json"
archive_verification="$evidence_dir/private-migration-archive-verification.json"

# The named data volume is mounted by the stopped production service. Copy only
# the signed report and verification evidence to a protected local staging area;
# the archive adapter must encrypt and upload both plus the HMAC key.
compose cp "$app_service:$report_path" "$staged_report"
compose cp "$app_service:$verification_path" "$staged_verification"
chmod 600 "$staged_report" "$staged_verification"

archive_json=$(EXAPI_ROLLOUT_ID="$rollout_id" \
  EXAPI_MIGRATION_REPORT_FILE="$staged_report" \
  EXAPI_MIGRATION_EVIDENCE_FILE="$staged_verification" \
  EXAPI_MIGRATION_REPORT_KEY_FILE="$staged_key" \
  EXAPI_MIGRATION_REPORT_KEY_FILE_SHA256="$key_file_sha256" \
  EXAPI_MIGRATION_REPORT_KEY_SHA256="$report_key_sha256" \
  "$EXAPI_REPORT_ARCHIVE_COMMAND")
printf '%s\n' "$archive_json" >"$archive_result"
chmod 600 "$archive_result"
ARCHIVE_JSON="$archive_json" REPORT_FILE="$staged_report" EVIDENCE_FILE="$staged_verification" \
KEY_FILE="$staged_key" KEY_FILE_SHA256="$key_file_sha256" REPORT_KEY_SHA256="$report_key_sha256" python3 - <<'PY'
import hashlib
import json
import os
import re
import stat

data = json.loads(os.environ["ARCHIVE_JSON"])
if data.get("encrypted") is not True:
    raise SystemExit("archive adapter did not report encrypted=true")
for key in ("report_uri", "report_version_id", "evidence_uri", "evidence_version_id", "key_uri", "key_version_id", "retention_until", "report_file_sha256", "evidence_file_sha256", "key_file_sha256", "report_key_sha256"):
    if not isinstance(data.get(key), str) or not data[key].strip():
        raise SystemExit(f"archive adapter omitted {key}")

def digest_value(field):
    value = data[field].removeprefix("sha256:")
    if re.fullmatch(r"[0-9a-f]{64}", value) is None:
        raise SystemExit(f"archive adapter {field} is not a lowercase SHA-256 digest")
    return value

key_fd = os.open(os.environ["KEY_FILE"], os.O_RDONLY | os.O_NOFOLLOW)
try:
    key_info = os.fstat(key_fd)
    key_bytes = os.read(key_fd, 66)
finally:
    os.close(key_fd)
if not stat.S_ISREG(key_info.st_mode) or stat.S_IMODE(key_info.st_mode) != 0o600 or key_info.st_nlink != 1:
    raise SystemExit("staged key identity or permissions changed before archive validation")
if len(key_bytes) != 65 or key_bytes[-1:] != b"\n" or any(value not in b"0123456789abcdef" for value in key_bytes[:-1]):
    raise SystemExit("staged key encoding changed before archive validation")
for field, path in (("report_file_sha256", os.environ["REPORT_FILE"]), ("evidence_file_sha256", os.environ["EVIDENCE_FILE"])):
    digest = hashlib.sha256(open(path, "rb").read()).hexdigest()
    if digest_value(field) != digest:
        raise SystemExit(f"archive adapter {field} does not match the staged file")
if digest_value("key_file_sha256") != hashlib.sha256(key_bytes).hexdigest() or digest_value("key_file_sha256") != os.environ["KEY_FILE_SHA256"]:
    raise SystemExit("staged key file does not match the immutable source snapshot")
decoded_key_digest = hashlib.sha256(bytes.fromhex(key_bytes[:-1].decode("ascii"))).hexdigest()
if digest_value("report_key_sha256") != decoded_key_digest or decoded_key_digest != os.environ["REPORT_KEY_SHA256"]:
    raise SystemExit("archive adapter decoded report-key fingerprint does not match the staged key")
verification = json.load(open(os.environ["EVIDENCE_FILE"], encoding="utf-8"))
report_file_digest = hashlib.sha256(open(os.environ["REPORT_FILE"], "rb").read()).hexdigest()
if verification.get("verified") is not True or verification.get("report_file_sha256") != report_file_digest:
    raise SystemExit("verification evidence is not bound to the staged signed report")
if verification.get("report_key_sha256") != os.environ["REPORT_KEY_SHA256"]:
    raise SystemExit("verification/database evidence is not bound to the staged decoded report key")
if not data["retention_until"].endswith("Z"):
    raise SystemExit("archive retention_until must be RFC3339 UTC")
from datetime import datetime, timezone
retention = datetime.fromisoformat(data["retention_until"][:-1] + "+00:00")
if retention <= datetime.now(timezone.utc):
    raise SystemExit("archive retention_until must be in the future")
from urllib.parse import urlparse

def valid_s3_object_uri(value):
    if not isinstance(value, str):
        return False
    parsed = urlparse(value)
    try:
        port = parsed.port
    except ValueError:
        return False
    return (
        parsed.scheme == "s3"
        and bool(parsed.netloc)
        and bool(parsed.path.strip("/"))
        and parsed.username is None
        and parsed.password is None
        and port is None
        and not parsed.query
        and not parsed.fragment
    )

for field in ("report_uri", "evidence_uri", "key_uri"):
    if not valid_s3_object_uri(data[field]):
        raise SystemExit(f"archive adapter {field} must use an off-host s3:// URI")
PY

archive_verification_json=$(EXAPI_ROLLOUT_ID="$rollout_id" \
  EXAPI_ARCHIVE_RESULT_FILE="$archive_result" \
  "$EXAPI_REPORT_ARCHIVE_VERIFY_COMMAND")
printf '%s\n' "$archive_verification_json" >"$archive_verification"
chmod 600 "$archive_verification"
ARCHIVE_JSON="$archive_json" VERIFICATION_JSON="$archive_verification_json" python3 - <<'PY'
import json
import os

archive = json.loads(os.environ["ARCHIVE_JSON"])
verified = json.loads(os.environ["VERIFICATION_JSON"])
for key in ("verified", "encrypted", "immutable"):
    if verified.get(key) is not True:
        raise SystemExit(f"independent archive verification omitted {key}=true")
for key in ("report_uri", "report_version_id", "evidence_uri", "evidence_version_id", "key_uri", "key_version_id", "retention_until", "report_file_sha256", "evidence_file_sha256", "key_file_sha256", "report_key_sha256"):
    if verified.get(key) != archive.get(key):
        raise SystemExit(f"independent archive verification mismatched {key}")
PY

printf 'private cutover verified and archived; application remains stopped\n'
printf 'review %s, then start the exact reviewed image explicitly\n' "$archive_result"
