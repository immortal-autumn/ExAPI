#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
LOCAL_DEPLOY_URL=$(python3 - "$ROOT_DIR/deploy" <<'PY'
import pathlib, sys
print(pathlib.Path(sys.argv[1]).resolve().as_uri())
PY
)
mkdir -p "$ROOT_DIR/tmp"
WORK_DIR=$(mktemp -d "$ROOT_DIR/tmp/docker-deploy-keyrings.XXXXXX")
trap 'rm -rf "$WORK_DIR"' EXIT

fail() {
    printf 'Docker deployment keyring test failed: %s\n' "$1" >&2
    exit 1
}

snapshot() {
    python3 - .env <<'PY'
import base64, hashlib, json, pathlib, sys
path = pathlib.Path(sys.argv[1])
values = {}
for raw in path.read_text(encoding="utf-8").splitlines():
    if "=" in raw and not raw.lstrip().startswith("#"):
        name, value = raw.split("=", 1)
        values[name] = value.strip().strip("'\"")

domains = (
    ("SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID", "SUB2API_DATA_ENCRYPTION_KEYS_JSON"),
    ("SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID", "SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON"),
    ("SUB2API_BACKUP_ENCRYPTION_ACTIVE_KEY_ID", "SUB2API_BACKUP_ENCRYPTION_KEYS_JSON"),
)
material = []
for active_name, keys_name in domains:
    active = values.get(active_name, "")
    keys = json.loads(values.get(keys_name, ""))
    decoded = base64.b64decode(keys[active] + "=" * (-len(keys[active]) % 4), validate=True)
    if len(decoded) != 32:
        raise SystemExit(f"{keys_name} does not contain a 32-byte active key")
    material.append(decoded)
if len(set(material)) != len(material):
    raise SystemExit("generated cryptographic domains reuse key material")

required = ("JWT_SECRET", "TOTP_ENCRYPTION_KEY", "POSTGRES_PASSWORD", "REDIS_PASSWORD")
if any(not values.get(name) for name in required):
    raise SystemExit("state-coupled credential is empty")
secure_outbound = {
    "SECURITY_OUTBOUND_MODE": "enforce",
    "SECURITY_URL_ALLOWLIST_ENABLED": "true",
    "SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP": "false",
    "SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS": "false",
}
for name, expected in secure_outbound.items():
    if values.get(name) != expected:
        raise SystemExit(f"{name}={values.get(name)!r}, expected {expected!r} for a new install")
if not values.get("SECURITY_URL_ALLOWLIST_UPSTREAM_HOSTS"):
    raise SystemExit("new install outbound upstream allowlist is empty")
mode = path.stat().st_mode & 0o777
if mode != 0o600:
    probe = path.parent / ".mode-probe"
    probe.write_text("probe", encoding="utf-8")
    probe.chmod(0o600)
    supports_posix_modes = (probe.stat().st_mode & 0o777) == 0o600
    probe.unlink()
    if supports_posix_modes:
        raise SystemExit(f".env mode is {mode:o}, expected 600")

tracked = required + tuple(name for pair in domains for name in pair)
digest = hashlib.sha256()
for name in tracked:
    digest.update(name.encode())
    digest.update(b"\0")
    digest.update(values[name].encode())
    digest.update(b"\0")
print(digest.hexdigest())
PY
}

cd "$WORK_DIR"
export EXAPI_IMAGE='ghcr.io/example/exapi@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
export POSTGRES_IMAGE='postgres@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
export REDIS_IMAGE='redis@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc'
GITHUB_RAW_URL="$LOCAL_DEPLOY_URL" bash "$ROOT_DIR/deploy/docker-deploy.sh" >first-run.log
test -f docker-compose.local.yml || fail 'deployment script did not preserve the documented compose filename'
grep -Fqx "EXAPI_IMAGE=$EXAPI_IMAGE" .env || fail 'ExAPI image digest was not written to .env'
grep -Fqx "POSTGRES_IMAGE=$POSTGRES_IMAGE" .env || fail 'PostgreSQL image digest was not written to .env'
grep -Fqx "REDIS_IMAGE=$REDIS_IMAGE" .env || fail 'Redis image digest was not written to .env'
first_snapshot=$(snapshot)

printf 'y\n' | GITHUB_RAW_URL="$LOCAL_DEPLOY_URL" bash "$ROOT_DIR/deploy/docker-deploy.sh" >second-run.log
second_snapshot=$(snapshot)

[ "$first_snapshot" = "$second_snapshot" ] || fail 'redeploy rotated a state-coupled credential or keyring'

invalid_dir=$(mktemp -d "$WORK_DIR/invalid.XXXXXX")
if (cd "$invalid_dir" && env -u EXAPI_IMAGE -u POSTGRES_IMAGE -u REDIS_IMAGE \
    GITHUB_RAW_URL="$LOCAL_DEPLOY_URL" bash "$ROOT_DIR/deploy/docker-deploy.sh" >invalid.log 2>&1); then
    fail 'deployment script accepted mutable/placeholder image references'
fi
grep -Fq 'immutable image reference' "$invalid_dir/invalid.log" || fail 'image validation failure did not explain the required digest format'

printf 'Docker deployment generates and preserves independent keyrings.\n'
