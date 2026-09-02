#!/usr/bin/env bash
# Restore an exact versioned encrypted pg_dump into a networkless disposable
# PostgreSQL container and emit evidence only after repository-owned checks and
# any additional operator assertions pass.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT_DIR"
die() { printf 'logical restore drill failed: %s\n' "$*" >&2; exit 1; }
require_env() { [[ -n "${!1:-}" ]] || die "$1 is required"; }
for name in RECOVERY_EVIDENCE AGE_IDENTITY_FILE POSTGRES_IMAGE; do require_env "$name"; done
for command_name in age aws docker python3; do command -v "$command_name" >/dev/null 2>&1 || die "$command_name is required"; done
[[ "$POSTGRES_IMAGE" =~ @sha256:[0-9a-f]{64}$ ]] || die 'POSTGRES_IMAGE must be immutable by digest'
required_checks="$ROOT_DIR/deploy/ops/restore-checks.required.sql"
[[ -r "$RECOVERY_EVIDENCE" && -r "$AGE_IDENTITY_FILE" && -r "$required_checks" ]] || die 'a required input file is unreadable'
if [[ -n "${RESTORE_VERIFY_SQL_FILE:-}" && ! -r "$RESTORE_VERIFY_SQL_FILE" ]]; then
  die 'RESTORE_VERIFY_SQL_FILE is unreadable'
fi

readarray -t recovery < <(python3 - "$RECOVERY_EVIDENCE" <<'PY'
import json, sys
from urllib.parse import urlparse
data = json.load(open(sys.argv[1], encoding="utf-8"))
item = data["logical"]
u = urlparse(item["object_uri"])
if u.scheme != "s3" or not u.netloc or not u.path.lstrip("/"):
    raise SystemExit("logical object is not an s3 URI")
print(u.netloc)
print(u.path.lstrip("/"))
print(item["version_id"])
print(item["sha256"])
print(data["rollout_id"])
PY
)
(( ${#recovery[@]} == 5 )) || die 'invalid recovery evidence'
bucket=${recovery[0]}; key=${recovery[1]}; version_id=${recovery[2]}; expected_sha=${recovery[3]}; rollout_id=${recovery[4]}

OPS_TMP_DIR=${OPS_TMP_DIR:-$ROOT_DIR/tmp/rollouts/$rollout_id/logical-restore}
mkdir -p "$OPS_TMP_DIR"; chmod 700 "$OPS_TMP_DIR"
fifo="$OPS_TMP_DIR/object.fifo"
actual_checksum="$OPS_TMP_DIR/download.sha256"
evidence="$OPS_TMP_DIR/logical-restore-evidence.json"
container="exapi-logical-restore-${rollout_id//[^a-zA-Z0-9_.-]/-}"
volume="${container}-data"
AWS_ARGS=(); [[ -z "${S3_ENDPOINT_URL:-}" ]] || AWS_ARGS+=(--endpoint-url "$S3_ENDPOINT_URL")
rm -f "$fifo"; mkfifo -m 600 "$fifo"

cleanup() {
  rm -f "$fifo"
  if [[ "${KEEP_RESTORE_TARGET:-false}" != true ]]; then
    docker rm -f "$container" >/dev/null 2>&1 || true
    docker volume rm "$volume" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

docker volume create --label "exapi.rollout_id=$rollout_id" --label "exapi.restore_source=logical" "$volume" >/dev/null
docker run -d --name "$container" --network none \
  -e POSTGRES_HOST_AUTH_METHOD=trust -e POSTGRES_DB=exapi_restore -e PGDATA=/var/lib/postgresql/data \
  -v "$volume:/var/lib/postgresql/data" "$POSTGRES_IMAGE" >/dev/null
for _ in $(seq 1 60); do
  docker exec "$container" pg_isready -U postgres -d exapi_restore >/dev/null 2>&1 && break
  sleep 1
done
docker exec "$container" pg_isready -U postgres -d exapi_restore >/dev/null 2>&1 || die 'disposable PostgreSQL did not become ready'

aws "${AWS_ARGS[@]}" s3api get-object --bucket "$bucket" --key "$key" --version-id "$version_id" "$fifo" >/dev/null &
download_pid=$!
python3 deploy/ops/stream_hash.py "$actual_checksum" <"$fifo" |
  age --decrypt -i "$AGE_IDENTITY_FILE" |
  docker exec -i "$container" pg_restore --exit-on-error --no-owner --no-acl -U postgres -d exapi_restore
wait "$download_pid"
[[ "$(tr -d '\n' < "$actual_checksum")" == "$expected_sha" ]] || die 'downloaded encrypted object checksum does not match recovery evidence'

required_checks_sha=$(sha256sum "$required_checks" | awk '{print $1}')
docker exec -i "$container" psql -v ON_ERROR_STOP=1 -U postgres -d exapi_restore <"$required_checks" \
  >"$OPS_TMP_DIR/verification-required.txt"
optional_checks_sha=""
if [[ -n "${RESTORE_VERIFY_SQL_FILE:-}" ]]; then
  optional_checks_sha=$(sha256sum "$RESTORE_VERIFY_SQL_FILE" | awk '{print $1}')
  docker exec -i "$container" psql -v ON_ERROR_STOP=1 -U postgres -d exapi_restore <"$RESTORE_VERIFY_SQL_FILE" \
    >"$OPS_TMP_DIR/verification-operator.txt"
fi

# Persist the restored database cardinalities so the later restored-data
# network proof can compare the same disposable restore, rather than relying
# on a stale hard-coded account/key count.
database_counts=$(docker exec "$container" psql -U postgres -d exapi_restore -AtF "|" -c \
  "SELECT (SELECT COUNT(*) FROM users),(SELECT COUNT(*) FROM accounts WHERE deleted_at IS NULL),(SELECT COUNT(*) FROM api_keys WHERE deleted_at IS NULL),(SELECT COUNT(*) FROM batch_image_jobs),(SELECT COUNT(*) FROM schema_migrations),(SELECT COUNT(*) FROM groups)")
IFS='|' read -r count_users count_accounts count_api_keys count_batch_jobs count_migrations count_groups extra_counts <<<"$database_counts"
[[ -z "${extra_counts:-}" && "$count_users" =~ ^[0-9]+$ && "$count_accounts" =~ ^[0-9]+$ && \
  "$count_api_keys" =~ ^[0-9]+$ && "$count_batch_jobs" =~ ^[0-9]+$ && \
  "$count_migrations" =~ ^[0-9]+$ && "$count_groups" =~ ^[0-9]+$ ]] || \
  die 'restored database counts are not numeric'
database_counts_json=$(python3 - "$count_users" "$count_accounts" "$count_api_keys" "$count_batch_jobs" "$count_migrations" "$count_groups" <<'PY'
import json, sys
keys = ("users", "active_accounts", "active_api_keys", "batch_image_jobs", "schema_migrations", "groups")
print(json.dumps(dict(zip(keys, (int(value) for value in sys.argv[1:]))), sort_keys=True, separators=(",", ":")))
PY
)

ROLLOUT_ID="$rollout_id" CONTAINER="$container" VOLUME="$volume" OBJECT_SHA="$expected_sha" \
POSTGRES_IMAGE="$POSTGRES_IMAGE" REQUIRED_CHECKS_SHA="$required_checks_sha" \
OPTIONAL_CHECKS_SHA="$optional_checks_sha" DATABASE_COUNTS_JSON="$database_counts_json" python3 - "$evidence" <<'PY'
import json, os, sys
from datetime import datetime, timezone
data = {
    "rollout_id": os.environ["ROLLOUT_ID"],
    "disposable_target": os.environ["CONTAINER"],
    "volume": os.environ["VOLUME"],
    "database": "exapi_restore",
    "postgres_image": os.environ["POSTGRES_IMAGE"],
    "restored_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
    "backup_sha256": os.environ["OBJECT_SHA"],
    "required_validator_sha256": os.environ["REQUIRED_CHECKS_SHA"],
    "required_validator_verified": True,
    "operator_validator_sha256": os.environ["OPTIONAL_CHECKS_SHA"] or None,
    "operator_validator_verified": bool(os.environ["OPTIONAL_CHECKS_SHA"]),
    "database_counts": json.loads(os.environ["DATABASE_COUNTS_JSON"]),
    "network_mode": "none",
    "verified": True,
}
with open(sys.argv[1], "w", encoding="utf-8") as handle:
    json.dump(data, handle, indent=2, sort_keys=True); handle.write("\n")
PY
printf 'Logical restore evidence: %s\n' "$evidence"
if [[ "${KEEP_RESTORE_TARGET:-false}" == true ]]; then
  printf 'Disposable target retained as %s; it has Docker network mode none.\n' "$container"
fi
