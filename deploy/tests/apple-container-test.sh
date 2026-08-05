#!/bin/bash

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "${TEST_DIR}/.." && pwd)"
SCRIPT="${DEPLOY_DIR}/apple-container.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/sub2api-apple-test.XXXXXX")"
STATE_DIR="${TEST_ROOT}/state"
ENV_FILE="${TEST_ROOT}/sub2api.env"

cleanup() {
    rm -rf "${TEST_ROOT}"
}
trap cleanup EXIT

fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

assert_exists() {
    [[ -e "$1" ]] || fail "Expected path to exist: $1"
}

assert_missing() {
    [[ ! -e "$1" ]] || fail "Expected path to be absent: $1"
}

export FAKE_CONTAINER_STATE="${STATE_DIR}"
export PATH="${TEST_DIR}/fixtures/bin:${PATH}"
export SUB2API_ENV_FILE="${ENV_FILE}"

mkdir -p "${STATE_DIR}"

"${SCRIPT}" init
if [[ "$(uname -s)" == "Darwin" ]]; then
    env_mode="$(stat -f '%Lp' "${ENV_FILE}")"
else
    env_mode="$(stat -c '%a' "${ENV_FILE}")"
fi
[[ "${env_mode}" == "600" ]] || fail "init did not create a mode-600 env file"
grep -q '^POSTGRES_PASSWORD=change_this_secure_password$' "${ENV_FILE}" && fail "init retained the placeholder password"
python3 - "${ENV_FILE}" <<'PY' || fail "init did not create three valid independent keyrings"
import base64, json, sys
values = {}
for raw in open(sys.argv[1], encoding="utf-8"):
    if "=" in raw and not raw.lstrip().startswith("#"):
        name, value = raw.rstrip("\r\n").split("=", 1)
        values[name] = value
domains = (
    ("SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID", "SUB2API_DATA_ENCRYPTION_KEYS_JSON"),
    ("SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID", "SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON"),
    ("SUB2API_BACKUP_ENCRYPTION_ACTIVE_KEY_ID", "SUB2API_BACKUP_ENCRYPTION_KEYS_JSON"),
)
material = []
for active_name, keys_name in domains:
    active = values[active_name]
    keys = json.loads(values[keys_name])
    decoded = base64.b64decode(keys[active], validate=True)
    if len(decoded) != 32:
        raise SystemExit(1)
    material.append(decoded)
if len(set(material)) != 3:
    raise SystemExit(1)
PY
python3 - "${ENV_FILE}" <<'PY'
import pathlib, sys
path = pathlib.Path(sys.argv[1])
text = path.read_text(encoding="utf-8")
text = text.replace(
    "APPLE_CONTAINER_SUB2API_IMAGE=ghcr.io/immortal-autumn/sub2api2personal@sha256:REPLACE_WITH_RELEASE_DIGEST",
    "APPLE_CONTAINER_SUB2API_IMAGE=ghcr.io/immortal-autumn/sub2api2personal@sha256:" + "a" * 64,
)
path.write_text(text, encoding="utf-8")
path.chmod(0o600)
PY

chmod 644 "${ENV_FILE}"
if "${SCRIPT}" up >/dev/null 2>&1; then
    fail "up accepted an insecure env file"
fi
chmod 600 "${ENV_FILE}"

"${SCRIPT}" up
assert_exists "${STATE_DIR}/containers/sub2api-apple"
assert_exists "${STATE_DIR}/containers/sub2api-apple-postgres"
assert_exists "${STATE_DIR}/containers/sub2api-apple-redis"
assert_exists "${STATE_DIR}/running/sub2api-apple"
"${SCRIPT}" status >/dev/null

"${SCRIPT}" up --recreate
assert_exists "${STATE_DIR}/running/sub2api-apple"
"${SCRIPT}" down
assert_missing "${STATE_DIR}/running/sub2api-apple"
assert_missing "${STATE_DIR}/running/sub2api-apple-postgres"
assert_missing "${STATE_DIR}/running/sub2api-apple-redis"

"${SCRIPT}" destroy --yes
assert_missing "${STATE_DIR}/containers/sub2api-apple"
assert_missing "${STATE_DIR}/networks/sub2api-apple"
assert_exists "${STATE_DIR}/volumes/sub2api-apple-data"

"${SCRIPT}" up
"${SCRIPT}" destroy --volumes --yes
assert_missing "${STATE_DIR}/volumes/sub2api-apple-data"
assert_missing "${STATE_DIR}/volumes/sub2api-apple-postgres-data"
assert_missing "${STATE_DIR}/volumes/sub2api-apple-redis-data"

touch "${STATE_DIR}/system-running"
touch "${STATE_DIR}/containers/sub2api-apple"
touch "${STATE_DIR}/unowned/container/sub2api-apple"
if "${SCRIPT}" status >/dev/null 2>&1; then
    fail "status accepted an unowned same-name container"
fi

printf 'Apple container lifecycle tests passed.\n'
