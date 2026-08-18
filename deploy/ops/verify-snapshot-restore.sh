#!/usr/bin/env bash
# Require the provider adapter to restore the captured snapshot independently
# into a disposable, egress-denied target and report its verification result.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT_DIR"
die() { printf 'snapshot restore drill failed: %s\n' "$*" >&2; exit 1; }
[[ -n "${RECOVERY_EVIDENCE:-}" && -r "$RECOVERY_EVIDENCE" ]] || die 'RECOVERY_EVIDENCE is required and must be readable'
[[ -n "${SNAPSHOT_RESTORE_COMMAND:-}" && -x "$SNAPSHOT_RESTORE_COMMAND" ]] || die 'SNAPSHOT_RESTORE_COMMAND must be an executable adapter'

readarray -t values < <(python3 - "$RECOVERY_EVIDENCE" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
for value in (d["rollout_id"], d["snapshot"]["provider"], d["snapshot"]["snapshot_id"], d["snapshot"]["target"]):
    if not isinstance(value, str) or not value:
        raise SystemExit("invalid snapshot recovery evidence")
    print(value)
PY
)
(( ${#values[@]} == 4 )) || die 'invalid recovery evidence'
rollout_id=${values[0]}; provider=${values[1]}; snapshot_id=${values[2]}; snapshot_target=${values[3]}
OPS_TMP_DIR=${OPS_TMP_DIR:-$ROOT_DIR/tmp/rollouts/$rollout_id/snapshot-restore}
mkdir -p "$OPS_TMP_DIR"; chmod 700 "$OPS_TMP_DIR"
evidence="$OPS_TMP_DIR/snapshot-restore-evidence.json"

EXAPI_ROLLOUT_ID="$rollout_id" EXAPI_SNAPSHOT_PROVIDER="$provider" \
EXAPI_SNAPSHOT_ID="$snapshot_id" EXAPI_SNAPSHOT_TARGET="$snapshot_target" \
  "$SNAPSHOT_RESTORE_COMMAND" >"$evidence"
python3 - "$evidence" "$rollout_id" "$provider" "$snapshot_id" <<'PY'
import json, sys
from datetime import datetime
d = json.load(open(sys.argv[1], encoding="utf-8"))
expected = {"rollout_id": sys.argv[2], "provider": sys.argv[3], "snapshot_id": sys.argv[4]}
for key, value in expected.items():
    if d.get(key) != value:
        raise SystemExit(f"snapshot adapter returned incorrect {key}")
for key in ("disposable_target", "restored_at"):
    if not isinstance(d.get(key), str) or not d[key]:
        raise SystemExit(f"snapshot adapter omitted {key}")
if not d["restored_at"].endswith("Z"):
    raise SystemExit("restored_at is not RFC3339 UTC")
datetime.fromisoformat(d["restored_at"][:-1] + "+00:00")
for key in ("verified", "egress_denied", "integrity_verified", "decryption_verified"):
    if d.get(key) is not True:
        raise SystemExit(f"snapshot adapter must prove {key}=true")
PY
printf 'Snapshot restore evidence: %s\n' "$evidence"
