#!/usr/bin/env bash
# Observe a digest-pinned canary/production target and fail the documented SLOs.
set -euo pipefail

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
OPS_TMP_DIR=${OPS_TMP_DIR:-$ROOT_DIR/tmp/rollouts/$rollout_id/$OBSERVATION_CLASS}
mkdir -p "$OPS_TMP_DIR"; chmod 700 "$OPS_TMP_DIR"

ready_failures=0
ready_checks=0
start_epoch=$(date +%s)
next_probe_epoch=$start_epoch
initial_restarts=$(docker inspect -f '{{.RestartCount}}' "$CONTAINER_NAME")
while (( $(date +%s) - start_epoch < duration )); do
  ready_checks=$((ready_checks + 1))
  headers="$OPS_TMP_DIR/ready.headers"
  body="$OPS_TMP_DIR/ready.json"
  if ! curl --fail --silent --show-error --max-time 10 -D "$headers" -o "$body" "${TARGET_BASE_URL%/}/ready"; then
    ready_failures=$((ready_failures + 1))
  elif ! python3 - "$headers" "$body" <<'PY'
import json, sys
headers = open(sys.argv[1], encoding="utf-8", errors="replace").read().lower()
if "content-type:" not in headers or "application/json" not in headers:
    raise SystemExit("readiness response is not JSON")
data = json.load(open(sys.argv[2], encoding="utf-8"))
if not isinstance(data, dict):
    raise SystemExit("readiness response is not an object")
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
final_restarts=$(docker inspect -f '{{.RestartCount}}' "$CONTAINER_NAME")
(( ready_failures == 0 )) || die "$ready_failures readiness checks failed"
(( final_restarts == initial_restarts )) || die 'container restarted during observation'

metrics="$OPS_TMP_DIR/metrics.json"
network="$OPS_TMP_DIR/network-proof.json"
EXAPI_OBSERVATION_START_EPOCH="$start_epoch" EXAPI_OBSERVATION_END_EPOCH="$end_epoch" "$METRICS_COMMAND" >"$metrics"
EXAPI_CONTAINER_NAME="$CONTAINER_NAME" EXAPI_OBSERVATION_CLASS="$OBSERVATION_CLASS" "$NETWORK_PROOF_COMMAND" >"$network"
evidence="$OPS_TMP_DIR/observation-evidence.json"
OBSERVATION_CLASS="$OBSERVATION_CLASS" IMAGE_DIGEST="$IMAGE_DIGEST" START_EPOCH="$start_epoch" END_EPOCH="$end_epoch" \
INTERVAL="$interval" READY_CHECKS="$ready_checks" RESTARTS="$((final_restarts-initial_restarts))" python3 - "$metrics" "$network" "$evidence" <<'PY'
import json, os, sys
from datetime import datetime, timezone
metrics = json.load(open(sys.argv[1], encoding="utf-8"))
network = json.load(open(sys.argv[2], encoding="utf-8"))
required_numbers = ("request_count", "unexpected_5xx", "error_rate", "latency_sample_count", "p95_ms", "baseline_p95_ms", "new_p0_p1")
for key in required_numbers:
    if isinstance(metrics.get(key), bool) or not isinstance(metrics.get(key), (int, float)):
        raise SystemExit(f"metrics adapter omitted numeric {key}")
if metrics["new_p0_p1"] != 0:
    raise SystemExit("new P0/P1 alerts were observed")
if metrics["request_count"] >= 100:
    if metrics["error_rate"] >= 0.01:
        raise SystemExit("error rate is not below 1%")
elif metrics["unexpected_5xx"] != 0:
    raise SystemExit("unexpected 5xx response with fewer than 100 requests")
if metrics["latency_sample_count"] >= 100:
    if metrics["baseline_p95_ms"] <= 0 or metrics["p95_ms"] > metrics["baseline_p95_ms"] * 1.2:
        raise SystemExit("p95 regression exceeds 20%")
kind = os.environ["OBSERVATION_CLASS"]
if kind == "restored-data":
    for key in ("egress_denied", "integrity_verified", "decryption_verified"):
        if network.get(key) is not True:
            raise SystemExit(f"restored-data proof omitted {key}=true")
elif kind == "synthetic-provider":
    for key in ("egress_allowlist_verified", "canary_only_credentials", "provider_smoke_verified"):
        if network.get(key) is not True:
            raise SystemExit(f"synthetic-provider proof omitted {key}=true")
start = int(os.environ["START_EPOCH"]); end = int(os.environ["END_EPOCH"])
out = {
    **metrics,
    **network,
    "image_digest": os.environ["IMAGE_DIGEST"],
    "duration_minutes": (end - start) / 60,
    "readiness_interval_seconds": int(os.environ["INTERVAL"]),
    "readiness_checks": int(os.environ["READY_CHECKS"]),
    "readiness_failures": 0,
    "restarts": int(os.environ["RESTARTS"]),
    "observed_from": datetime.fromtimestamp(start, timezone.utc).isoformat().replace("+00:00", "Z"),
    "observed_until": datetime.fromtimestamp(end, timezone.utc).isoformat().replace("+00:00", "Z"),
}
if kind == "restored-data": out["production_data"] = True
if kind == "synthetic-provider": out["production_data"] = False
with open(sys.argv[3], "w", encoding="utf-8") as handle:
    json.dump(out, handle, indent=2, sort_keys=True); handle.write("\n")
PY
printf 'Observation evidence: %s\n' "$evidence"
