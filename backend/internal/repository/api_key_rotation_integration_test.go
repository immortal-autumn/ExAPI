//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGatewayAPIKeyLookupSurvivesDigestKeyRotation(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	old := testGatewayDigestKeyring(t, "old", map[string]byte{"old": 2})
	oldDigester, err := newGatewayAPIKeyDigester(old)
	require.NoError(t, err)
	oldRepo := newAPIKeyRepositoryWithSQLAndDigester(client, integrationDB, oldDigester)

	user := mustCreateUser(t, client, &service.User{
		Email:        "digest-rotation@example.com",
		Username:     "digest-rotation",
		PasswordHash: "not-used",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})
	const raw = "test-key-created-before-rotation"
	key := &service.APIKey{UserID: user.ID, Key: raw, Name: "old digest", Status: service.StatusActive}
	require.NoError(t, oldRepo.Create(ctx, key))

	rotatedRing := testGatewayDigestKeyring(t, "new", map[string]byte{"old": 2, "new": 3})
	rotatedDigester, err := newGatewayAPIKeyDigester(rotatedRing)
	require.NoError(t, err)
	rotatedRepo := newAPIKeyRepositoryWithSQLAndDigester(client, integrationDB, rotatedDigester)

	got, err := rotatedRepo.GetByKeyForAuth(ctx, raw)
	require.NoError(t, err)
	require.Equal(t, key.ID, got.ID)
	exists, err := rotatedRepo.ExistsByKey(ctx, raw)
	require.NoError(t, err)
	require.True(t, exists)
}
