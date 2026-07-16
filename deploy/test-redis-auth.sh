#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

for file in deploy/docker-compose.yml deploy/docker-compose.local.yml; do
  if REDIS_PASSWORD= docker compose -f "$file" config >/dev/null 2>&1; then
    fail "$file rendered without REDIS_PASSWORD"
  fi
  rendered=$(REDIS_PASSWORD='test-only-redis-password' POSTGRES_PASSWORD='test-only-postgres' \
    SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=test \
    SUB2API_DATA_ENCRYPTION_KEYS_JSON='{"test":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}' \
    docker compose -f "$file" config)
  grep -q -- '--requirepass "test-only-redis-password"' <<<"$rendered" || fail "$file omitted Redis requirepass"
  grep -q 'REDIS_PASSWORD: test-only-redis-password' <<<"$rendered" || fail "$file omitted application Redis password"
done

printf 'redis-auth-contract: PASS\n'
