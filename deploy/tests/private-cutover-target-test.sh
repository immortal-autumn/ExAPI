#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT_DIR"
fail() { printf 'private cutover target test failed: %s\n' "$*" >&2; exit 1; }

mkdir -p tmp
test_dir=$(mktemp -d tmp/cutover-target-test.XXXXXX)
report_file=
cleanup() {
  rm -rf "$test_dir"
  [[ -z "$report_file" ]] || rm -f "$report_file"
}
trap cleanup EXIT HUP INT TERM

secure_tmp=
if [[ -n "${EXAPI_CONTRACT_SECURE_TMP_DIR:-}" ]]; then
  mkdir -p "$EXAPI_CONTRACT_SECURE_TMP_DIR" || fail 'cannot create EXAPI_CONTRACT_SECURE_TMP_DIR'
  candidates=("$EXAPI_CONTRACT_SECURE_TMP_DIR")
else
  candidates=("$ROOT_DIR/tmp" /dev/shm "${TMPDIR:-/tmp}" /tmp)
fi
for candidate in "${candidates[@]}"; do
  [[ -d "$candidate" && -w "$candidate" ]] || continue
  mode_probe=$(mktemp "$candidate/cutover-target-test-mode.XXXXXX") || continue
  chmod 600 "$mode_probe"
  mode=$(stat -c '%a' "$mode_probe" 2>/dev/null || stat -f '%Lp' "$mode_probe" 2>/dev/null || true)
  rm -f "$mode_probe"
  if [[ "$mode" == 600 ]]; then
    secure_tmp=$candidate
    break
  fi
done
[[ -n "$secure_tmp" ]] || fail 'no permission-capable temporary directory; set EXAPI_CONTRACT_SECURE_TMP_DIR'
report_file=$(mktemp "$secure_tmp/exapi-cutover-target-test.XXXXXX")

candidate_compose="$test_dir/docker-compose.v0.2.6.yml"
legacy_compose="$test_dir/docker-compose.local.yml"
candidate_env="$test_dir/.env.v0.2.6"
legacy_env="$test_dir/.env"
for file in "$candidate_compose" "$legacy_compose"; do
  printf 'services: {}\n' >"$file"
done
for file in "$candidate_env" "$legacy_env"; do
  printf 'TEST_ONLY=true\n' >"$file"
  chmod 600 "$file"
done

export PATH="$ROOT_DIR/deploy/tests/fixtures/cutover-target:$PATH"
export COMPOSE_FILE="$candidate_compose"
export COMPOSE_ENV_FILE="$candidate_env"
export COMPOSE_PROJECT_NAME=sub2api
export EXAPI_IMAGE="ghcr.io/example/exapi@sha256:$(printf '%064d' 0)"
export EXAPI_CONTROL_BIND_HOST=100.97.17.1
export EXAPI_WIREGUARD_INTERFACE=wg0
export EXAPI_CUTOVER_TARGET_REPORT="$report_file"
export CUTOVER_APP_PROVENANCE_COMPOSE="$candidate_compose"
export CUTOVER_DB_PROVENANCE_COMPOSE="$legacy_compose"
export CUTOVER_APP_PROVENANCE_ENV="$candidate_env"
export CUTOVER_DB_PROVENANCE_ENV="$legacy_env"

first=$(deploy/ops/report-private-cutover-target.sh)
second=$(deploy/ops/report-private-cutover-target.sh)
[[ "$first" =~ ^[0-9a-f]{64}$ && "$first" == "$second" ]] || \
  fail 'stable target identity did not produce a stable digest'
report_mode=$(stat -c '%a' "$EXAPI_CUTOVER_TARGET_REPORT" 2>/dev/null || \
  stat -f '%Lp' "$EXAPI_CUTOVER_TARGET_REPORT" 2>/dev/null || true)
[[ "$report_mode" == 600 ]] || \
  fail 'target report is not mode 0600'

python3 - "$EXAPI_CUTOVER_TARGET_REPORT" "$candidate_compose" "$legacy_compose" <<'PY'
import json
import os
import sys

report = json.load(open(sys.argv[1], encoding="utf-8"))
assert report["candidate"]["compose_project"] == "sub2api"
assert report["candidate"]["compose_file"] == os.path.realpath(sys.argv[2])
assert report["current"]["application"]["compose_files"][0]["path"] == os.path.realpath(sys.argv[2])
assert report["current"]["database"]["compose_files"][0]["path"] == os.path.realpath(sys.argv[3])
assert report["current"]["database"]["compose_environment_files"] == []
assert report["current"]["application"]["container_id"] == "a" * 64
assert report["current"]["database"]["container_id"] == "b" * 64
assert report["current"]["application"]["image_id"] == "sha256:" + "e" * 64
assert report["current"]["application"]["image_reference"] != report["candidate"]["image"]
assert report["current"]["application"]["mounts_sha256"]
assert report["current"]["database_identity"]["system_identifier"] == "7429012345678901234"
assert report["current"]["database_identity"]["server_port"] == "local-socket"
assert report["candidate"]["compose_environment_sha256"]
assert report["wireguard"]["interface"] == "wg0"
PY

CUTOVER_APP_ID=$(printf '1%.0s' {1..64})
export CUTOVER_APP_ID
changed=$(deploy/ops/report-private-cutover-target.sh)
[[ "$changed" != "$first" ]] || fail 'container identity change did not alter the target digest'

export EXAPI_CONTROL_BIND_HOST=100.97.17.99
if deploy/ops/report-private-cutover-target.sh >/dev/null 2>&1; then
  fail 'report accepted a control address absent from the WireGuard interface'
fi

printf 'private cutover target test passed\n'
