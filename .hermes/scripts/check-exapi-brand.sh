#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../.."

# This gate checks for visible/product-level Sub2API branding while allowing
# compatibility identifiers that are intentionally deferred in the ExAPI fork.
# Keep this allowlist narrow: every remaining occurrence should be either
# upstream attribution, runtime compatibility, or a migration-deferred artifact.

matches=$(rg -n --hidden \
  --glob '!frontend/node_modules/**' \
  --glob '!backend/internal/web/dist/**' \
  --glob '!.git/**' \
  --glob '!frontend/tsconfig.tsbuildinfo' \
  --glob '!scripts/check-exapi-brand.sh' \
  'Sub2API|Sub2Api|SUB2API' \
  frontend/src backend/cmd backend/internal deploy Dockerfile Dockerfile.goreleaser .goreleaser.yaml .goreleaser.simple.yaml 2>/dev/null \
  | rg -v 'github.com/Wei-Shaw/sub2api|otpauth://totp/Sub2API|SUB2API_|X-Sub2API-Grok-Client-Tool-Cache|sub2apipay|Sub2ApiPay|Sub2API-compatible|upstream Sub2API|derived from|compatibility|Historical|original upstream|NestedSub2API|frontend/src/i18n/__tests__/brand-copy.spec.ts' || true)

if [[ -n "$matches" ]]; then
  printf '%s\n' "$matches" >&2
  echo "Found unapproved visible Sub2API branding. Replace with ExAPI or justify in the allowlist." >&2
  exit 1
fi
