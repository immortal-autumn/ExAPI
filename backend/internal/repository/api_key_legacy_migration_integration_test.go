//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrateLegacyGatewayKeyMaterial_ProtectsRowsTransactionally(t *testing.T) {
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	var userID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO users (email, username, password_hash, role, status, concurrency, balance, created_at, updated_at)
		VALUES ('legacy-key-migration@example.com', 'legacy-key-migration', 'not-used', 'user', 'active', 1, 0, NOW(), NOW())
		RETURNING id`).Scan(&userID))

	const rawAPIKey = "test-legacy-api-key-material"
	var apiKeyID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO api_keys (user_id, key, name, status, created_at, updated_at)
		VALUES ($1, $2, 'legacy', 'active', NOW(), NOW())
		RETURNING id`, userID, rawAPIKey).Scan(&apiKeyID))

	const rawDeletedKey = "test-legacy-deleted-key-material"
	var auditID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO deleted_api_key_audits (key, api_key_id, user_id, key_name, deleted_at)
		VALUES ($1, 900001, $2, 'legacy deleted', NOW())
		RETURNING id`, rawDeletedKey, userID).Scan(&auditID))

	digester := mustGatewayAPIKeyDigesterForTest(t)
	stats, err := inspectLegacyGatewayKeyMaterialInTx(ctx, tx, digester)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.APIKeysMigrated)
	require.Equal(t, int64(1), stats.DeletedAuditsMigrated)
	var dryRunKey string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT key FROM api_keys WHERE id = $1`, apiKeyID).Scan(&dryRunKey))
	require.Equal(t, rawAPIKey, dryRunKey, "dry-run must not rewrite API keys")

	stats, err = migrateLegacyGatewayKeyMaterialInTx(ctx, tx, digester)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.APIKeysMigrated)
	require.Equal(t, int64(1), stats.DeletedAuditsMigrated)
	stats, err = migrateLegacyGatewayKeyMaterialInTx(ctx, tx, digester)
	require.NoError(t, err)
	require.Zero(t, stats.APIKeysMigrated)
	require.Zero(t, stats.DeletedAuditsMigrated)

	var storedKey, storedDigest, storedPrefix string
	require.NoError(t, tx.QueryRowContext(ctx,
		`SELECT key, key_digest, key_prefix FROM api_keys WHERE id = $1`, apiKeyID,
	).Scan(&storedKey, &storedDigest, &storedPrefix))
	require.NotEqual(t, rawAPIKey, storedKey)
	require.NotContains(t, storedKey, rawAPIKey)
	require.True(t, digester.Verify(rawAPIKey, storedDigest))
	require.Equal(t, gatewayAPIKeyDisplayPrefix(rawAPIKey), storedPrefix)

	var auditKey, auditDigest string
	require.NoError(t, tx.QueryRowContext(ctx,
		`SELECT key, key_digest FROM deleted_api_key_audits WHERE id = $1`, auditID,
	).Scan(&auditKey, &auditDigest))
	require.NotEqual(t, rawDeletedKey, auditKey)
	require.NotContains(t, auditKey, rawDeletedKey)
	require.True(t, digester.Verify(rawDeletedKey, auditDigest))
}

func TestMigrateLegacyGatewayKeyMaterial_RollsBackWholeTransactionOnInvalidRow(t *testing.T) {
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	var userID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO users (email, username, password_hash, role, status, concurrency, balance, created_at, updated_at)
		VALUES ('legacy-key-rollback@example.com', 'legacy-key-rollback', 'not-used', 'user', 'active', 1, 0, NOW(), NOW())
		RETURNING id`).Scan(&userID))

	const raw = "test-rollback-api-key"
	var apiKeyID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO api_keys (user_id, key, name, status, created_at, updated_at)
		VALUES ($1, $2, 'rollback', 'active', NOW(), NOW()) RETURNING id`, userID, raw).Scan(&apiKeyID))
	_, err = tx.ExecContext(ctx, `
		INSERT INTO deleted_api_key_audits (key, api_key_id, user_id, key_name, deleted_at)
		VALUES ('', 900002, $1, 'invalid empty legacy key', NOW())`, userID)
	require.NoError(t, err)

	digester := mustGatewayAPIKeyDigesterForTest(t)
	err = migrateLegacyGatewayKeyMaterial(ctx, tx, digester)
	require.Error(t, err)

	var storedKey string
	var digest *string
	require.NoError(t, tx.QueryRowContext(ctx,
		`SELECT key, key_digest FROM api_keys WHERE id = $1`, apiKeyID,
	).Scan(&storedKey, &digest))
	require.Equal(t, raw, storedKey)
	require.Nil(t, digest, "failed migration must roll back earlier row rewrites")
}
