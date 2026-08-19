#!/usr/bin/env python3
import base64
import json
import os
import pathlib
import re
import secrets
import sys

if len(sys.argv) != 6:
    raise SystemExit(
        "usage: create-synthetic-env.py SOURCE_ENV TARGET_ENV EXAPI_IMAGE "
        "PROVIDER_IMAGE ROLLOUT_ID"
    )

source = pathlib.Path(sys.argv[1])
target = pathlib.Path(sys.argv[2])
image = sys.argv[3]
provider_image = sys.argv[4]
rollout_id = sys.argv[5]

digest_reference = re.compile(r"^[^\s]+@sha256:[0-9a-f]{64}$")
if not digest_reference.fullmatch(image):
    raise SystemExit("application image must be pinned by sha256 digest")
if not digest_reference.fullmatch(provider_image):
    raise SystemExit("provider image must be pinned by sha256 digest")
if not re.fullmatch(r"[a-z0-9][a-z0-9_-]{7,62}", rollout_id):
    raise SystemExit("rollout ID must be 8-63 lowercase letters, digits, '_' or '-'")

source_values = {}
for raw in source.read_text(encoding="utf-8").splitlines():
    if "=" not in raw or raw.lstrip().startswith("#"):
        continue
    name, value = raw.split("=", 1)
    source_values[name] = value

for required_image in ("POSTGRES_IMAGE", "REDIS_IMAGE"):
    if not digest_reference.fullmatch(source_values.get(required_image, "")):
        raise SystemExit(f"{required_image} must be pinned by sha256 digest")

def key():
    return base64.b64encode(secrets.token_bytes(32)).decode()

suffix = rollout_id[-24:]
project = f"exapi-syn-{suffix}".replace("_", "-")
upstream_key = "exapi-synthetic-" + secrets.token_urlsafe(32)

values = {
    "COMPOSE_PROJECT_NAME": project,
    "SYNTHETIC_ROLLOUT_ID": rollout_id,
    "EXAPI_IMAGE": image,
    "SYNTHETIC_PROVIDER_IMAGE": provider_image,
    "SYNTHETIC_PROVIDER_SOURCE": f"/protected/synthetic-runtime/{rollout_id}/mock-provider.py",
    "SYNTHETIC_PROVIDER_ARM64_DIGEST": "sha256:c95cd47204b8f236725fc8cf94726abe3f32755a062393597efadd9a5d24fbe1",
    "EXAPI_CONTAINER_NAME": f"{project}-app",
    "EXAPI_POSTGRES_CONTAINER_NAME": f"{project}-postgres",
    "EXAPI_REDIS_CONTAINER_NAME": f"{project}-redis",
    "EXAPI_PROVIDER_CONTAINER_NAME": f"{project}-provider",
    "BIND_HOST": "127.0.0.1",
    "SERVER_PORT": "18081",
    "EXAPI_PUBLIC_LISTEN_ADDR": "0.0.0.0:8080",
    "EXAPI_CONTROL_BIND_HOST": "127.0.0.1",
    "EXAPI_CONTROL_PORT": "18028",
    "EXAPI_CONTROL_LISTEN_ADDR": "0.0.0.0:8027",
    "EXAPI_CONTROL_HOSTS": "localhost,127.0.0.1,::1",
    "EXAPI_OPERATOR_PEER_IPS": "127.0.0.1,::1",
    "SUB2API_PUBLIC_HOST": "127.0.0.1",
    "SUB2API_PRIVATE_CONTROL_HOSTS": "localhost,127.0.0.1,::1",
    "SUB2API_PRIVATE_CONTROL_CIDRS": "127.0.0.0/8,::1/128",
    "SUB2API_LOCAL_ADMIN_BYPASS": "true",
    "SUB2API_LOCAL_ADMIN_BYPASS_CIDRS": "127.0.0.0/8,::1/128",
    "SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE": "true",
    "SERVER_MODE": "release",
    "RUN_MODE": "standard",
    "POSTGRES_IMAGE": source_values["POSTGRES_IMAGE"],
    "REDIS_IMAGE": source_values["REDIS_IMAGE"],
    "POSTGRES_USER": "sub2api",
    "POSTGRES_DB": "exapi_synthetic",
    "POSTGRES_PASSWORD": secrets.token_hex(32),
    "REDIS_PASSWORD": secrets.token_hex(32),
    "ADMIN_EMAIL": "synthetic-operator@invalid.example",
    "ADMIN_PASSWORD": secrets.token_urlsafe(32),
    "JWT_SECRET": secrets.token_hex(32),
    "TOTP_ENCRYPTION_KEY": secrets.token_hex(32),
    "SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID": "synthetic-data-2026",
    "SUB2API_DATA_ENCRYPTION_KEYS_JSON": json.dumps({"synthetic-data-2026": key()}, separators=(",", ":")),
    "SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID": "synthetic-digest-2026",
    "SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON": json.dumps({"synthetic-digest-2026": key()}, separators=(",", ":")),
    "SUB2API_BACKUP_ENCRYPTION_ACTIVE_KEY_ID": "",
    "SUB2API_BACKUP_ENCRYPTION_KEYS_JSON": "",
    "SUB2API_ALLOW_LEGACY_PLAINTEXT_BACKUP_RESTORE": "false",
    "SUB2API_MIGRATE_LEGACY_SECURITY_SECRETS": "false",
    "SECURITY_URL_ALLOWLIST_ENABLED": "false",
    "SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP": "true",
    "SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS": "true",
    "SECURITY_URL_ALLOWLIST_UPSTREAM_HOSTS": "",
    "SECURITY_URL_ALLOWLIST_PRICING_HOSTS": "",
    "SECURITY_URL_ALLOWLIST_CRS_HOSTS": "",
    "UPDATE_PROXY_URL": "",
    "SYNTHETIC_UPSTREAM_KEY": upstream_key,
    "TZ": "UTC",
}

lines = [f"{name}={value}" for name, value in values.items()]

flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL
fd = os.open(target, flags, 0o600)
with os.fdopen(fd, "w", encoding="utf-8") as handle:
    handle.write("\n".join(lines) + "\n")
