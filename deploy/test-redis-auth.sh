#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

for file in deploy/docker-compose.yml deploy/docker-compose.local.yml; do
  immutable_image='ghcr.io/example/exapi@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
  if EXAPI_IMAGE="$immutable_image" REDIS_PASSWORD= POSTGRES_PASSWORD='test-only-postgres' \
    SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=test \
    SUB2API_DATA_ENCRYPTION_KEYS_JSON='{"test":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}' \
    SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=digest-test \
    SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON='{"digest-test":"QkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkI="}' \
    docker compose -f "$file" config >/dev/null 2>&1; then
    fail "$file rendered without REDIS_PASSWORD"
  fi
  rendered=$(EXAPI_IMAGE="$immutable_image" REDIS_PASSWORD='test-only-redis-password' POSTGRES_PASSWORD='test-only-postgres' \
    SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=test \
    SUB2API_DATA_ENCRYPTION_KEYS_JSON='{"test":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}' \
    SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=digest-test \
    SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON='{"digest-test":"QkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkI="}' \
    docker compose -f "$file" config --format json)
  python3 -c '
import json, sys
cfg = json.load(sys.stdin)
expected = "exec redis-server --save 60 1 --appendonly yes --appendfsync everysec --requirepass \"$$REDISCLI_AUTH\""
if cfg["services"]["redis"]["command"] != ["sh", "-c", expected]:
    raise SystemExit("Redis command is not a single exec with requirepass")
if cfg["services"]["sub2api"]["environment"].get("REDIS_PASSWORD") != "test-only-redis-password":
    raise SystemExit("application Redis password is missing")
' <<<"$rendered"
done

printf 'redis-auth-contract: PASS\n'
