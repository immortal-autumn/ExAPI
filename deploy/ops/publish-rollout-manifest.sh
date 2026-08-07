#!/usr/bin/env bash
# Verify OCI provenance, validate/sign the manifest, and retain all evidence.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT_DIR"
die() { printf 'manifest publication failed: %s\n' "$*" >&2; exit 1; }
for name in ROLLOUT_MANIFEST ROLLOUT_MANIFEST_S3_URI ROLLOUT_MANIFEST_RETENTION_UNTIL; do
  [[ -n "${!name:-}" ]] || die "$name is required"
done
for command_name in aws cosign python3 sha256sum; do command -v "$command_name" >/dev/null 2>&1 || die "$command_name is required"; done
[[ -r "$ROLLOUT_MANIFEST" ]] || die 'ROLLOUT_MANIFEST is not readable'
[[ "$ROLLOUT_MANIFEST_S3_URI" == s3://* ]] || die 'ROLLOUT_MANIFEST_S3_URI must be off-host s3:// storage'
ROLLOUT_MANIFEST_RETENTION_UNTIL="$ROLLOUT_MANIFEST_RETENTION_UNTIL" python3 - <<'PY'
import os
from datetime import datetime, timezone
value = os.environ["ROLLOUT_MANIFEST_RETENTION_UNTIL"]
if not value.endswith("Z") or datetime.fromisoformat(value[:-1] + "+00:00") <= datetime.now(timezone.utc):
    raise SystemExit("ROLLOUT_MANIFEST_RETENTION_UNTIL must be a future RFC3339 UTC time")
PY

bundle="${ROLLOUT_MANIFEST}.sigstore.json"
checksum="${ROLLOUT_MANIFEST}.sha256"
attestation_evidence="${ROLLOUT_MANIFEST}.oci-provenance-verification.json"
python3 tools/check_rollout_manifest.py "$ROLLOUT_MANIFEST"
sha256sum "$ROLLOUT_MANIFEST" >"$checksum"

oci_image=$(python3 - "$ROLLOUT_MANIFEST" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
print(d["artifact"]["image"])
PY
)
[[ "$oci_image" =~ @sha256:[0-9a-f]{64}$ ]] || die 'manifest artifact image is not an exact OCI digest'
canonical_oidc_issuer=https://token.actions.githubusercontent.com
canonical_workflow_identity='^https://github\.com/immortal-autumn/Sub2API2Personal/\.github/workflows/release\.yml@'
cosign verify-attestation \
  --type slsaprovenance \
  --certificate-identity-regexp "$canonical_workflow_identity" \
  --certificate-oidc-issuer "$canonical_oidc_issuer" \
  "$oci_image" >"$attestation_evidence"
[[ -s "$attestation_evidence" ]] || die 'OCI provenance verification produced no evidence'

sign_args=(sign-blob --yes --bundle "$bundle")
verify_args=(--bundle "$bundle")
if [[ -n "${COSIGN_KEY_REF:-}" ]]; then
  sign_args+=(--key "$COSIGN_KEY_REF")
  [[ -n "${COSIGN_VERIFY_KEY:-}" ]] || die 'COSIGN_VERIFY_KEY is required for keyed signing'
  verify_args+=(--key "$COSIGN_VERIFY_KEY")
else
  [[ -n "${COSIGN_CERTIFICATE_IDENTITY_REGEXP:-}" && -n "${COSIGN_CERTIFICATE_OIDC_ISSUER:-}" ]] || \
    die 'keyless signing requires COSIGN_CERTIFICATE_IDENTITY_REGEXP and COSIGN_CERTIFICATE_OIDC_ISSUER'
  verify_args+=(--certificate-identity-regexp "$COSIGN_CERTIFICATE_IDENTITY_REGEXP")
  verify_args+=(--certificate-oidc-issuer "$COSIGN_CERTIFICATE_OIDC_ISSUER")
fi
cosign "${sign_args[@]}" "$ROLLOUT_MANIFEST" >/dev/null
python3 tools/check_rollout_manifest.py "$ROLLOUT_MANIFEST" "${verify_args[@]}"

AWS_ARGS=(); [[ -z "${S3_ENDPOINT_URL:-}" ]] || AWS_ARGS+=(--endpoint-url "$S3_ENDPOINT_URL")
base=${ROLLOUT_MANIFEST_S3_URI%/}
for pair in \
  "$ROLLOUT_MANIFEST:manifest.json" \
  "$checksum:manifest.json.sha256" \
  "$bundle:manifest.sigstore.json" \
  "$attestation_evidence:oci-provenance-verification.json"; do
  source=${pair%%:*}; name=${pair#*:}
  aws "${AWS_ARGS[@]}" s3 cp "$source" "$base/$name" --only-show-errors
done

python3 - "$base" "$ROLLOUT_MANIFEST_RETENTION_UNTIL" "${AWS_ARGS[@]}" <<'PY'
import json, subprocess, sys
from datetime import datetime, timezone
from urllib.parse import urlparse
base = sys.argv[1]
retention_until = sys.argv[2]
aws_args = sys.argv[3:]
root = urlparse(base)
versioning = subprocess.run(
    ["aws", *aws_args, "s3api", "get-bucket-versioning", "--bucket", root.netloc,
     "--query", "Status", "--output", "text"],
    check=True, capture_output=True, text=True,
)
if versioning.stdout.strip() != "Enabled":
    raise SystemExit("rollout manifest bucket does not have versioning enabled")
requested_retention = datetime.fromisoformat(retention_until[:-1] + "+00:00")
for name in ("manifest.json", "manifest.json.sha256", "manifest.sigstore.json", "oci-provenance-verification.json"):
    uri = urlparse(base + "/" + name)
    result = subprocess.run(
        ["aws", *aws_args, "s3api", "head-object", "--bucket", uri.netloc,
         "--key", uri.path.lstrip("/"), "--output", "json"],
        check=True, capture_output=True, text=True,
    )
    head = json.loads(result.stdout)
    if not head.get("VersionId") or head["VersionId"] == "null":
        raise SystemExit(f"uploaded {name} has no object version ID")
    subprocess.run(
        ["aws", *aws_args, "s3api", "put-object-retention", "--bucket", uri.netloc,
         "--key", uri.path.lstrip("/"), "--version-id", head["VersionId"],
         "--retention", f"Mode=COMPLIANCE,RetainUntilDate={retention_until}"],
        check=True,
    )
    retained = subprocess.run(
        ["aws", *aws_args, "s3api", "head-object", "--bucket", uri.netloc,
         "--key", uri.path.lstrip("/"), "--version-id", head["VersionId"], "--output", "json"],
        check=True, capture_output=True, text=True,
    )
    retained_head = json.loads(retained.stdout)
    if retained_head.get("ObjectLockMode") != "COMPLIANCE" or not retained_head.get("ObjectLockRetainUntilDate"):
        raise SystemExit(f"uploaded {name} does not have compliance retention")
    actual_retention = datetime.fromisoformat(retained_head["ObjectLockRetainUntilDate"].replace("Z", "+00:00"))
    if actual_retention <= datetime.now(timezone.utc):
        raise SystemExit(f"uploaded {name} retention is not in the future")
    if actual_retention < requested_retention:
        raise SystemExit(f"uploaded {name} retention is shorter than requested")
    print(f"{name}: retained version {head['VersionId']}")
PY
printf 'Signed rollout manifest published to %s\n' "$base"
