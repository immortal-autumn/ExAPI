//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpsRepositoryLookupDeletedKeyAudit(t *testing.T) {
	ctx := context.Background()
	_, _ = integrationDB.ExecContext(ctx, "TRUNCATE deleted_api_key_audits RESTART IDENTITY")
	repo := newOpsRepositoryWithDigester(integrationDB, mustGatewayAPIKeyDigesterForTest(t))

	// 同一 key 两条审计,取最近一条(deleted_at DESC, id DESC)
	digest, err := repo.digester.Digest("sk-lookup-1")
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO deleted_api_key_audits (key, key_digest, api_key_id, user_id, key_name, deleted_at)
		VALUES ('__hmac__old', $3, 10, 100, 'old', $1),
		       ('__hmac__new', $3, 11, 200, 'new', $2)`,
		time.Now().Add(-time.Hour), time.Now(), digest)
	require.NoError(t, err)

	res, err := repo.LookupDeletedKeyAudit(ctx, "sk-lookup-1")
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, int64(200), res.UserID)
	require.Equal(t, "new", res.KeyName)

	// 未命中返回 nil
	miss, err := repo.LookupDeletedKeyAudit(ctx, "sk-never-existed")
	require.NoError(t, err)
	require.Nil(t, miss)
}
