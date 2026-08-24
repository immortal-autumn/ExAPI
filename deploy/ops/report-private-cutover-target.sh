#!/usr/bin/env bash
# Produce a secret-free, stable identity report for the exact private cutover
# target. The report is reviewed in dry-run mode and its digest is required by
# the real cutover immediately before the application is stopped.
set -euo pipefail

umask 077
ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT_DIR"

die() { printf 'private cutover target report failed: %s\n' "$*" >&2; exit 1; }
require_env() { [[ -n "${!1:-}" ]] || die "$1 is required"; }

for command_name in docker python3 ip; do
  command -v "$command_name" >/dev/null 2>&1 || die "$command_name is required"
done
docker compose version >/dev/null 2>&1 || die 'Docker Compose v2 is required'
for name in COMPOSE_FILE COMPOSE_ENV_FILE COMPOSE_PROJECT_NAME EXAPI_IMAGE \
  EXAPI_CONTROL_BIND_HOST EXAPI_WIREGUARD_INTERFACE EXAPI_CUTOVER_TARGET_REPORT; do
  require_env "$name"
done
[[ "$EXAPI_IMAGE" =~ @sha256:[0-9a-f]{64}$ ]] || die 'EXAPI_IMAGE must be immutable by digest'

app_service=${EXAPI_APP_SERVICE:-sub2api}
db_service=${EXAPI_DB_SERVICE:-postgres}
[[ "$app_service" =~ ^[a-zA-Z0-9._-]+$ ]] || die 'EXAPI_APP_SERVICE is invalid'
[[ "$db_service" =~ ^[a-zA-Z0-9._-]+$ ]] || die 'EXAPI_DB_SERVICE is invalid'

canonical_path() {
  python3 - "$1" <<'PY'
import os
import sys
print(os.path.realpath(sys.argv[1]))
PY
}

compose_file=$(canonical_path "$COMPOSE_FILE")
env_file=$(canonical_path "$COMPOSE_ENV_FILE")
report_file=$(canonical_path "$EXAPI_CUTOVER_TARGET_REPORT")
[[ -f "$compose_file" && -r "$compose_file" ]] || die 'COMPOSE_FILE must be a readable regular file'
[[ -f "$env_file" && -r "$env_file" ]] || die 'COMPOSE_ENV_FILE must be a readable regular file'
mkdir -p "$(dirname "$report_file")" "$ROOT_DIR/tmp"

secure_tmp=
if [[ -n "${EXAPI_SECURE_TMP_DIR:-}" ]]; then
  mkdir -p "$EXAPI_SECURE_TMP_DIR"
  secure_tmp=$EXAPI_SECURE_TMP_DIR
else
  for candidate in "$ROOT_DIR/tmp" /dev/shm "${TMPDIR:-/tmp}" /tmp; do
    [[ -d "$candidate" && -w "$candidate" ]] || continue
    mode_probe=$(mktemp "$candidate/cutover-target-mode.XXXXXX") || continue
    chmod 600 "$mode_probe"
    mode=$(stat -c '%a' "$mode_probe" 2>/dev/null || stat -f '%Lp' "$mode_probe" 2>/dev/null || true)
    rm -f "$mode_probe"
    if [[ "$mode" == 600 ]]; then
      secure_tmp=$candidate
      break
    fi
  done
fi
[[ -n "$secure_tmp" ]] || die 'no permission-capable temporary directory; set EXAPI_SECURE_TMP_DIR'

tmp_dir=$(mktemp -d "$secure_tmp/cutover-target.XXXXXX") || die 'cannot create report staging directory'
chmod 700 "$tmp_dir"
cleanup_target_report() { rm -f "$tmp_dir"/*; rmdir "$tmp_dir" 2>/dev/null || true; }
trap cleanup_target_report EXIT HUP INT TERM

compose() {
  COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME" EXAPI_IMAGE="$EXAPI_IMAGE" \
    EXAPI_CONTROL_BIND_HOST="$EXAPI_CONTROL_BIND_HOST" \
    docker compose --env-file "$env_file" -f "$compose_file" "$@"
}

compose config --format json >"$tmp_dir/compose.json"
chmod 600 "$tmp_dir/compose.json"

single_container_id() {
  local ids count
  ids=$(compose ps --all -q "$1")
  count=$(awk 'NF { count++ } END { print count + 0 }' <<<"$ids")
  [[ "$count" -eq 1 ]] || die "expected exactly one retained $1 container, found $count"
  awk 'NF { print; exit }' <<<"$ids"
}

app_container_id=$(single_container_id "$app_service")
db_container_id=$(single_container_id "$db_service")
docker inspect "$app_container_id" "$db_container_id" >"$tmp_dir/inspect.json"
chmod 600 "$tmp_dir/inspect.json"

# Query through the exact retained database container ID. No credential is
# copied to the host: the official image's local socket and POSTGRES_* env are
# used inside the container.
docker exec "$db_container_id" sh -ec '
  psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -AtF "|" -c \
    "SELECT current_database(), current_user, COALESCE(inet_server_addr()::text, '\''local-socket'\''), COALESCE(inet_server_port()::text, '\''local-socket'\''), (SELECT system_identifier::text FROM pg_control_system())"
' >"$tmp_dir/database.txt"
chmod 600 "$tmp_dir/database.txt"

ip -j addr show dev "$EXAPI_WIREGUARD_INTERFACE" >"$tmp_dir/interface.json" || \
  die 'cannot inspect EXAPI_WIREGUARD_INTERFACE'
chmod 600 "$tmp_dir/interface.json"

COMPOSE_FILE_CANONICAL="$compose_file" COMPOSE_ENV_CANONICAL="$env_file" \
COMPOSE_PROJECT_EXPECTED="$COMPOSE_PROJECT_NAME" APP_SERVICE="$app_service" \
DB_SERVICE="$db_service" APP_CONTAINER_ID="$app_container_id" \
DB_CONTAINER_ID="$db_container_id" CANDIDATE_IMAGE="$EXAPI_IMAGE" \
CONTROL_BIND_HOST="$EXAPI_CONTROL_BIND_HOST" WG_INTERFACE="$EXAPI_WIREGUARD_INTERFACE" \
REPORT_FILE="$report_file" python3 - "$tmp_dir" <<'PY'
import hashlib
import ipaddress
import json
import os
import pathlib
import re
import sys
import tempfile

tmp = pathlib.Path(sys.argv[1])

def digest_bytes(value):
    return hashlib.sha256(value).hexdigest()

def digest_file(path):
    with open(path, "rb") as handle:
        return digest_bytes(handle.read())

def canonical_json(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=True).encode()

def labelled_files(labels, field, required):
    raw = labels.get(field, "")
    paths = []
    for value in raw.split(","):
        value = value.strip()
        if not value:
            continue
        path = os.path.realpath(value)
        if not os.path.isfile(path):
            raise SystemExit(f"container provenance file is missing: {path}")
        paths.append({"path": path, "sha256": digest_file(path)})
    if required and not paths:
        raise SystemExit(f"container label {field} is missing")
    return paths

def normalized_mounts(container):
    mounts = []
    for mount in container.get("Mounts", []):
        mounts.append({
            "destination": mount.get("Destination"),
            "driver": mount.get("Driver", ""),
            "name": mount.get("Name", ""),
            "propagation": mount.get("Propagation", ""),
            "read_write": mount.get("RW") is True,
            "source": mount.get("Source"),
            "type": mount.get("Type"),
        })
    mounts.sort(key=lambda item: (item["destination"] or "", item["source"] or ""))
    return mounts

def service_identity(container, expected_id, service, required_mount):
    container_id = container.get("Id", "")
    if container_id != expected_id or re.fullmatch(r"[0-9a-f]{64}", container_id) is None:
        raise SystemExit(f"{service} container identity changed during inspection")
    labels = container.get("Config", {}).get("Labels") or {}
    if labels.get("com.docker.compose.project") != os.environ["COMPOSE_PROJECT_EXPECTED"]:
        raise SystemExit(f"{service} belongs to a different Compose project")
    if labels.get("com.docker.compose.service") != service:
        raise SystemExit(f"{service} container has the wrong Compose service label")
    if labels.get("com.docker.compose.oneoff") != "False":
        raise SystemExit(f"{service} resolved to a one-off container")
    config_hash = labels.get("com.docker.compose.config-hash", "")
    if re.fullmatch(r"[0-9a-f]{64}", config_hash) is None:
        raise SystemExit(f"{service} has an invalid Compose config hash label")
    working_directory = labels.get("com.docker.compose.project.working_dir", "")
    if not working_directory or not os.path.isdir(os.path.realpath(working_directory)):
        raise SystemExit(f"{service} has an invalid Compose working-directory label")
    mounts = normalized_mounts(container)
    required = [item for item in mounts if item["destination"] == required_mount]
    if len(required) != 1 or required[0]["read_write"] is not True:
        raise SystemExit(f"{service} must have exactly one writable {required_mount} mount")
    image_id = container.get("Image", "")
    if re.fullmatch(r"sha256:[0-9a-f]{64}", image_id) is None:
        raise SystemExit(f"{service} image ID is not a sha256 digest")
    return {
        "compose_config_hash": config_hash,
        "compose_environment_files": labelled_files(
            labels, "com.docker.compose.project.environment_file", False
        ),
        "compose_files": labelled_files(
            labels, "com.docker.compose.project.config_files", True
        ),
        "compose_working_directory": os.path.realpath(working_directory),
        "container_id": container_id,
        "container_name": container.get("Name", "").removeprefix("/"),
        "image_id": image_id,
        "image_reference": container.get("Config", {}).get("Image", ""),
        "mounts": mounts,
        "mounts_sha256": digest_bytes(canonical_json(mounts)),
    }

compose = json.loads((tmp / "compose.json").read_text())
services = compose.get("services", {})
app_name = os.environ["APP_SERVICE"]
db_name = os.environ["DB_SERVICE"]
if app_name not in services or db_name not in services:
    raise SystemExit("candidate Compose file does not define the application and database services")
if services[app_name].get("image") != os.environ["CANDIDATE_IMAGE"]:
    raise SystemExit("rendered application image differs from EXAPI_IMAGE")
for name, service in services.items():
    image = service.get("image")
    if image is not None and re.fullmatch(r"[^\s@]+@sha256:[0-9a-f]{64}", image) is None:
        raise SystemExit(f"candidate service {name} image is not immutable")

containers = {item["Id"]: item for item in json.loads((tmp / "inspect.json").read_text())}
app_id = os.environ["APP_CONTAINER_ID"]
db_id = os.environ["DB_CONTAINER_ID"]
if set(containers) != {app_id, db_id}:
    raise SystemExit("Docker inspect did not return the exact selected containers")

database_fields = (tmp / "database.txt").read_text().strip().split("|")
if len(database_fields) != 5 or not all(database_fields):
    raise SystemExit("database identity query returned an invalid record")
database = {
    "database": database_fields[0],
    "user": database_fields[1],
    "server_address": database_fields[2],
    "server_port": database_fields[3],
    "system_identifier": database_fields[4],
}

interfaces = json.loads((tmp / "interface.json").read_text())
if len(interfaces) != 1 or interfaces[0].get("ifname") != os.environ["WG_INTERFACE"]:
    raise SystemExit("WireGuard interface identity is ambiguous")
addresses = sorted(
    item["local"]
    for item in interfaces[0].get("addr_info", [])
    if item.get("family") in {"inet", "inet6"} and item.get("local")
)
bind = str(ipaddress.ip_address(os.environ["CONTROL_BIND_HOST"]))
if bind not in addresses:
    raise SystemExit("control bind address is not assigned to the selected WireGuard interface")

environment_map = {
    name: service.get("environment", {}) for name, service in sorted(services.items())
}
report = {
    "schema_version": 1,
    "candidate": {
        "compose_config_sha256": digest_bytes(canonical_json(compose)),
        "compose_environment_sha256": digest_bytes(canonical_json(environment_map)),
        "compose_file": os.environ["COMPOSE_FILE_CANONICAL"],
        "compose_file_sha256": digest_file(os.environ["COMPOSE_FILE_CANONICAL"]),
        "compose_project": os.environ["COMPOSE_PROJECT_EXPECTED"],
        "environment_file": os.environ["COMPOSE_ENV_CANONICAL"],
        "environment_file_sha256": digest_file(os.environ["COMPOSE_ENV_CANONICAL"]),
        "image": os.environ["CANDIDATE_IMAGE"],
    },
    "current": {
        "application": service_identity(
            containers[app_id], app_id, app_name, "/app/data"
        ),
        "database": service_identity(
            containers[db_id], db_id, db_name, "/var/lib/postgresql/data"
        ),
        "database_identity": database,
        "database_identity_sha256": digest_bytes(canonical_json(database)),
    },
    "wireguard": {
        "addresses": addresses,
        "control_bind_address": bind,
        "ifindex": interfaces[0].get("ifindex"),
        "interface": os.environ["WG_INTERFACE"],
    },
}

report_path = pathlib.Path(os.environ["REPORT_FILE"])
payload = json.dumps(report, indent=2, sort_keys=True).encode() + b"\n"
fd, staged = tempfile.mkstemp(prefix=f".{report_path.name}.", dir=report_path.parent)
try:
    os.fchmod(fd, 0o600)
    with os.fdopen(fd, "wb") as handle:
        handle.write(payload)
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(staged, report_path)
    if (report_path.stat().st_mode & 0o777) != 0o600:
        raise SystemExit("target report filesystem does not preserve mode 0600")
finally:
    try:
        os.unlink(staged)
    except FileNotFoundError:
        pass
print(digest_bytes(payload))
PY
