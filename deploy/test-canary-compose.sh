#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

fail() {
    printf 'canary Compose contract failed: %s\n' "$1" >&2
    exit 1
}

command -v docker >/dev/null 2>&1 || fail 'docker CLI is required'
docker compose version >/dev/null 2>&1 || fail 'Docker Compose v2 is required'

immutable_image='ghcr.io/example/exapi@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
postgres_image='postgres@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
redis_image='redis@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc'
rendered=$(
    COMPOSE_PROJECT_NAME=exapi-canary \
    EXAPI_IMAGE="$immutable_image" \
    POSTGRES_IMAGE="$postgres_image" \
    REDIS_IMAGE="$redis_image" \
    EXAPI_CONTAINER_NAME=exapi-canary \
    EXAPI_POSTGRES_CONTAINER_NAME=exapi-canary-postgres \
    EXAPI_REDIS_CONTAINER_NAME=exapi-canary-redis \
    BIND_HOST=127.0.0.1 \
    SERVER_PORT=18080 \
    POSTGRES_PASSWORD=canary-postgres \
    REDIS_PASSWORD=canary-redis \
    SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=data-canary \
    SUB2API_DATA_ENCRYPTION_KEYS_JSON='{"data-canary":"QUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUE="}' \
    SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=digest-canary \
    SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON='{"digest-canary":"QkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkI="}' \
    docker compose -f deploy/docker-compose.yml config --format json
)

COMPOSE_CONFIG_JSON="$rendered" python3 - "$immutable_image" <<'PY'
import json, os, sys
expected_image = sys.argv[1]
config = json.loads(os.environ["COMPOSE_CONFIG_JSON"])
services = config["services"]
expected_names = {
    "sub2api": "exapi-canary",
    "postgres": "exapi-canary-postgres",
    "redis": "exapi-canary-redis",
}
for service, expected in expected_names.items():
    actual = services[service].get("container_name")
    if actual != expected:
        raise SystemExit(f"{service} container name is {actual!r}, expected {expected!r}")
if services["sub2api"].get("image") != expected_image:
    raise SystemExit("canary image is not the requested immutable digest")
published = services["sub2api"]["ports"][0]
if published.get("host_ip") != "127.0.0.1" or int(published.get("published")) != 18080:
    raise SystemExit("canary is not bound to the dedicated loopback port")
for volume in config.get("volumes", {}).values():
    if not volume.get("name", "").startswith("exapi-canary_"):
        raise SystemExit("canary volume is not isolated by Compose project name")
PY

printf 'digest-pinned canary Compose contract: pass\n'
