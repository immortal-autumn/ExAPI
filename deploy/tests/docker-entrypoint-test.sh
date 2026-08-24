#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"
fail() { printf 'docker entrypoint test failed: %s\n' "$1" >&2; exit 1; }

fixture_path="$repo_root/deploy/tests/fixtures/entrypoint:$PATH"
mkdir -p tmp/entrypoint-writable

if PATH="$fixture_path" ENTRYPOINT_TEST_UID=0 DATA_DIR="$repo_root/tmp/entrypoint-writable" \
  deploy/docker-entrypoint.sh /bin/true >/dev/null 2>&1; then
  fail 'root runtime was accepted'
fi

if PATH="$fixture_path" ENTRYPOINT_TEST_UID=1000 DATA_DIR="$repo_root/tmp/entrypoint-missing" \
  deploy/docker-entrypoint.sh /bin/true >/dev/null 2>&1; then
  fail 'missing data directory was accepted'
fi

if PATH="$fixture_path" ENTRYPOINT_TEST_UID=1000 DATA_DIR=/sys \
  deploy/docker-entrypoint.sh /bin/true >/dev/null 2>&1; then
  fail 'unwritable data directory was accepted'
fi

result=$(PATH="$fixture_path" ENTRYPOINT_TEST_UID=1000 \
  DATA_DIR="$repo_root/tmp/entrypoint-writable" \
  deploy/docker-entrypoint.sh /bin/sh -c 'printf entrypoint-exec-ok')
[ "$result" = entrypoint-exec-ok ] || fail 'normal non-root startup did not exec its command'

printf 'docker entrypoint test passed\n'
