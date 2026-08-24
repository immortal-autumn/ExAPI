#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

command -v docker >/dev/null 2>&1 || {
  printf 'docker compose is required for the rendered security contract\n' >&2
  exit 1
}
docker compose version >/dev/null 2>&1 || {
  printf 'Docker Compose v2 is required for the rendered security contract\n' >&2
  exit 1
}

mkdir -p tmp
rendered=$(mktemp tmp/compose-security.XXXXXX)
cleanup() { rm -f "$rendered"; }
trap cleanup EXIT HUP INT TERM

render_and_check() {
  compose_file=$1
  EXAPI_IMAGE='ghcr.io/example/exapi@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' \
  POSTGRES_IMAGE='postgres@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb' \
  REDIS_IMAGE='redis@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc' \
  POSTGRES_PASSWORD=test REDIS_PASSWORD=test DATABASE_HOST=postgres DATABASE_PASSWORD=test \
  REDIS_HOST=redis \
  SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=data \
  SUB2API_DATA_ENCRYPTION_KEYS_JSON='{"data":"QUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUE="}' \
  SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=digest \
  SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON='{"digest":"QkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkI="}' \
    docker compose --env-file /dev/null -f "$compose_file" config --format json >"$rendered"

  COMPOSE_FILE_UNDER_TEST="$compose_file" python3 - "$rendered" <<'PY'
import json
import os
import sys

path = os.environ["COMPOSE_FILE_UNDER_TEST"]
with open(sys.argv[1], encoding="utf-8") as handle:
    config = json.load(handle)

services = config.get("services", {})
if "sub2api" not in services:
    raise SystemExit(f"{path}: sub2api service is missing")
for name, service in services.items():
    user = service.get("user", "")
    if not user or user.split(":", 1)[0] == "0":
        raise SystemExit(f"{path}: {name} does not declare a non-root user")
    if service.get("read_only") is not True:
        raise SystemExit(f"{path}: {name} root filesystem is writable")
    if service.get("cap_drop") != ["ALL"]:
        raise SystemExit(f"{path}: {name} does not drop every capability")
    if "no-new-privileges:true" not in service.get("security_opt", []):
        raise SystemExit(f"{path}: {name} lacks no-new-privileges")
    if not isinstance(service.get("pids_limit"), int) or service["pids_limit"] <= 0:
        raise SystemExit(f"{path}: {name} lacks a positive PID limit")
    memory_limit = service.get("mem_limit")
    if not str(memory_limit).isdigit() or int(memory_limit) <= 0:
        raise SystemExit(f"{path}: {name} lacks a positive memory limit")
    if not isinstance(service.get("cpus"), (int, float)) or service["cpus"] <= 0:
        raise SystemExit(f"{path}: {name} lacks a positive CPU limit")
    if not any(value.startswith("/tmp:") for value in service.get("tmpfs", [])):
        raise SystemExit(f"{path}: {name} lacks an explicit writable /tmp tmpfs")
    logging = service.get("logging", {})
    options = logging.get("options", {})
    if logging.get("driver") != "json-file" or not options.get("max-size") or not options.get("max-file"):
        raise SystemExit(f"{path}: {name} lacks bounded json-file logging")

application = services["sub2api"]
if application.get("user") != "1000:1000":
    raise SystemExit(f"{path}: application is not explicitly UID/GID 1000")
data_mounts = [item for item in application.get("volumes", []) if item.get("target") == "/app/data"]
if len(data_mounts) != 1 or data_mounts[0].get("read_only") is True:
    raise SystemExit(f"{path}: application data must be the sole writable /app/data mount")

if "postgres" in services:
    postgres_tmpfs = services["postgres"].get("tmpfs", [])
    if not any(value.startswith("/var/run/postgresql:") for value in postgres_tmpfs):
        raise SystemExit(f"{path}: PostgreSQL lacks an explicit writable socket tmpfs")
PY
}

for compose_file in \
  deploy/docker-compose.yml \
  deploy/docker-compose.local.yml \
  deploy/docker-compose.standalone.yml \
  deploy/docker-compose.dev.yml
do
  render_and_check "$compose_file"
done

grep -Fq 'USER sub2api:sub2api' deploy/Dockerfile || {
  printf 'deploy/Dockerfile must set the non-root runtime user\n' >&2
  exit 1
}
grep -Fq 'USER sub2api:sub2api' Dockerfile.goreleaser || {
  printf 'Dockerfile.goreleaser must set the non-root runtime user\n' >&2
  exit 1
}
grep -Fq 'USER sub2api:sub2api' Dockerfile || {
  printf 'Dockerfile must set the non-root runtime user\n' >&2
  exit 1
}
if grep -Eq 'chown[[:space:]]+-R|chown[[:space:]]+--recursive' deploy/docker-entrypoint.sh deploy/Dockerfile Dockerfile.goreleaser Dockerfile; then
  printf 'production runtime files must not recursively chown application data\n' >&2
  exit 1
fi

deploy/tests/docker-entrypoint-test.sh

printf 'docker compose security test passed\n'
