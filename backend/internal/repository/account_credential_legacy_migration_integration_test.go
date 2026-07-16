//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrateLegacyAccountCredentials_ProtectsMixedRowsAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	protector := mustAccountCredentialProtectorForTest(t)

	legacyJSON, err := json.Marshal(map[string]any{"access_token": "test-legacy-access", "refresh_token": "test-legacy-refresh"})
	require.NoError(t, err)
	var legacyID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO accounts (name, platform, type, credentials, extra, concurrency, priority, status, schedulable, created_at, updated_at)
		VALUES ('legacy credentials', 'openai', 'oauth', $1::jsonb, '{}'::jsonb, 1, 1, 'active', true, NOW(), NOW())
		RETURNING id`, legacyJSON).Scan(&legacyID))

	protectedCredentials, err := protector.seal(legacyID+1, map[string]any{"api_key": "test-already-protected"})
	require.NoError(t, err)
	protectedJSON, err := json.Marshal(protectedCredentials)
	require.NoError(t, err)
	var protectedID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO accounts (id, name, platform, type, credentials, extra, concurrency, priority, status, schedulable, created_at, updated_at)
		VALUES ($1, 'protected credentials', 'openai', 'api_key', $2::jsonb, '{}'::jsonb, 1, 1, 'active', true, NOW(), NOW())
		RETURNING id`, legacyID+1, protectedJSON).Scan(&protectedID))

	stats, err := inspectLegacyAccountCredentialsInTx(ctx, tx, protector)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.AccountsMigrated)
	var dryRunRaw []byte
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT credentials::text FROM accounts WHERE id=$1`, legacyID).Scan(&dryRunRaw))
	require.Contains(t, string(dryRunRaw), "test-legacy-access", "dry-run must not mutate credentials")

	stats, err = migrateLegacyAccountCredentialsInTx(ctx, tx, protector)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.AccountsMigrated)

	var stored []byte
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT credentials::text FROM accounts WHERE id=$1`, legacyID).Scan(&stored))
	require.NotContains(t, string(stored), "test-legacy-access")
	require.NotContains(t, string(stored), "test-legacy-refresh")
	var storedMap map[string]any
	require.NoError(t, json.Unmarshal(stored, &storedMap))
	opened, rewrite, err := protector.openLegacy(legacyID, storedMap)
	require.NoError(t, err)
	require.False(t, rewrite)
	require.Equal(t, "test-legacy-access", opened["access_token"])

	stats, err = migrateLegacyAccountCredentialsInTx(ctx, tx, protector)
	require.NoError(t, err)
	require.Zero(t, stats.AccountsMigrated)
}

func TestMigrateLegacyAccountCredentials_RollsBackOnMalformedEnvelope(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	protector := mustAccountCredentialProtectorForTest(t)

	var firstID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO accounts (name, platform, type, credentials, extra, concurrency, priority, status, schedulable, created_at, updated_at)
		VALUES ('first legacy', 'openai', 'oauth', '{"access_token":"test-first"}'::jsonb, '{}'::jsonb, 1, 1, 'active', true, NOW(), NOW())
		RETURNING id`).Scan(&firstID))
	_, err := tx.ExecContext(ctx, `
		INSERT INTO accounts (name, platform, type, credentials, extra, concurrency, priority, status, schedulable, created_at, updated_at)
		VALUES ('malformed envelope', 'openai', 'oauth', '{"__sub2api_encrypted_credentials":"enc:not-valid"}'::jsonb, '{}'::jsonb, 1, 1, 'active', true, NOW(), NOW())`)
	require.NoError(t, err)

	_, err = migrateLegacyAccountCredentialsInTx(ctx, tx, protector)
	require.Error(t, err)

	var credentials map[string]any
	var raw []byte
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT credentials::text FROM accounts WHERE id=$1`, firstID).Scan(&raw))
	require.NoError(t, json.Unmarshal(raw, &credentials))
	require.Equal(t, "test-first", credentials["access_token"], "earlier rewrite must roll back")

	var one int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT 1`).Scan(&one))
	require.Equal(t, 1, one, "caller transaction must remain usable")
}
