package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration217PersistsOperationalBillingPolicyForDeletedResources(t *testing.T) {
	content, err := FS.ReadFile("217_batch_image_operational_quota_snapshot.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	for _, field := range []string{
		"api_key_quota_tracked BOOLEAN NOT NULL DEFAULT FALSE",
		"api_key_rate_limit_tracked BOOLEAN NOT NULL DEFAULT FALSE",
		"account_type_snapshot VARCHAR(32) NOT NULL DEFAULT ''",
		"account_quota_tracked BOOLEAN NOT NULL DEFAULT FALSE",
	} {
		require.Contains(t, sql, field)
	}
	require.Contains(t, sql, "FROM api_keys AS key WHERE job.api_key_id = key.id")
	require.Contains(t, sql, "FROM accounts AS account WHERE job.account_id = account.id")
	require.NotContains(t, sql, "key.deleted_at IS NULL")
	require.NotContains(t, sql, "account.deleted_at IS NULL")
	require.Contains(t, sql, "job.pricing_snapshot_version >= 1")
	require.Contains(t, sql, "jsonb_typeof(account.extra->'quota_limit')")
	require.Contains(t, sql, "btrim(account.extra->>'quota_limit')")
	require.Contains(t, sql, "[eE][+-]?[0-9]+")
}
