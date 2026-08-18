#!/usr/bin/env bash
# Render Compose and reject every registry image that is not sha256-pinned.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT_DIR"
command -v docker >/dev/null 2>&1 || { printf 'docker is required\n' >&2; exit 1; }
compose_args=()
[[ -z "${COMPOSE_ENV_FILE:-}" ]] || compose_args+=(--env-file "$COMPOSE_ENV_FILE")
rendered=$(docker compose "${compose_args[@]}" "$@" config --format json)
COMPOSE_CONFIG_JSON="$rendered" REQUIRE_INTERNAL_NETWORK="${REQUIRE_INTERNAL_NETWORK:-false}" python3 - <<'PY'
import json, os, re
config = json.loads(os.environ["COMPOSE_CONFIG_JSON"])
digest = re.compile(r"^[^\s@]+@sha256:[0-9a-f]{64}$")
for name, service in config.get("services", {}).items():
    image = service.get("image")
    if image is not None and not digest.fullmatch(image):
        raise SystemExit(f"service {name} image is not immutable: {image!r}")
if os.environ["REQUIRE_INTERNAL_NETWORK"].lower() == "true":
    networks = config.get("networks", {})
    internal = {name for name, value in networks.items() if value.get("internal") is True}
    if not internal:
        raise SystemExit("no internal network is configured")
    for name, service in config.get("services", {}).items():
        attached = set(service.get("networks", {}))
        if not attached or not attached <= internal:
            raise SystemExit(f"service {name} is attached to an egress-capable network")
PY
printf 'immutable Compose contract: pass\n'
