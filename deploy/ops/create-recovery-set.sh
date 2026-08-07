#!/usr/bin/env bash
# Create one encrypted logical backup, one quiesced provider snapshot, and a
# separately encrypted environment/keyring object. No secret value is logged.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT_DIR"

die() { printf 'recovery set failed: %s\n' "$*" >&2; exit 1; }
require_env() { [[ -n "${!1:-}" ]] || die "$1 is required"; }
require_command() { command -v "$1" >/dev/null 2>&1 || die "$1 is required"; }

for name in EXAPI_IMAGE RECOVERY_S3_URI SECRETS_S3_URI AGE_RECIPIENTS_FILE \
  SECRETS_AGE_RECIPIENTS_FILE EXAPI_ENV_FILE SNAPSHOT_CREATE_COMMAND \
  RECOVERY_RETENTION_UNTIL SECRETS_RETENTION_UNTIL; do
  require_env "$name"
done
for command_name in age aws docker python3; do require_command "$command_name"; done
docker compose version >/dev/null 2>&1 || die 'Docker Compose v2 is required'
[[ "$EXAPI_IMAGE" =~ @sha256:[0-9a-f]{64}$ ]] || die 'EXAPI_IMAGE must be immutable by digest'
RECOVERY_RETENTION_UNTIL="$RECOVERY_RETENTION_UNTIL" SECRETS_RETENTION_UNTIL="$SECRETS_RETENTION_UNTIL" python3 - <<'PY'
import os
from datetime import datetime, timezone
for name in ("RECOVERY_RETENTION_UNTIL", "SECRETS_RETENTION_UNTIL"):
    value = os.environ[name]
    if not value.endswith("Z"):
        raise SystemExit(f"{name} must be RFC3339 UTC")
    parsed = datetime.fromisoformat(value[:-1] + "+00:00")
    if parsed <= datetime.now(timezone.utc):
        raise SystemExit(f"{name} must be in the future")
PY
[[ -x "$SNAPSHOT_CREATE_COMMAND" ]] || die 'SNAPSHOT_CREATE_COMMAND must be an executable adapter'
[[ -r "$AGE_RECIPIENTS_FILE" && -r "$SECRETS_AGE_RECIPIENTS_FILE" ]] || die 'age recipient files must be readable'
[[ -r "$EXAPI_ENV_FILE" ]] || die 'EXAPI_ENV_FILE must be readable'
[[ "$(stat -c '%a' "$EXAPI_ENV_FILE" 2>/dev/null || stat -f '%Lp' "$EXAPI_ENV_FILE")" =~ ^(400|600)$ ]] || \
  die 'EXAPI_ENV_FILE must have mode 0400 or 0600'
AGE_RECIPIENTS_FILE="$AGE_RECIPIENTS_FILE" SECRETS_AGE_RECIPIENTS_FILE="$SECRETS_AGE_RECIPIENTS_FILE" python3 - <<'PY'
import os

def recipients(path):
    result = set()
    with open(path, encoding="utf-8") as handle:
        for raw in handle:
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            parts = line.split()
            # OpenSSH public-key comments are not part of recipient identity.
            # Normalize them so changing only a comment cannot evade separation.
            if parts[0].startswith("ssh-") and len(parts) >= 2:
                line = " ".join(parts[:2])
            result.add(line)
    return result

database = recipients(os.environ["AGE_RECIPIENTS_FILE"])
secrets = recipients(os.environ["SECRETS_AGE_RECIPIENTS_FILE"])
if not database or not secrets:
    raise SystemExit("age recipient files must each contain at least one recipient")
overlap = database & secrets
if overlap:
    raise SystemExit("database and secret recovery objects must have disjoint age recipients")
PY

COMPOSE_FILE=${COMPOSE_FILE:-deploy/docker-compose.yml}
COMPOSE_ENV_FILE=${COMPOSE_ENV_FILE:-$EXAPI_ENV_FILE}
COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME:-exapi-production}
EXAPI_APP_SERVICE=${EXAPI_APP_SERVICE:-sub2api}
EXAPI_DB_SERVICE=${EXAPI_DB_SERVICE:-postgres}
POSTGRES_USER=${POSTGRES_USER:-sub2api}
POSTGRES_DB=${POSTGRES_DB:-sub2api}
ROLLOUT_ID=${ROLLOUT_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$(git rev-parse --short=12 HEAD)}
OPS_TMP_DIR=${OPS_TMP_DIR:-$ROOT_DIR/tmp/rollouts/$ROLLOUT_ID}
mkdir -p "$OPS_TMP_DIR"
chmod 700 "$OPS_TMP_DIR"

COMPOSE_ENV_FILE="$COMPOSE_ENV_FILE" deploy/ops/validate-immutable-compose.sh -f "$COMPOSE_FILE" >/dev/null

AWS_ARGS=()
[[ -z "${S3_ENDPOINT_URL:-}" ]] || AWS_ARGS+=(--endpoint-url "$S3_ENDPOINT_URL")

parse_s3() {
  python3 - "$1" <<'PY'
import sys
from urllib.parse import urlparse
u = urlparse(sys.argv[1])
if u.scheme != "s3" or not u.netloc or not u.path.lstrip("/") or any(c.isspace() for c in sys.argv[1]):
    raise SystemExit("invalid s3 URI")
print(u.netloc, u.path.lstrip("/"))
PY
}

read -r recovery_bucket recovery_prefix < <(parse_s3 "$RECOVERY_S3_URI") || die 'invalid RECOVERY_S3_URI'
read -r secrets_bucket secrets_prefix < <(parse_s3 "$SECRETS_S3_URI") || die 'invalid SECRETS_S3_URI'
[[ "$recovery_bucket/$recovery_prefix" != "$secrets_bucket/$secrets_prefix" ]] || \
  die 'recovery and secrets objects require distinct protected locations'
for bucket in "$recovery_bucket" "$secrets_bucket"; do
  status=$(aws "${AWS_ARGS[@]}" s3api get-bucket-versioning --bucket "$bucket" --query Status --output text)
  [[ "$status" == Enabled ]] || die "bucket versioning is not enabled for $bucket"
done

backup_uri="${RECOVERY_S3_URI%/}/$ROLLOUT_ID/postgres.dump.age"
secrets_uri="${SECRETS_S3_URI%/}/$ROLLOUT_ID/exapi.env.age"
backup_checksum="$OPS_TMP_DIR/postgres.dump.age.sha256"
secrets_checksum="$OPS_TMP_DIR/exapi.env.age.sha256"
snapshot_json="$OPS_TMP_DIR/snapshot-create.json"
recovery_json="$OPS_TMP_DIR/recovery-evidence.json"
app_stopped=false

restart_app() {
  if [[ "$app_stopped" == true ]]; then
    COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME" docker compose --env-file "$COMPOSE_ENV_FILE" -f "$COMPOSE_FILE" start "$EXAPI_APP_SERVICE" >/dev/null || true
  fi
}
trap restart_app EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME" docker compose --env-file "$COMPOSE_ENV_FILE" -f "$COMPOSE_FILE" stop -t 60 "$EXAPI_APP_SERVICE"
app_stopped=true
COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME" docker compose --env-file "$COMPOSE_ENV_FILE" -f "$COMPOSE_FILE" exec -T "$EXAPI_DB_SERVICE" \
  psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c 'CHECKPOINT' >/dev/null

EXAPI_ROLLOUT_ID="$ROLLOUT_ID" EXAPI_WRITER_QUIESCED=true EXAPI_CHECKPOINT_COMPLETED=true \
  "$SNAPSHOT_CREATE_COMMAND" >"$snapshot_json"
RECOVERY_RETENTION_UNTIL="$RECOVERY_RETENTION_UNTIL" python3 - "$snapshot_json" <<'PY'
import json, os, sys
from datetime import datetime, timezone
data = json.load(open(sys.argv[1], encoding="utf-8"))
for key in ("provider", "snapshot_id", "target", "created_at", "retention_until"):
    if not isinstance(data.get(key), str) or not data[key].strip():
        raise SystemExit(f"snapshot adapter omitted {key}")
parsed = {}
for key in ("created_at", "retention_until"):
    if not data[key].endswith("Z"):
        raise SystemExit(f"snapshot {key} is not RFC3339 UTC")
    parsed[key] = datetime.fromisoformat(data[key][:-1] + "+00:00")
requested = datetime.fromisoformat(os.environ["RECOVERY_RETENTION_UNTIL"][:-1] + "+00:00")
if parsed["retention_until"] <= datetime.now(timezone.utc):
    raise SystemExit("snapshot retention_until must be in the future")
if parsed["retention_until"] < requested:
    raise SystemExit("snapshot retention_until is shorter than RECOVERY_RETENTION_UNTIL")
PY

# Stream encrypted bytes directly off host; only small checksums/evidence use tmp/.
COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME" docker compose --env-file "$COMPOSE_ENV_FILE" -f "$COMPOSE_FILE" exec -T "$EXAPI_DB_SERVICE" \
  pg_dump --format=custom --compress=6 --no-owner --no-acl -U "$POSTGRES_USER" "$POSTGRES_DB" |
  age -R "$AGE_RECIPIENTS_FILE" |
  python3 deploy/ops/stream_hash.py "$backup_checksum" |
  aws "${AWS_ARGS[@]}" s3 cp - "$backup_uri" --only-show-errors

COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME" docker compose --env-file "$COMPOSE_ENV_FILE" -f "$COMPOSE_FILE" start "$EXAPI_APP_SERVICE" >/dev/null
app_stopped=false
trap - EXIT INT TERM

age -R "$SECRETS_AGE_RECIPIENTS_FILE" "$EXAPI_ENV_FILE" |
  python3 deploy/ops/stream_hash.py "$secrets_checksum" |
  aws "${AWS_ARGS[@]}" s3 cp - "$secrets_uri" --only-show-errors

head_object() {
  local uri=$1 version_id=${2:-} bucket key
  read -r bucket key < <(parse_s3 "$uri")
  local args=(s3api head-object --bucket "$bucket" --key "$key" --output json)
  [[ -z "$version_id" ]] || args+=(--version-id "$version_id")
  aws "${AWS_ARGS[@]}" "${args[@]}"
}
backup_head=$(head_object "$backup_uri")
secrets_head=$(head_object "$secrets_uri")
backup_version=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["VersionId"])' <<<"$backup_head")
secrets_version=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["VersionId"])' <<<"$secrets_head")
[[ -n "$backup_version" && "$backup_version" != null ]] || die 'backup object has no version ID'
[[ -n "$secrets_version" && "$secrets_version" != null ]] || die 'secrets object has no version ID'
read -r backup_bucket backup_key < <(parse_s3 "$backup_uri")
read -r secret_bucket secret_key < <(parse_s3 "$secrets_uri")
aws "${AWS_ARGS[@]}" s3api put-object-retention --bucket "$backup_bucket" --key "$backup_key" \
  --version-id "$backup_version" --retention "Mode=COMPLIANCE,RetainUntilDate=$RECOVERY_RETENTION_UNTIL" >/dev/null
aws "${AWS_ARGS[@]}" s3api put-object-retention --bucket "$secret_bucket" --key "$secret_key" \
  --version-id "$secrets_version" --retention "Mode=COMPLIANCE,RetainUntilDate=$SECRETS_RETENTION_UNTIL" >/dev/null
backup_head=$(head_object "$backup_uri" "$backup_version")
secrets_head=$(head_object "$secrets_uri" "$secrets_version")

ROLLOUT_ID="$ROLLOUT_ID" BACKUP_URI="$backup_uri" SECRETS_URI="$secrets_uri" \
BACKUP_CHECKSUM="$(tr -d '\n' < "$backup_checksum")" SECRETS_CHECKSUM="$(tr -d '\n' < "$secrets_checksum")" \
BACKUP_HEAD="$backup_head" SECRETS_HEAD="$secrets_head" EXAPI_ENV_FILE="$EXAPI_ENV_FILE" \
RECOVERY_RETENTION_UNTIL="$RECOVERY_RETENTION_UNTIL" SECRETS_RETENTION_UNTIL="$SECRETS_RETENTION_UNTIL" \
python3 - "$snapshot_json" "$recovery_json" <<'PY'
import json, os, sys
from datetime import datetime, timezone

snapshot = json.load(open(sys.argv[1], encoding="utf-8"))
backup_head = json.loads(os.environ["BACKUP_HEAD"])
secrets_head = json.loads(os.environ["SECRETS_HEAD"])
env = {}
with open(os.environ["EXAPI_ENV_FILE"], encoding="utf-8") as handle:
    for raw in handle:
        line = raw.strip()
        if line and not line.startswith("#") and "=" in line:
            key, value = line.split("=", 1)
            env[key] = value.strip("'\"")
key_ids = []
for name in (
    "SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID",
    "SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID",
    "SUB2API_BACKUP_ENCRYPTION_ACTIVE_KEY_ID",
):
    if not env.get(name):
        raise SystemExit(f"missing active key ID in protected environment: {name}")
    key_ids.append(env[name])
requested_retentions = {
    "backup": os.environ["RECOVERY_RETENTION_UNTIL"],
    "secrets": os.environ["SECRETS_RETENTION_UNTIL"],
}
for label, head in (("backup", backup_head), ("secrets", secrets_head)):
    if not head.get("VersionId") or head["VersionId"] == "null":
        raise SystemExit(f"{label} object has no version ID")
    if head.get("ObjectLockMode") != "COMPLIANCE" or not head.get("ObjectLockRetainUntilDate"):
        raise SystemExit(f"{label} object does not have compliance retention")
    actual = datetime.fromisoformat(head["ObjectLockRetainUntilDate"].replace("Z", "+00:00"))
    requested = datetime.fromisoformat(requested_retentions[label][:-1] + "+00:00")
    if actual <= datetime.now(timezone.utc):
        raise SystemExit(f"{label} object retention is not in the future")
    if actual < requested:
        raise SystemExit(f"{label} object retention is shorter than requested")
document = {
    "schema_version": 1,
    "rollout_id": os.environ["ROLLOUT_ID"],
    "created_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
    "logical": {
        "object_uri": os.environ["BACKUP_URI"],
        "version_id": backup_head["VersionId"],
        "sha256": os.environ["BACKUP_CHECKSUM"],
        "encrypted": True,
        "size_bytes": backup_head["ContentLength"],
        "retention_until": os.environ["RECOVERY_RETENTION_UNTIL"],
    },
    "snapshot": {
        **snapshot,
        "writer_quiesced": True,
        "checkpoint_completed": True,
    },
    "secrets": {
        "object_uri": os.environ["SECRETS_URI"],
        "version_id": secrets_head["VersionId"],
        "sha256": os.environ["SECRETS_CHECKSUM"],
        "encrypted": True,
        "size_bytes": secrets_head["ContentLength"],
        "retention_until": os.environ["SECRETS_RETENTION_UNTIL"],
        "key_ids": key_ids,
    },
}
with open(sys.argv[2], "w", encoding="utf-8") as handle:
    json.dump(document, handle, indent=2, sort_keys=True)
    handle.write("\n")
PY

printf 'Recovery evidence: %s\n' "$recovery_json"
printf 'The recovery set is not promotion-ready until both objects are restored independently into disposable targets.\n'
