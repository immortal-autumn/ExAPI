#!/usr/bin/env bash
set -euo pipefail
umask 077

SRC=${SRC:-/home/opc/src/exapi-release-3bc462731e1d}
: "${PRODUCTION_ENV:?PRODUCTION_ENV must identify the reviewed production environment}"
CANARY_ENV=${CANARY_ENV:-/protected/exapi-canary-synthetic.env}
REPORT_KEY=${REPORT_KEY:-/protected/keys/exapi-canary-synthetic-report.key}
PROOF_FILE=${PROOF_FILE:-/protected/monitoring/synthetic-provider-proof.json}
ROLLOUT_ID=${ROLLOUT_ID:-exapi-v021-synthetic-20260819a}
RUNTIME_DIR=/protected/synthetic-runtime/$ROLLOUT_ID
: "${CONFIRMATION_TOKEN:?CONFIRMATION_TOKEN is required}"
[[ "$CONFIRMATION_TOKEN" == DROP-SAAS-DATA-KEEP-USER-1 ]] || {
  printf 'synthetic canary setup failed: confirmation token mismatch\n' >&2; exit 1;
}
IMAGE=${IMAGE:-ghcr.io/immortal-autumn/sub2api2personal@sha256:53d8032bfaa812c0fc84ec4f57741317a6e01906b8afafd6f0c4d8e332c6736f}
PROVIDER_IMAGE=${PROVIDER_IMAGE:-python:3.12-alpine@sha256:d09d15e60962ca365d1cd544a48773bac9d33f2fb1b00f2aa0deec78ade7dc31}
OVERLAY=$SRC/tmp/infrastructure/docker-compose.synthetic.yml
GENERATOR=$SRC/tmp/infrastructure/create-synthetic-env.py
PROVIDER_SOURCE=$SRC/tmp/infrastructure/mock-provider.py
COMPOSE_FILES=(-f "$SRC/deploy/docker-compose.yml" -f "$OVERLAY")

die() { printf 'synthetic canary setup failed: %s\n' "$*" >&2; exit 1; }
env_value() { sed -n "s/^$1=//p" "$CANARY_ENV"; }
[[ $(id -u) -eq 0 ]] || die 'run as root'
for command_name in curl docker jq python3 sha256sum; do
  command -v "$command_name" >/dev/null 2>&1 || die "$command_name is required"
done
docker compose version >/dev/null 2>&1 || die 'Docker Compose v2 is required'
[[ -r "$PRODUCTION_ENV" && -x "$GENERATOR" && -r "$OVERLAY" && -r "$PROVIDER_SOURCE" ]] || die 'required input is missing'
[[ "$IMAGE" =~ @sha256:[0-9a-f]{64}$ ]] || die 'IMAGE must be digest pinned'
[[ "$PROVIDER_IMAGE" =~ @sha256:[0-9a-f]{64}$ ]] || die 'PROVIDER_IMAGE must be digest pinned'
[[ ! -e "$CANARY_ENV" && ! -e "$REPORT_KEY" && ! -e "$PROOF_FILE" && ! -e "$RUNTIME_DIR" ]] || die 'protected synthetic inputs already exist'

setup_complete=false
cleanup_failed_setup() {
  local rc=$?
  if [[ $rc -ne 0 && "$setup_complete" != true ]]; then
    local project=${PROJECT:-} app=${APP:-} postgres=${POSTGRES:-} redis=${REDIS:-} provider=${PROVIDER:-}
    if [[ -z "$project" && -r "$CANARY_ENV" ]]; then
      project=$(env_value COMPOSE_PROJECT_NAME || true)
      app=$(env_value EXAPI_CONTAINER_NAME || true)
      postgres=$(env_value EXAPI_POSTGRES_CONTAINER_NAME || true)
      redis=$(env_value EXAPI_REDIS_CONTAINER_NAME || true)
      provider=$(env_value EXAPI_PROVIDER_CONTAINER_NAME || true)
    fi
    [[ -z "$app" ]] || docker rm -f "$app" >/dev/null 2>&1 || true
    [[ -z "$postgres" ]] || docker rm -f "$postgres" >/dev/null 2>&1 || true
    [[ -z "$redis" ]] || docker rm -f "$redis" >/dev/null 2>&1 || true
    [[ -z "$provider" ]] || docker rm -f "$provider" >/dev/null 2>&1 || true
    [[ -z "${secret_volume:-}" ]] || docker volume rm -f "$secret_volume" >/dev/null 2>&1 || true
    if [[ -n "$project" && "$project" == exapi-syn-* ]]; then
      docker network rm "${project}_sub2api-network" >/dev/null 2>&1 || true
      for volume in "${project}_sub2api_data" "${project}_postgres_data" "${project}_redis_data"; do
        docker volume rm -f "$volume" >/dev/null 2>&1 || true
      done
    fi
    rm -f "$CANARY_ENV" "$REPORT_KEY" "$PROOF_FILE"
    rm -f "$RUNTIME_DIR/gateway-key" "$RUNTIME_DIR/mock-provider.py"
    rmdir "$RUNTIME_DIR" >/dev/null 2>&1 || true
  fi
  exit "$rc"
}
trap cleanup_failed_setup EXIT HUP INT TERM

install -d -o root -g root -m 0700 "$RUNTIME_DIR"
install -o root -g root -m 0644 "$PROVIDER_SOURCE" "$RUNTIME_DIR/mock-provider.py"
PROVIDER_SOURCE="$RUNTIME_DIR/mock-provider.py"

python3 "$GENERATOR" "$PRODUCTION_ENV" "$CANARY_ENV" "$IMAGE" "$PROVIDER_IMAGE" "$ROLLOUT_ID"
chown root:root "$CANARY_ENV"
chmod 600 "$CANARY_ENV"
python3 - "$REPORT_KEY" <<'PY'
import os, secrets, sys
fd=os.open(sys.argv[1], os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, "w", encoding="ascii") as handle:
    handle.write(secrets.token_hex(32) + "\n")
PY
chown root:root "$REPORT_KEY"
chmod 600 "$REPORT_KEY"

PROJECT=$(env_value COMPOSE_PROJECT_NAME)
APP=$(env_value EXAPI_CONTAINER_NAME)
POSTGRES=$(env_value EXAPI_POSTGRES_CONTAINER_NAME)
REDIS=$(env_value EXAPI_REDIS_CONTAINER_NAME)
PROVIDER=$(env_value EXAPI_PROVIDER_CONTAINER_NAME)
NETWORK=${PROJECT}_sub2api-network
[[ "$PROJECT" =~ ^exapi-syn-[a-z0-9-]+$ ]] || die 'generated project name is unsafe'
for name in "$APP" "$POSTGRES" "$REDIS" "$PROVIDER"; do
  [[ "$name" == "$PROJECT"-* ]] || die "generated container name is outside project: $name"
  ! docker container inspect "$name" >/dev/null 2>&1 || die "$name already exists"
done
! docker network inspect "$NETWORK" >/dev/null 2>&1 || die "$NETWORK already exists"
for volume in "${PROJECT}_sub2api_data" "${PROJECT}_postgres_data" "${PROJECT}_redis_data"; do
  ! docker volume inspect "$volume" >/dev/null 2>&1 || die "$volume already exists"
done

compose=(docker compose --project-directory "$SRC" --env-file "$CANARY_ENV" -p "$PROJECT" "${COMPOSE_FILES[@]}")
COMPOSE_ENV_FILE="$CANARY_ENV" REQUIRE_INTERNAL_NETWORK=true \
  "$SRC/deploy/ops/validate-immutable-compose.sh" --project-directory "$SRC" -p "$PROJECT" "${COMPOSE_FILES[@]}" >/dev/null

config_json=$(mktemp "$SRC/tmp/synthetic-compose-config.XXXXXX")
"${compose[@]}" config --format json >"$config_json"
PROJECT="$PROJECT" APP="$APP" POSTGRES="$POSTGRES" REDIS="$REDIS" PROVIDER="$PROVIDER" \
IMAGE="$IMAGE" PROVIDER_IMAGE="$PROVIDER_IMAGE" python3 - "$config_json" <<'PY'
import json, os, sys
c=json.load(open(sys.argv[1], encoding="utf-8"))
assert c["name"] == os.environ["PROJECT"]
assert c["networks"]["sub2api-network"]["internal"] is True
s=c["services"]
assert set(s) == {"sub2api","postgres","redis","mock-provider"}
assert s["sub2api"]["image"] == os.environ["IMAGE"]
assert s["mock-provider"]["image"] == os.environ["PROVIDER_IMAGE"]
assert s["sub2api"]["container_name"] == os.environ["APP"]
assert s["postgres"]["container_name"] == os.environ["POSTGRES"]
assert s["redis"]["container_name"] == os.environ["REDIS"]
assert s["mock-provider"]["container_name"] == os.environ["PROVIDER"]
for service in s.values():
    assert set(service.get("networks", {})) == {"sub2api-network"}
    for port in service.get("ports", []):
        assert port.get("host_ip") in (None, "127.0.0.1")
for volume in c["volumes"].values():
    assert not volume.get("external", False)
PY
rm -f "$config_json"

"${compose[@]}" pull postgres redis mock-provider >/dev/null
"${compose[@]}" up -d postgres redis mock-provider >/dev/null
for _ in $(seq 1 90); do
  pg_health=$(docker inspect "$POSTGRES" --format '{{if .State.Health}}{{.State.Health.Status}}{{end}}' 2>/dev/null || true)
  redis_health=$(docker inspect "$REDIS" --format '{{if .State.Health}}{{.State.Health.Status}}{{end}}' 2>/dev/null || true)
  provider_health=$(docker inspect "$PROVIDER" --format '{{if .State.Health}}{{.State.Health.Status}}{{end}}' 2>/dev/null || true)
  [[ "$pg_health" == healthy && "$redis_health" == healthy && "$provider_health" == healthy ]] && break
  sleep 1
done
[[ "$pg_health" == healthy ]] || die 'synthetic PostgreSQL is not healthy'
[[ "$redis_health" == healthy ]] || die 'synthetic Redis is not healthy'
[[ "$provider_health" == healthy ]] || die 'synthetic provider is not healthy'
[[ $(docker network inspect "$NETWORK" --format '{{.Internal}}') == true ]] || die 'synthetic network is not internal'

"${compose[@]}" up -d --no-deps sub2api >/dev/null
for _ in $(seq 1 90); do
  users=$(docker exec "$POSTGRES" sh -ec 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT COUNT(*) FROM users"' 2>/dev/null || true)
  [[ "$users" == 1 ]] && break
  sleep 2
done
[[ "$users" == 1 ]] || die 'bootstrap did not create exactly one operator'
"${compose[@]}" stop -t 60 sub2api >/dev/null

evidence_dir="$SRC/tmp/rollouts/$ROLLOUT_ID/synthetic-provider"
mkdir -p "$evidence_dir"
chmod 700 "$evidence_dir"
batch_evidence="$evidence_dir/batch-cleanup-evidence.json"
EXAPI_ROLLOUT_ID="$ROLLOUT_ID" EXAPI_POSTGRES_CONTAINER_NAME="$POSTGRES" \
  /protected/adapters/verify-batch-cleanup >"$batch_evidence"
chmod 600 "$batch_evidence"
jq -e '.verified == true and .sql_rows_remaining == 0 and .provider_jobs_remaining == 0 and .provider_inputs_remaining == 0 and .provider_outputs_remaining == 0' "$batch_evidence" >/dev/null

secret_volume=$(docker volume create --label "exapi.rollout_id=$ROLLOUT_ID")
cleanup_secret() { docker volume rm -f "$secret_volume" >/dev/null 2>&1 || true; }
docker run --rm --network none --entrypoint /bin/sh \
  -v "$REPORT_KEY:/source/report.key:ro" \
  -v "$batch_evidence:/source/batch-cleanup-evidence.json:ro" \
  -v "$secret_volume:/target" \
  "$IMAGE" -ec '
    cp /source/report.key /target/report.key
    cp /source/batch-cleanup-evidence.json /target/batch-cleanup-evidence.json
    chown 1000:1000 /target/report.key /target/batch-cleanup-evidence.json
    chmod 0600 /target/report.key /target/batch-cleanup-evidence.json
  '

"${compose[@]}" run --rm --no-deps -T \
  -v "$secret_volume:/run/exapi-private-cutover:ro" \
  sub2api /app/with-migration-report-key.sh /run/exapi-private-cutover/report.key \
    /app/migrate-private-only \
    --confirm "$CONFIRMATION_TOKEN" \
    --no-local-backups \
    --batch-cleanup-evidence-file /run/exapi-private-cutover/batch-cleanup-evidence.json \
    --report-file /app/data/private-migration-report.json

"${compose[@]}" run --rm --no-deps -T \
  -v "$secret_volume:/run/exapi-private-cutover:ro" \
  sub2api /app/with-migration-report-key.sh /run/exapi-private-cutover/report.key \
    /app/verify-private-cutover-report \
    --report-file /app/data/private-migration-report.json \
    --evidence-file /app/data/private-migration-evidence.json
cleanup_secret
secret_volume=

"${compose[@]}" up -d --no-deps sub2api >/dev/null
for _ in $(seq 1 90); do
  [[ "$(docker inspect "$APP" --format '{{.State.Health.Status}}' 2>/dev/null || true)" == healthy ]] && break
  sleep 2
done
[[ $(docker inspect "$APP" --format '{{.State.Health.Status}}') == healthy ]] || die 'synthetic app did not become healthy'
docker exec "$APP" wget -q -T 5 -O /tmp/ready.json http://localhost:8080/ready >/dev/null
docker exec "$APP" grep -q '"status"' /tmp/ready.json || die 'synthetic readiness response is not JSON'

provider_key=$(env_value SYNTHETIC_UPSTREAM_KEY)
[[ "$provider_key" == exapi-synthetic-* ]] || die 'synthetic provider credential is invalid'
group_payload='{"name":"synthetic-openai-only","description":"isolated synthetic provider canary","platform":"openai","subscription_type":"standard","rate_multiplier":1}'
group_response=$(docker exec "$APP" wget -q -T 15 -O - --header='Content-Type: application/json' \
  --header='X-ExAPI-Control-Request: 1' --header='Origin: http://localhost:8027' \
  --header='Sec-Fetch-Site: same-origin' --header='Sec-Fetch-Mode: cors' \
  --post-data="$group_payload" http://localhost:8027/api/v1/admin/groups)
group_id=$(GROUP_RESPONSE="$group_response" python3 - <<'PY'
import json, os
d=json.loads(os.environ["GROUP_RESPONSE"]); d=d.get("data", d)
assert isinstance(d.get("id"), int) and d.get("platform") == "openai"
print(d["id"])
PY
)

account_payload=$(PROVIDER_KEY="$provider_key" GROUP_ID="$group_id" python3 - <<'PY'
import json, os
print(json.dumps({
  "name":"synthetic-openai-provider","platform":"openai","type":"apikey",
  "credentials":{
    "api_key":os.environ["PROVIDER_KEY"],
    "base_url":"http://mock-provider:19091/v1",
    "model_mapping":{"synthetic-model":"synthetic-model"},
  },
  "extra":{"openai_responses_mode":"force_chat_completions"},
  "concurrency":2,"priority":100,"rate_multiplier":0,
  "group_ids":[int(os.environ["GROUP_ID"])],
}, separators=(",", ":")))
PY
)
account_response=$(docker exec "$APP" wget -q -T 15 -O - --header='Content-Type: application/json' \
  --header='X-ExAPI-Control-Request: 1' --header='Origin: http://localhost:8027' \
  --header='Sec-Fetch-Site: same-origin' --header='Sec-Fetch-Mode: cors' \
  --post-data="$account_payload" http://localhost:8027/api/v1/admin/accounts)
account_id=$(ACCOUNT_RESPONSE="$account_response" GROUP_ID="$group_id" python3 - <<'PY'
import json, os
d=json.loads(os.environ["ACCOUNT_RESPONSE"]); d=d.get("data", d)
assert isinstance(d.get("id"), int) and d.get("platform") == "openai" and d.get("type") == "apikey"
assert d.get("group_ids") == [int(os.environ["GROUP_ID"])]
print(d["id"])
PY
)
key_payload=$(GROUP_ID="$group_id" python3 - <<'PY'
import json, os
print(json.dumps({"name":"synthetic-gateway-smoke","group_id":int(os.environ["GROUP_ID"])}, separators=(",", ":")))
PY
)
key_response=$(docker exec "$APP" wget -q -T 10 -O - --header='Content-Type: application/json' \
  --header='X-ExAPI-Control-Request: 1' --header='Origin: http://localhost:8027' \
  --header='Sec-Fetch-Site: same-origin' --header='Sec-Fetch-Mode: cors' \
  --post-data="$key_payload" http://localhost:8027/api/v1/keys)
gateway_key=$(KEY_RESPONSE="$key_response" python3 - <<'PY'
import json, os
d=json.loads(os.environ["KEY_RESPONSE"]); d=d.get("data", d)
value=d.get("key")
assert isinstance(value, str) and len(value) >= 16
print(value)
PY
)
install -d -o root -g root -m 700 "$RUNTIME_DIR"
gateway_key_tmp=$(mktemp "$RUNTIME_DIR/gateway-key.XXXXXX")
printf '%s\n' "$gateway_key" >"$gateway_key_tmp"
install -o root -g root -m 600 "$gateway_key_tmp" "$RUNTIME_DIR/gateway-key"
rm -f "$gateway_key_tmp"

smoke_payload='{"model":"synthetic-model","messages":[{"role":"user","content":"synthetic deployment probe"}],"stream":false}'
smoke_response=$(docker exec "$APP" wget -q -T 20 -O - \
  --header='Content-Type: application/json' --header="Authorization: Bearer $gateway_key" \
  --post-data="$smoke_payload" http://localhost:8080/v1/chat/completions)
SMOKE_RESPONSE="$smoke_response" python3 - <<'PY'
import json, os
d=json.loads(os.environ["SMOKE_RESPONSE"])
assert d["choices"][0]["message"]["content"] == "synthetic-ok"
PY
provider_stats=$(docker exec "$PROVIDER" python3 -c 'import urllib.request; print(urllib.request.urlopen("http://127.0.0.1:19091/stats", timeout=5).read().decode())')
PROVIDER_STATS="$provider_stats" ROLLOUT_ID="$ROLLOUT_ID" PROVIDER_KEY="$provider_key" python3 - <<'PY'
import hashlib, json, os
d=json.loads(os.environ["PROVIDER_STATS"])
assert d["rollout_id"] == os.environ["ROLLOUT_ID"]
assert d["expected_token_sha256"] == hashlib.sha256(os.environ["PROVIDER_KEY"].encode()).hexdigest()
assert d["chat_completions"] >= 1 and d["auth_failures"] == 0
assert d["last_request"]["authorization_sha256"] == d["expected_token_sha256"]
PY

if docker exec "$APP" sh -ec 'wget -q -T 3 -O /dev/null https://example.com' >/dev/null 2>&1; then
  die 'synthetic canary unexpectedly reached the public internet'
fi
docker exec "$APP" wget -q -T 3 -O /dev/null http://mock-provider:19091/health || die 'synthetic provider is not reachable from app'

counts=$(docker exec "$POSTGRES" sh -ec 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -AtF "|" -c "SELECT (SELECT private_schema_version FROM exapi_private_state WHERE id=true),(SELECT COUNT(*) FROM users),(SELECT COUNT(*) FROM groups WHERE deleted_at IS NULL),(SELECT COUNT(*) FROM accounts WHERE deleted_at IS NULL),(SELECT COUNT(*) FROM api_keys WHERE deleted_at IS NULL),(SELECT COUNT(*) FROM batch_image_jobs)"')
[[ "$counts" == '2|1|2|1|1|0' ]] || die "unexpected synthetic database counts: $counts"
bindings=$(docker exec "$POSTGRES" sh -ec 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -AtF "|" -v account_id="$1" -v group_id="$2" -c "SELECT (SELECT COUNT(*) FROM account_groups WHERE account_id=:'\''account_id'\'' AND group_id=:'\''group_id'\''),(SELECT COUNT(*) FROM api_keys WHERE group_id=:'\''group_id'\'' AND deleted_at IS NULL)"' sh "$account_id" "$group_id")
[[ "$bindings" == '1|1' ]] || die "synthetic account/key group bindings are incomplete: $bindings"

mkdir -p "$(dirname "$PROOF_FILE")"
proof_tmp=$(mktemp "${PROOF_FILE}.XXXXXX")
chmod 600 "$proof_tmp"
PRODUCTION_ENV="$PRODUCTION_ENV" CANARY_ENV="$CANARY_ENV" IMAGE="$IMAGE" PROVIDER_IMAGE="$PROVIDER_IMAGE" \
NETWORK="$NETWORK" APP="$APP" POSTGRES="$POSTGRES" REDIS="$REDIS" PROVIDER="$PROVIDER" \
ROLLOUT_ID="$ROLLOUT_ID" PROVIDER_STATS="$provider_stats" SMOKE_RESPONSE="$smoke_response" \
GATEWAY_KEY="$gateway_key" PROVIDER_SOURCE="$PROVIDER_SOURCE" GROUP_ID="$group_id" \
ACCOUNT_ID="$account_id" python3 - "$proof_tmp" <<'PY'
import datetime, hashlib, json, os, subprocess, sys

def env(path):
    out={}
    for raw in open(path, encoding="utf-8"):
        raw=raw.rstrip("\n")
        if "=" in raw and not raw.lstrip().startswith("#"):
            key, value=raw.split("=", 1); out[key]=value
    return out

production=env(os.environ["PRODUCTION_ENV"]); canary=env(os.environ["CANARY_ENV"])
sensitive=(
    "POSTGRES_PASSWORD","REDIS_PASSWORD","ADMIN_EMAIL","ADMIN_PASSWORD","JWT_SECRET",
    "TOTP_ENCRYPTION_KEY","SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID",
    "SUB2API_DATA_ENCRYPTION_KEYS_JSON","SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID",
    "SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON",
)
for key in sensitive:
    if key in production and production[key] and production[key] == canary.get(key):
        raise SystemExit(f"synthetic environment reused production field {key}")
for key in ("POSTGRES_IMAGE","REDIS_IMAGE"):
    if not canary.get(key) or canary[key] != production.get(key):
        raise SystemExit(f"synthetic environment did not inherit reviewed {key}")

def docker_json(*args):
    return json.loads(subprocess.check_output(["docker", *args], text=True))

network=docker_json("network","inspect",os.environ["NETWORK"])[0]
assert network["Internal"] is True
containers=sorted(item["Name"] for item in network["Containers"].values())
expected=sorted([os.environ["APP"],os.environ["POSTGRES"],os.environ["REDIS"],os.environ["PROVIDER"]])
assert containers == expected
container_evidence={}
for name in containers:
    inspect=docker_json("inspect",name)[0]
    assert list(inspect["NetworkSettings"]["Networks"]) == [os.environ["NETWORK"]]
    container_evidence[name]={
      "id":inspect["Id"],"image_id":inspect["Image"],"started_at":inspect["State"]["StartedAt"],
      "restart_count":inspect["RestartCount"],"networks":sorted(inspect["NetworkSettings"]["Networks"]),
    }
    assert inspect["RestartCount"] == 0

prod_mounts=set()
for name in ("sub2api","sub2api-postgres","sub2api-redis"):
    inspect=docker_json("inspect",name)[0]
    prod_mounts.update((item.get("Name") or item["Source"]) for item in inspect["Mounts"])
canary_mounts=set()
for name in expected:
    inspect=docker_json("inspect",name)[0]
    canary_mounts.update((item.get("Name") or item["Source"]) for item in inspect["Mounts"] if item["Type"] == "volume")
assert canary_mounts.isdisjoint(prod_mounts)

stats=json.loads(os.environ["PROVIDER_STATS"]); smoke=json.loads(os.environ["SMOKE_RESPONSE"])
revision=subprocess.check_output([
    "docker","image","inspect",os.environ["IMAGE"],"--format",
    '{{ index .Config.Labels "org.opencontainers.image.revision" }}'
], text=True).strip()
document={
  "schema_version":1,"verified":True,
  "verified_at":datetime.datetime.now(datetime.timezone.utc).isoformat().replace("+00:00","Z"),
  "rollout_id":os.environ["ROLLOUT_ID"],"image":os.environ["IMAGE"],
  "image_digest":"sha256:"+os.environ["IMAGE"].rsplit("@sha256:",1)[1],"image_revision":revision,
  "provider_image":os.environ["PROVIDER_IMAGE"],"production_data":False,
  "canary_only_credentials":True,"production_keyroots_reused":False,
  "compared_sensitive_field_count":len(sensitive),"source_fields_reused":["POSTGRES_IMAGE","REDIS_IMAGE"],
  "database":{"private_schema_version":2,"users":1,"groups":2,"accounts":1,"active_api_keys":1,
    "batch_jobs":0,"synthetic_group_id":int(os.environ["GROUP_ID"]),
    "synthetic_account_id":int(os.environ["ACCOUNT_ID"]),"account_group_bindings":1},
  "network":{"name":os.environ["NETWORK"],"id":network["Id"],"internal":True,
    "containers":container_evidence,"public_internet_denied":True,"production_mounts_shared":False},
  "egress_control":{"kind":"docker_internal_network","allowed_services":["mock-provider","postgres","redis"]},
  "egress_allowlist_verified":True,"provider_smoke_verified":True,
  "provider":{"kind":"openai-compatible","endpoint":"http://mock-provider:19091/v1","stats":stats,
    "source_sha256":hashlib.sha256(open(os.environ["PROVIDER_SOURCE"],"rb").read()).hexdigest(),
    "upstream_key_sha256":hashlib.sha256(canary["SYNTHETIC_UPSTREAM_KEY"].encode()).hexdigest(),
    "gateway_key_sha256":hashlib.sha256(os.environ["GATEWAY_KEY"].encode()).hexdigest(),
    "response_sha256":hashlib.sha256(json.dumps(smoke,sort_keys=True,separators=(",", ":")).encode()).hexdigest()},
}
with open(sys.argv[1],"w",encoding="utf-8") as handle:
    json.dump(document,handle,indent=2,sort_keys=True); handle.write("\n")
PY
install -o root -g root -m 600 "$proof_tmp" "$PROOF_FILE"
rm -f "$proof_tmp"

setup_complete=true
trap - EXIT HUP INT TERM
printf 'synthetic_canary=ready\n'
printf 'rollout_id=%s\n' "$ROLLOUT_ID"
printf 'project=%s\n' "$PROJECT"
printf 'app=%s\n' "$APP"
printf 'image_digest=sha256:%s\n' "${IMAGE##*@sha256:}"
printf 'network=%s\n' "$NETWORK"
printf 'proof_file=%s\n' "$PROOF_FILE"
printf 'evidence_dir=%s\n' "$evidence_dir"
