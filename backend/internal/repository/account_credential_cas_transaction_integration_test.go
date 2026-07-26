//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type credentialCASMutation func(context.Context, *accountRepository, *service.Account, service.GrokCredentialMutationSnapshot) (bool, error)

func TestGrokCredentialConditionalMutations_HonorCallerTransactionRollback(t *testing.T) {
	cases := []struct {
		name   string
		mutate credentialCASMutation
	}{
		{
			name: "credential permanent error",
			mutate: func(ctx context.Context, repo *accountRepository, account *service.Account, snapshot service.GrokCredentialMutationSnapshot) (bool, error) {
				return repo.SetGrokCredentialErrorIfMatch(ctx, account.ID, snapshot, "caller transaction rollback")
			},
		},
		{
			name: "oauth reconciliation error",
			mutate: func(ctx context.Context, repo *accountRepository, account *service.Account, _ service.GrokCredentialMutationSnapshot) (bool, error) {
				return repo.SetGrokOAuthErrorIfCredentialsUnchanged(ctx, account.ID, account.Credentials, "caller transaction rollback")
			},
		},
		{
			name: "oauth credential replacement",
			mutate: func(ctx context.Context, repo *accountRepository, account *service.Account, _ service.GrokCredentialMutationSnapshot) (bool, error) {
				return repo.UpdateGrokOAuthCredentialsIfUnchanged(ctx, account.ID, account.Credentials, account.ProxyID, map[string]any{
					"access_token": "replacement-access",
				})
			},
		},
		{
			name: "oauth refresh permanent error",
			mutate: func(ctx context.Context, repo *accountRepository, account *service.Account, _ service.GrokCredentialMutationSnapshot) (bool, error) {
				return repo.SetGrokOAuthRefreshErrorIfCredentialsUnchanged(ctx, account.ID, account.Credentials, account.ProxyID, "caller transaction rollback")
			},
		},
		{
			name: "oauth refresh temporary quarantine",
			mutate: func(ctx context.Context, repo *accountRepository, account *service.Account, _ service.GrokCredentialMutationSnapshot) (bool, error) {
				return repo.SetGrokOAuthRefreshTempUnschedulableIfCredentialsUnchanged(ctx, account.ID, account.Credentials, account.ProxyID, time.Now().UTC().Add(time.Minute), "caller transaction rollback")
			},
		},
		{
			name: "credential temporary quarantine",
			mutate: func(ctx context.Context, repo *accountRepository, account *service.Account, snapshot service.GrokCredentialMutationSnapshot) (bool, error) {
				return repo.SetGrokCredentialTempUnschedulableIfMatch(ctx, account.ID, snapshot, time.Now().UTC().Add(time.Minute), "caller transaction rollback")
			},
		},
	}

	for index, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			account := mustCreateAccount(t, integrationEntClient, &service.Account{
				Name:        fmt.Sprintf("grok-cas-outer-transaction-%d", index),
				Platform:    service.PlatformGrok,
				Type:        service.AccountTypeOAuth,
				Credentials: map[string]any{"access_token": fmt.Sprintf("outer-transaction-token-%d", index)},
				Status:      service.StatusActive,
				Schedulable: true,
			})
			t.Cleanup(func() {
				_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM scheduler_outbox WHERE account_id = $1", account.ID)
				_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", account.ID)
			})
			_, err := integrationDB.ExecContext(ctx, "DELETE FROM scheduler_outbox WHERE account_id = $1", account.ID)
			require.NoError(t, err)

			tx, err := integrationEntClient.Tx(ctx)
			require.NoError(t, err)
			rolledBack := false
			t.Cleanup(func() {
				if !rolledBack {
					_ = tx.Rollback()
				}
			})
			deadlineCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			txCtx := dbent.NewTxContext(deadlineCtx, tx)
			var lockedID int64
			require.NoError(t, scanSingleRow(txCtx, tx.Client(), "SELECT id FROM accounts WHERE id = $1 FOR UPDATE", []any{account.ID}, &lockedID))

			credentialsJSON, err := json.Marshal(account.Credentials)
			require.NoError(t, err)
			cacheRecorder := &schedulerCacheRecorder{}
			repo := newAccountRepositoryWithSQLAndProtector(integrationEntClient, integrationDB, cacheRecorder, mustAccountCredentialProtectorForTest(t))
			applied, err := testCase.mutate(txCtx, repo, account, service.GrokCredentialMutationSnapshot{
				CredentialsJSON: string(credentialsJSON),
				AccessToken:     fmt.Sprintf("outer-transaction-token-%d", index),
			})
			require.NoError(t, err)
			require.True(t, applied)
			require.Empty(t, cacheRecorder.setAccounts, "caller-owned transaction must not publish cache state before commit")

			require.NoError(t, tx.Rollback())
			rolledBack = true
			got, err := integrationAccountRepository(t, integrationEntClient, integrationDB, nil).GetByID(ctx, account.ID)
			require.NoError(t, err)
			require.Equal(t, service.StatusActive, got.Status)
			require.True(t, got.Schedulable)
			require.Empty(t, got.ErrorMessage)
			require.Equal(t, account.Credentials, got.Credentials)
			require.Nil(t, got.TempUnschedulableUntil)

			var outboxCount int
			require.NoError(t, scanSingleRow(ctx, integrationDB, "SELECT COUNT(*) FROM scheduler_outbox WHERE account_id = $1", []any{account.ID}, &outboxCount))
			require.Zero(t, outboxCount)
		})
	}
}
