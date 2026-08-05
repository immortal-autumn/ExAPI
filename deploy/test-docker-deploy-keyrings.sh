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

required = ("JWT_SECRET", "TOTP_ENCRYPTION_KEY", "POSTGRES_PASSWORD")
if any(not values.get(name) for name in required):
    raise SystemExit("state-coupled credential is empty")
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
GITHUB_RAW_URL="$LOCAL_DEPLOY_URL" bash "$ROOT_DIR/deploy/docker-deploy.sh" >first-run.log
first_snapshot=$(snapshot)

printf 'y\n' | GITHUB_RAW_URL="$LOCAL_DEPLOY_URL" bash "$ROOT_DIR/deploy/docker-deploy.sh" >second-run.log
second_snapshot=$(snapshot)

[ "$first_snapshot" = "$second_snapshot" ] || fail 'redeploy rotated a state-coupled credential or keyring'
printf 'Docker deployment generates and preserves independent keyrings.\n'
