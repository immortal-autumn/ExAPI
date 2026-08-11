package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type privateUsageQuotaUpdater struct{}

func (privateUsageQuotaUpdater) UpdateQuotaUsed(context.Context, int64, float64) error { return nil }
func (privateUsageQuotaUpdater) UpdateRateLimitUsage(context.Context, int64, float64) error {
	return nil
}

func TestApplyPrivateOperationalUsagePreservesCountersWithoutCustomerCharges(t *testing.T) {
	subscriptionID := int64(91)
	repo := &openAIRecordUsageBillingRepoStub{}
	params := &postUsageBillingParams{
		Cost:                  &CostBreakdown{TotalCost: 2, ActualCost: 3},
		User:                  &User{ID: 11},
		APIKey:                &APIKey{ID: 22, Key: "secret", Quota: 100, RateLimit1d: 100},
		Account:               &Account{ID: 33},
		Subscription:          &UserSubscription{ID: subscriptionID},
		IsSubscriptionBill:    true,
		AccountRateMultiplier: 1,
		APIKeyService:         privateUsageQuotaUpdater{},
	}

	applied, err := applyPrivateOperationalUsage(
		context.Background(),
		"request-private-1",
		&UsageLog{Model: "gpt-5", BillingType: BillingTypeSubscription},
		params,
		&billingDeps{},
		repo,
	)

	require.NoError(t, err)
	require.True(t, applied)
	require.Equal(t, 1, repo.calls)
	require.NotNil(t, repo.lastCmd)
	require.Equal(t, BillingTypeOperational, repo.lastCmd.BillingType)
	require.Nil(t, repo.lastCmd.SubscriptionID)
	require.Zero(t, repo.lastCmd.SubscriptionCost)
	require.Zero(t, repo.lastCmd.BalanceCost)
	require.Equal(t, 3.0, repo.lastCmd.APIKeyQuotaCost)
	require.Equal(t, 3.0, repo.lastCmd.APIKeyRateLimitCost)
	require.NotEmpty(t, repo.lastCmd.RequestFingerprint, "idempotent apply still requires a stable fingerprint")
}

func TestApplyPrivateOperationalUsageFailsWhenTransactionalRepositoryIsMissing(t *testing.T) {
	applied, err := applyPrivateOperationalUsage(
		context.Background(),
		"request-private-2",
		&UsageLog{},
		&postUsageBillingParams{
			Cost:    &CostBreakdown{ActualCost: 1},
			User:    &User{ID: 1},
			APIKey:  &APIKey{ID: 2},
			Account: &Account{ID: 3},
		},
		&billingDeps{},
		nil,
	)
	require.False(t, applied)
	require.ErrorContains(t, err, "repository is unavailable")
}
