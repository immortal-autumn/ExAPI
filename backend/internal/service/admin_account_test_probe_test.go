package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateAccountManagesAccountTestProbeLifecycle(t *testing.T) {
	t.Parallel()

	const accountID int64 = 221
	probe := map[string]any{"status": AccountTestProbeStatusFailed, "reason": AccountTestProbeReasonQuotaExhausted}
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{
		accountID: {
			ID:          accountID,
			Platform:    PlatformAntigravity,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Credentials: map[string]any{"access_token": "old-token", "project_id": "old-project"},
			Extra:       map[string]any{AccountTestProbeExtraKey: probe},
		},
	}}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{
		Extra: map[string]any{"custom": "value", AccountTestProbeExtraKey: map[string]any{"status": "success"}},
	})
	require.NoError(t, err)
	require.Equal(t, probe, updated.Extra[AccountTestProbeExtraKey], "ordinary edits must preserve managed probe state and reject injected state")

	updated, err = svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{
		Credentials: map[string]any{"access_token": "new-token", "project_id": "old-project"},
	})
	require.NoError(t, err)
	require.NotContains(t, updated.Extra, AccountTestProbeExtraKey, "credential changes must invalidate the stale probe result")
}

func TestBulkUpdateAccountsInvalidatesAccountTestProbeForRoutingChanges(t *testing.T) {
	t.Parallel()

	repo := &upstreamBillingProbeAccountRepo{}
	result, err := (&adminServiceImpl{accountRepo: repo}).BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs:  []int64{1},
		Credentials: map[string]any{"model_mapping": map[string]any{"old": "new"}},
		Extra:       map[string]any{AccountTestProbeExtraKey: map[string]any{"status": "success"}},
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Success)
	require.Len(t, repo.bulkUpdates, 1)
	require.Contains(t, repo.bulkUpdates[0].Extra, AccountTestProbeExtraKey)
	require.Nil(t, repo.bulkUpdates[0].Extra[AccountTestProbeExtraKey])
}
