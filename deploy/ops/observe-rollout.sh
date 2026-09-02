#!/usr/bin/env bash
# Observe a digest-pinned canary/production target and fail the documented SLOs.
set -euo pipefail
umask 077

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT_DIR"
die() { printf 'rollout observation failed: %s\n' "$*" >&2; exit 1; }
for name in OBSERVATION_CLASS TARGET_BASE_URL CONTAINER_NAME IMAGE_DIGEST METRICS_COMMAND NETWORK_PROOF_COMMAND; do
  [[ -n "${!name:-}" ]] || die "$name is required"
done
[[ -x "$METRICS_COMMAND" && -x "$NETWORK_PROOF_COMMAND" ]] || die 'metrics and network proof adapters must be executable'
[[ "$IMAGE_DIGEST" =~ ^sha256:[0-9a-f]{64}$ ]] || die 'IMAGE_DIGEST must be a sha256 digest'
case "$OBSERVATION_CLASS" in
  restored-data|synthetic-provider) minimum_seconds=1800 ;;
  production) minimum_seconds=3600 ;;
  *) die 'OBSERVATION_CLASS must be restored-data, synthetic-provider, or production' ;;
esac
duration=${OBSERVATION_DURATION_SECONDS:-$minimum_seconds}
interval=${OBSERVATION_INTERVAL_SECONDS:-30}
(( duration >= minimum_seconds )) || die "observation must last at least $minimum_seconds seconds"
(( interval > 0 && interval <= 30 )) || die 'readiness interval must be between 1 and 30 seconds'
rollout_id=${ROLLOUT_ID:-$(date -u +%Y%m%dT%H%M%SZ)}
[[ "$rollout_id" =~ ^[A-Za-z0-9._-]+$ ]] || die 'ROLLOUT_ID contains unsafe characters'
OPS_TMP_DIR=${OPS_TMP_DIR:-$ROOT_DIR/tmp/rollouts/$rollout_id/$OBSERVATION_CLASS}
mkdir -p "$(dirname "$OPS_TMP_DIR")"
if ! mkdir "$OPS_TMP_DIR"; then
  die "observation directory already exists; use a new ROLLOUT_ID: $OPS_TMP_DIR"
fi
chmod 700 "$OPS_TMP_DIR"

capture_identity() {
  local output=$1
  CONTAINER_NAME="$CONTAINER_NAME" IMAGE_DIGEST="$IMAGE_DIGEST" python3 - "$output" <<'PY'
import json, os, subprocess, sys

name=os.environ["CONTAINER_NAME"]
expected=os.environ["IMAGE_DIGEST"]
raw=json.loads(subprocess.check_output(["docker","inspect",name],text=True))[0]
state=raw.get("State",{})
if state.get("Running") is not True:
    raise SystemExit("observation target is not running")
health=state.get("Health",{}).get("Status")
if health not in (None,"healthy"):
    raise SystemExit(f"observation target is not healthy: {health}")
config_image=raw.get("Config",{}).get("Image","")
if "@" not in config_image or config_image.rsplit("@",1)[1] != expected:
    raise SystemExit("container Config.Image does not match IMAGE_DIGEST")
networks={name:{"network_id":value.get("NetworkID"),"ip_address":value.get("IPAddress")} for name,value in raw.get("NetworkSettings",{}).get("Networks",{}).items()}
mounts=[]
for item in raw.get("Mounts",[]):
    mounts.append({
        "type":item.get("Type"),"name":item.get("Name"),"source":item.get("Source"),
        "destination":item.get("Destination"),"rw":item.get("RW"),
    })
document={
    "name":name,"id":raw.get("Id"),"image_id":raw.get("Image"),
    "config_image":config_image,"started_at":state.get("StartedAt"),
    "restart_count":raw.get("RestartCount"),"health":health,
    "networks":networks,"mounts":sorted(mounts,key=lambda value:(value.get("destination") or "")),
}
with open(sys.argv[1],"w",encoding="utf-8") as handle:
    json.dump(document,handle,indent=2,sort_keys=True); handle.write("\n")
PY
  chmod 600 "$output"
}

identity_before="$OPS_TMP_DIR/container-before.json"
identity_after="$OPS_TMP_DIR/container-after.json"
probe_trace="$OPS_TMP_DIR/readiness-probes.ndjson"
capture_identity "$identity_before"
: >"$probe_trace"
chmod 600 "$probe_trace"

# A healthy arbitrary URL is not evidence about CONTAINER_NAME. Observations
# use the container's exact Docker-network address and record that binding.
TARGET_BASE_URL="$TARGET_BASE_URL" OBSERVATION_CLASS="$OBSERVATION_CLASS" \
python3 - "$identity_before" <<'PY'
import ipaddress, json, os, sys
from urllib.parse import urlparse

identity=json.load(open(sys.argv[1],encoding="utf-8"))
target=urlparse(os.environ["TARGET_BASE_URL"])
if target.scheme != "http" or target.username or target.password or target.query or target.fragment:
    raise SystemExit("TARGET_BASE_URL must be a plain container-network http URL without credentials/query/fragment")
if target.path not in ("", "/"):
    raise SystemExit("TARGET_BASE_URL must not contain a path")
try:
    host=str(ipaddress.ip_address(target.hostname or ""))
except ValueError as exc:
    raise SystemExit("TARGET_BASE_URL host must be a literal container IP") from exc
addresses={item.get("ip_address") for item in identity.get("networks",{}).values() if item.get("ip_address")}
if host not in addresses:
    raise SystemExit("TARGET_BASE_URL is not bound to the observed container identity")
if target.port != 8080:
    raise SystemExit("TARGET_BASE_URL must use the container readiness port 8080")
PY

tooling="$OPS_TMP_DIR/observation-tooling.json"
OBSERVER_PATH="${BASH_SOURCE[0]}" METRICS_PATH="$METRICS_COMMAND" NETWORK_PATH="$NETWORK_PROOF_COMMAND" \
python3 - "$tooling" <<'PY'
import hashlib,json,os,pathlib,sys
items={}
for key,label in (("OBSERVER_PATH","observer"),("METRICS_PATH","metrics_adapter"),("NETWORK_PATH","network_adapter")):
    path=pathlib.Path(os.environ[key]).resolve()
    items[label]={"path":str(path),"sha256":hashlib.sha256(path.read_bytes()).hexdigest()}
with open(sys.argv[1],"w",encoding="utf-8") as handle:
    json.dump(items,handle,indent=2,sort_keys=True); handle.write("\n")
PY
chmod 600 "$tooling"

ready_failures=0
ready_checks=0
start_epoch=$(date +%s)
next_probe_epoch=$start_epoch
if [[ "$OBSERVATION_CLASS" == synthetic-provider ]]; then
  [[ -x "${EXAPI_OBSERVATION_EXERCISE_COMMAND:-}" ]] || \
    die 'EXAPI_OBSERVATION_EXERCISE_COMMAND is required for a synthetic-provider observation'
  EXAPI_CONTAINER_NAME="$CONTAINER_NAME" EXAPI_ROLLOUT_ID="$rollout_id" \
    EXAPI_OBSERVATION_START_EPOCH="$start_epoch" "$EXAPI_OBSERVATION_EXERCISE_COMMAND"
fi
while (( $(date +%s) - start_epoch < duration )); do
  ready_checks=$((ready_checks + 1))
  probe_request_id="rollout-$rollout_id-ready-$ready_checks"
  headers="$OPS_TMP_DIR/ready.headers"
  body="$OPS_TMP_DIR/ready.json"
  rm -f "$headers" "$body"
  set +e
  curl_meta=$(curl --fail --silent --show-error --max-time 10 -H "X-Request-Id: $probe_request_id" -D "$headers" -o "$body" \
    -w '%{http_code}|%{time_total}' "${TARGET_BASE_URL%/}/ready")
  curl_rc=$?
  set -e
  if ! PROBE_EPOCH="$(date +%s)" PROBE_REQUEST_ID="$probe_request_id" CURL_RC="$curl_rc" CURL_META="$curl_meta" \
    python3 - "$headers" "$body" "$probe_trace" <<'PY'
import datetime, hashlib, json, os, pathlib, sys

headers_path, body_path, trace_path = map(pathlib.Path, sys.argv[1:])
curl_rc=int(os.environ["CURL_RC"])
parts=os.environ.get("CURL_META","").rsplit("|",1)
try:
    status_code=int(parts[0]); latency_ms=float(parts[1])*1000
except (ValueError,IndexError):
    status_code=0; latency_ms=0.0
headers=headers_path.read_text(encoding="utf-8",errors="replace") if headers_path.exists() else ""
body=body_path.read_bytes() if body_path.exists() else b""
content_type_json="application/json" in headers.lower()
json_object=False
try:
    json_object=isinstance(json.loads(body),dict)
except (json.JSONDecodeError,UnicodeDecodeError):
    pass
success=curl_rc==0 and status_code==200 and content_type_json and json_object
epoch=int(os.environ["PROBE_EPOCH"])
entry={
    "epoch":epoch,"observed_at":datetime.datetime.fromtimestamp(epoch,datetime.timezone.utc).isoformat().replace("+00:00","Z"),
    "request_id":os.environ["PROBE_REQUEST_ID"],
    "curl_exit_code":curl_rc,"http_status":status_code,"latency_ms":round(latency_ms,3),
    "content_type_json":content_type_json,"json_object":json_object,
    "body_sha256":hashlib.sha256(body).hexdigest(),"success":success,
}
with trace_path.open("a",encoding="utf-8") as handle:
    handle.write(json.dumps(entry,sort_keys=True,separators=(",", ":"))+"\n")
raise SystemExit(0 if success else 1)
PY
  then
    ready_failures=$((ready_failures + 1))
  fi
  next_probe_epoch=$((next_probe_epoch + interval))
  now=$(date +%s)
  if (( next_probe_epoch > now )); then
    sleep "$((next_probe_epoch - now))"
  fi
done
end_epoch=$(date +%s)
capture_identity "$identity_after"
(( ready_failures == 0 )) || die "$ready_failures readiness checks failed"

metrics="$OPS_TMP_DIR/metrics.json"
network="$OPS_TMP_DIR/network-proof.json"
CONTAINER_NAME="$CONTAINER_NAME" EXAPI_RELEASE_ROOT="$ROOT_DIR" \
EXAPI_PROBE_TRACE_FILE="$probe_trace" EXAPI_METRICS_WORK_DIR="$OPS_TMP_DIR" \
EXAPI_OBSERVATION_START_EPOCH="$start_epoch" EXAPI_OBSERVATION_END_EPOCH="$end_epoch" \
  "$METRICS_COMMAND" >"$metrics"
EXAPI_CONTAINER_NAME="$CONTAINER_NAME" EXAPI_OBSERVATION_CLASS="$OBSERVATION_CLASS" \
EXAPI_ROLLOUT_ID="$rollout_id" EXAPI_IMAGE_DIGEST="$IMAGE_DIGEST" \
EXAPI_TARGET_BASE_URL="$TARGET_BASE_URL" EXAPI_CONTAINER_IDENTITY_FILE="$identity_before" \
EXAPI_OBSERVATION_START_EPOCH="$start_epoch" EXAPI_OBSERVATION_END_EPOCH="$end_epoch" \
  "$NETWORK_PROOF_COMMAND" >"$network"
chmod 600 "$metrics" "$network"

evidence="$OPS_TMP_DIR/observation-evidence.json"
evidence_tmp=$(mktemp "$OPS_TMP_DIR/observation-evidence.json.XXXXXX")
OBSERVATION_CLASS="$OBSERVATION_CLASS" IMAGE_DIGEST="$IMAGE_DIGEST" ROLLOUT_ID="$rollout_id" \
START_EPOCH="$start_epoch" END_EPOCH="$end_epoch" INTERVAL="$interval" READY_CHECKS="$ready_checks" \
TARGET_BASE_URL="$TARGET_BASE_URL" python3 - "$metrics" "$network" "$identity_before" "$identity_after" "$probe_trace" "$tooling" "$evidence_tmp" <<'PY'
import hashlib, json, os, pathlib, sys
from datetime import datetime, timezone

metrics_path, network_path, before_path, after_path, trace_path, tooling_path, output_path = map(pathlib.Path,sys.argv[1:])
metrics=json.loads(metrics_path.read_text(encoding="utf-8"))
network=json.loads(network_path.read_text(encoding="utf-8"))
tooling=json.loads(tooling_path.read_text(encoding="utf-8"))
before=json.loads(before_path.read_text(encoding="utf-8"))
after=json.loads(after_path.read_text(encoding="utf-8"))
trace_lines=[line for line in trace_path.read_text(encoding="utf-8").splitlines() if line]
trace=[json.loads(line) for line in trace_lines]
ready_checks=int(os.environ["READY_CHECKS"])
if len(trace)!=ready_checks or any(item.get("success") is not True for item in trace):
    raise SystemExit("readiness trace is incomplete or contains a failure")
stable_fields=("id","image_id","config_image","started_at","networks","mounts")
for field in stable_fields:
    if before.get(field)!=after.get(field):
        raise SystemExit(f"container identity changed during observation: {field}")
if before.get("restart_count")!=after.get("restart_count"):
    raise SystemExit("container restarted during observation")
if network.get("container_id")!=before.get("id"):
    raise SystemExit("network proof is not bound to the observed container")
trace_sha=hashlib.sha256(trace_path.read_bytes()).hexdigest()
if metrics.get("probe_trace_sha256")!=trace_sha or metrics.get("probe_request_count")!=ready_checks:
    raise SystemExit("metrics are not bound to the readiness trace")
required_numbers=("request_count","unexpected_5xx","error_rate","latency_sample_count","p95_ms","baseline_p95_ms","new_p0_p1")
for key in required_numbers:
    if isinstance(metrics.get(key),bool) or not isinstance(metrics.get(key),(int,float)):
        raise SystemExit(f"metrics adapter omitted numeric {key}")
if metrics["request_count"]<ready_checks or metrics["latency_sample_count"]<ready_checks:
    raise SystemExit("metrics do not cover every readiness probe")
if metrics["baseline_p95_ms"]<=0:
    raise SystemExit("metrics baseline is unavailable")
if metrics["new_p0_p1"]!=0:
    raise SystemExit("new P0/P1 alerts were observed")
if metrics["request_count"]>=100:
    if metrics["error_rate"]>=0.01:
        raise SystemExit("error rate is not below 1%")
elif metrics["unexpected_5xx"]!=0:
    raise SystemExit("unexpected 5xx response with fewer than 100 requests")
if metrics["latency_sample_count"]>=100 and metrics["p95_ms"]>metrics["baseline_p95_ms"]*1.2:
    raise SystemExit("p95 regression exceeds 20%")
kind=os.environ["OBSERVATION_CLASS"]
if kind=="restored-data":
    for key in ("egress_denied","integrity_verified","decryption_verified"):
        if network.get(key) is not True: raise SystemExit(f"restored-data proof omitted {key}=true")
    evidence_sha=network.get("restored_counts_evidence_sha256")
    if not isinstance(evidence_sha,str) or len(evidence_sha)!=64 or any(char not in "0123456789abcdef" for char in evidence_sha):
        raise SystemExit("restored-data proof is not bound to protected restore evidence")
    if not isinstance(network.get("restored_counts_evidence_rollout_id"),str) or not network["restored_counts_evidence_rollout_id"]:
        raise SystemExit("restored-data proof omitted restore evidence rollout identity")
elif kind=="synthetic-provider":
    for key in ("egress_allowlist_verified","canary_only_credentials","provider_smoke_verified"):
        if network.get(key) is not True: raise SystemExit(f"synthetic-provider proof omitted {key}=true")
    if not isinstance(network.get("provider_request_count"),int) or network["provider_request_count"]<1:
        raise SystemExit("synthetic provider proof omitted a provider request")
elif network.get("production_topology_verified") is not True:
    raise SystemExit("production topology proof is unavailable")
start=int(os.environ["START_EPOCH"]); end=int(os.environ["END_EPOCH"])
if end-start < (1800 if kind!="production" else 3600):
    raise SystemExit("observation duration is too short")
def digest(path): return hashlib.sha256(path.read_bytes()).hexdigest()
out={
    **metrics,**network,"rollout_id":os.environ["ROLLOUT_ID"],"observation_class":kind,
    "image_digest":os.environ["IMAGE_DIGEST"],"container_identity":before,
    "target_base_url":os.environ["TARGET_BASE_URL"],"tooling":tooling,
    "duration_minutes":(end-start)/60,"readiness_interval_seconds":int(os.environ["INTERVAL"]),
    "readiness_checks":ready_checks,"readiness_failures":0,"restarts":0,
    "readiness_trace_sha256":trace_sha,"metrics_sha256":digest(metrics_path),
    "network_proof_sha256":digest(network_path),
    "observed_from":datetime.fromtimestamp(start,timezone.utc).isoformat().replace("+00:00","Z"),
    "observed_until":datetime.fromtimestamp(end,timezone.utc).isoformat().replace("+00:00","Z"),
    "evidence_finalized_at":datetime.now(timezone.utc).isoformat().replace("+00:00","Z"),
}
if kind=="restored-data": out["production_data"]=True
if kind=="synthetic-provider": out["production_data"]=False
with output_path.open("w",encoding="utf-8") as handle:
    json.dump(out,handle,indent=2,sort_keys=True); handle.write("\n")
PY
chmod 600 "$evidence_tmp"
mv -f "$evidence_tmp" "$evidence"
printf 'Observation evidence: %s\n' "$evidence"
