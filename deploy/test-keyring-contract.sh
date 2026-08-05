#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

fail() {
    printf 'keyring deployment contract failed: %s\n' "$1" >&2
    exit 1
}

for compose in deploy/docker-compose.yml deploy/docker-compose.local.yml deploy/docker-compose.standalone.yml deploy/docker-compose.dev.yml; do
    grep -Fq 'SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=${SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID:?SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID is required}' "$compose" || fail "$compose permits an empty gateway-digest active key"
    grep -Fq 'SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON=${SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON:?SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON is required}' "$compose" || fail "$compose permits an empty gateway-digest keyring"
done

# shellcheck source=deploy/docker-deploy.sh
source deploy/docker-deploy.sh

data_key='QUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUE='
digest_key='QkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkI='
backup_key='Q0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0M='
data_json="{\"data-v1\":\"${data_key}\"}"
digest_json="{\"digest-v1\":\"${digest_key}\"}"
backup_json="{\"backup-v1\":\"${backup_key}\"}"
reused_digest_json="{\"digest-v1\":\"${data_key}\"}"

validate_keyring_pair data-v1 "$data_json" data-encryption
validate_keyring_pair digest-v1 "$digest_json" gateway-digest
validate_keyring_pair backup-v1 "$backup_json" backup-encryption
validate_independent_keyrings \
    data-v1 "$data_json" data-encryption \
    digest-v1 "$digest_json" gateway-digest \
    backup-v1 "$backup_json" backup-encryption

if validate_independent_keyrings \
    data-v1 "$data_json" data-encryption \
    digest-v1 "$reused_digest_json" gateway-digest \
    backup-v1 "$backup_json" backup-encryption >/dev/null 2>&1; then
    fail 'cross-domain key reuse was accepted'
fi

printf 'keyring deployment contract: pass\n'
