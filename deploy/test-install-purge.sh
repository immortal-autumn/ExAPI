#!/usr/bin/env bash
set -euo pipefail

script="${1:-deploy/install.sh}"
body=$(awk '
    /^uninstall\(\)[[:space:]]*\{/ { in_uninstall=1 }
    in_uninstall { print }
    in_uninstall && /^}/ { exit }
' "$script")

fail() {
    printf 'installer purge contract failed: %s\n' "$1" >&2
    exit 1
}

# Structural assertions are deliberately exact: comments or generic mentions do
# not satisfy the contract.
grep -Fq 'rm -f /etc/systemd/system/sub2api.service.d/10-runtime-secrets.conf' <<<"$body" || fail 'managed drop-in is not removed'
grep -Fq 'rm -f "$RUNTIME_ENV_FILE"' <<<"$body" || fail 'external keyring is not removed by purge'
grep -Fq 'if [ "$remove_config" = true ]; then' <<<"$body" || fail 'purge branch is missing'
grep -Fq 'Preserving external cryptographic keyring: $RUNTIME_ENV_FILE' <<<"$body" || fail 'normal-uninstall preservation notice is missing'
grep -Fq 'PURGE=true 将删除外部加密密钥环' <<<"$body" || fail 'non-interactive purge warning is missing'

grep -Fq 'SUB2API_BACKUP_ENCRYPTION_ACTIVE_KEY_ID=backup-v1' "$script" || fail 'systemd installer does not provision backup active key'
grep -Fq 'SUB2API_BACKUP_ENCRYPTION_KEYS_JSON=' "$script" || fail 'systemd installer does not provision backup keyring'
grep -Fq 'SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=digest-v1' "$script" || fail 'systemd installer does not provision gateway-digest active key'
grep -Fq 'SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON=' "$script" || fail 'systemd installer does not provision gateway-digest keyring'
grep -Fq 'BACKUP_ENCRYPTION_KEY=$(generate_base64_key)' deploy/docker-deploy.sh || fail 'Docker installer does not generate an independent backup key'
grep -Fq 'GATEWAY_KEY_DIGEST_KEY=$(generate_base64_key)' deploy/docker-deploy.sh || fail 'Docker installer does not generate an independent gateway-digest key'

grep -Fq 'validate_runtime_keyring SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID' "$script" || fail 'existing data keyring is not validated'
grep -Fq 'validate_runtime_keyring SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID' "$script" || fail 'existing gateway-digest keyring is not validated'
grep -Fq 'validate_runtime_keyring SUB2API_BACKUP_ENCRYPTION_ACTIVE_KEY_ID' "$script" || fail 'existing backup keyring is not validated'
grep -Fq 'validate_runtime_keyring_independence' "$script" || fail 'systemd keyrings are not checked for cross-domain reuse'

grep -Fq 'JWT_SECRET=$(read_env_value JWT_SECRET)' deploy/docker-deploy.sh || fail 'Docker redeploy does not preserve JWT secret'
grep -Fq 'TOTP_ENCRYPTION_KEY=$(read_env_value TOTP_ENCRYPTION_KEY)' deploy/docker-deploy.sh || fail 'Docker redeploy does not preserve TOTP key'
grep -Fq 'POSTGRES_PASSWORD=$(read_env_value POSTGRES_PASSWORD)' deploy/docker-deploy.sh || fail 'Docker redeploy does not preserve PostgreSQL password'
grep -Fq 'validate_keyring_pair "$DATA_ENCRYPTION_ACTIVE_KEY_ID"' deploy/docker-deploy.sh || fail 'Docker redeploy does not validate existing data keyring'
grep -Fq 'validate_keyring_pair "$GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID"' deploy/docker-deploy.sh || fail 'Docker redeploy does not validate existing gateway-digest keyring'
grep -Fq 'Generated missing gateway-digest keyring for existing deployment' deploy/docker-deploy.sh || fail 'Docker upgrade does not provision a missing gateway-digest keyring'
grep -Fq 'validate_independent_keyrings' deploy/docker-deploy.sh || fail 'Docker keyrings are not checked for cross-domain reuse'
grep -Fq 'Preserving existing validated encryption keyrings' deploy/docker-deploy.sh || fail 'Docker redeploy does not preserve encryption roots'
for compose in deploy/docker-compose.yml deploy/docker-compose.local.yml deploy/docker-compose.standalone.yml deploy/docker-compose.dev.yml; do
    grep -Fq 'SUB2API_MIGRATE_LEGACY_SECURITY_SECRETS=${SUB2API_MIGRATE_LEGACY_SECURITY_SECRETS:-false}' "$compose" || fail "migration switch missing from $compose"
done

grep -Fq 'PURGE=true 将删除外部加密密钥环' "$script" || fail 'purge warning is not Chinese'
help_output=$(bash "$script" --help </dev/null 2>&1) || fail '--help fails without a controlling terminal'
grep -Fq -- '--purge               同时删除配置和外部加密密钥环（不可逆）' <<<"$help_output" || fail '--purge is not documented in Chinese help output'

# Verify deletion occurs after the purge branch begins, while the preservation
# notice occurs in the else branch. The irreversible warning must precede the
# deletion.
purge_line=$(grep -nF 'if [ "$remove_config" = true ]; then' <<<"$body" | cut -d: -f1 | head -1)
warning_line=$(grep -nF 'PURGE=true 将删除外部加密密钥环' <<<"$body" | cut -d: -f1 | head -1)
delete_line=$(grep -nF 'rm -f "$RUNTIME_ENV_FILE"' <<<"$body" | cut -d: -f1 | head -1)
else_line=$(grep -nF '    else' <<<"$body" | awk -F: -v p="$purge_line" '$1 > p {print $1; exit}')
preserve_line=$(grep -nF 'Preserving external cryptographic keyring: $RUNTIME_ENV_FILE' <<<"$body" | cut -d: -f1 | head -1)

[ "$warning_line" -lt "$delete_line" ] || fail 'irreversible warning does not precede keyring deletion'
[ "$delete_line" -gt "$purge_line" ] && [ "$delete_line" -lt "$else_line" ] || fail 'keyring deletion is not confined to purge branch'
[ "$preserve_line" -gt "$else_line" ] || fail 'preservation notice is not in normal-uninstall branch'

printf 'installer purge cleanup contract: pass\n'
