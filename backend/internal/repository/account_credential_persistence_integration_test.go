//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountRepositoryCreateStoresEncryptedCredentialsAndReadsPlaintext(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	protector := mustAccountCredentialProtectorForTest(t)
	repo := newAccountRepositoryWithSQLAndProtector(client, integrationDB, nil, protector)
	credentials := map[string]any{
		"access_token":  "test-account-access-token",
		"refresh_token": "test-account-refresh-token",
	}
	account := &service.Account{
		Name: "protected account", Platform: service.PlatformOpenAI,
		Type: service.AccountTypeOAuth, Credentials: credentials,
		Concurrency: 1, Priority: 1, Status: service.StatusActive, Schedulable: true,
	}

	require.NoError(t, repo.Create(ctx, account))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", account.ID)
	})

	var encoded []byte
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT credentials::text FROM accounts WHERE id = $1`, account.ID,
	).Scan(&encoded))
	require.NotContains(t, string(encoded), "test-account-access-token")
	require.NotContains(t, string(encoded), "test-account-refresh-token")
	var stored map[string]any
	require.NoError(t, json.Unmarshal(encoded, &stored))
	require.Contains(t, stored, accountCredentialEnvelopeField)

	got, err := repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, credentials, got.Credentials)

	updatedCredentials := map[string]any{
		"access_token":  "test-updated-access-token",
		"refresh_token": "test-updated-refresh-token",
	}
	require.NoError(t, repo.UpdateCredentials(ctx, account.ID, updatedCredentials))
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT credentials::text FROM accounts WHERE id = $1`, account.ID,
	).Scan(&encoded))
	require.NotContains(t, string(encoded), "test-updated-access-token")
	require.NotContains(t, string(encoded), "test-updated-refresh-token")
	got, err = repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, updatedCredentials, got.Credentials)

	got.Credentials["access_token"] = "test-full-update-token"
	require.NoError(t, repo.Update(ctx, got))
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT credentials::text FROM accounts WHERE id = $1`, account.ID,
	).Scan(&encoded))
	require.NotContains(t, string(encoded), "test-full-update-token")
	got, err = repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, "test-full-update-token", got.Credentials["access_token"])

	_, err = repo.BulkUpdate(ctx, []int64{account.ID}, service.AccountBulkUpdate{
		Credentials: map[string]any{"plan_type": "test-plan"},
	})
	require.NoError(t, err)
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT credentials::text FROM accounts WHERE id = $1`, account.ID,
	).Scan(&encoded))
	require.NotContains(t, string(encoded), "test-plan")
	got, err = repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, "test-full-update-token", got.Credentials["access_token"])
	require.Equal(t, "test-plan", got.Credentials["plan_type"])
}

func TestAccountRepositoryCreateRollsBackWhenCredentialProtectionFails(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := newAccountRepositoryWithSQLAndProtector(client, integrationDB, nil, nil)
	account := &service.Account{
		Name: "must not persist", Platform: service.PlatformOpenAI,
		Type: service.AccountTypeOAuth, Credentials: map[string]any{"access_token": "test-token"},
		Concurrency: 1, Priority: 1, Status: service.StatusActive,
	}

	err := repo.Create(ctx, account)
	require.Error(t, err)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM accounts WHERE name = 'must not persist'`,
	).Scan(&count))
	require.Zero(t, count)
}
