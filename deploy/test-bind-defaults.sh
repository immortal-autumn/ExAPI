#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

expect_literal() {
  local file=$1 literal=$2
  grep -Fq -- "$literal" "$file" || fail "$file is missing: $literal"
}

reject_literal() {
  local file=$1 literal=$2
  if grep -Fq -- "$literal" "$file"; then
    fail "$file still contains unsafe default: $literal"
  fi
}

for file in \
  deploy/docker-compose.yml \
  deploy/docker-compose.local.yml \
  deploy/docker-compose.standalone.yml \
  deploy/docker-compose.dev.yml; do
  expect_literal "$file" '${BIND_HOST:-127.0.0.1}:${SERVER_PORT:-8080}:8080'
  reject_literal "$file" '${BIND_HOST:-0.0.0.0}:${SERVER_PORT:-8080}:8080'
done

expect_literal deploy/.env.example 'BIND_HOST=127.0.0.1'
reject_literal deploy/.env.example 'BIND_HOST=0.0.0.0'
expect_literal deploy/install.sh 'SERVER_HOST="127.0.0.1"'
reject_literal deploy/install.sh 'SERVER_HOST="0.0.0.0"'
expect_literal deploy/sub2api.service 'Environment=SERVER_HOST=127.0.0.1'
reject_literal deploy/sub2api.service 'Environment=SERVER_HOST=0.0.0.0'

printf 'Deployment bind defaults are loopback-only.\n'
