package repository

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountEntityToServiceMappingCharacterization(t *testing.T) {
	protector := mustAccountCredentialProtectorForTest(t)
	storedCredentials, err := protector.seal(42, map[string]any{
		"api_key": "mapping-test-secret",
		"nested":  map[string]any{"region": "test"},
	})
	require.NoError(t, err)

	notes := "mapping notes"
	errorMessage := "upstream unavailable"
	tempReason := "cooldown"
	sessionStatus := "active"
	proxyID := int64(7)
	fallbackProxyID := int64(8)
	loadFactor := 3
	parentID := int64(41)
	createdAt := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	lastUsedAt := createdAt.Add(-time.Minute)
	expiresAt := createdAt.Add(time.Hour)
	rateLimitedAt := createdAt.Add(2 * time.Minute)
	rateLimitResetAt := createdAt.Add(3 * time.Minute)
	overloadUntil := createdAt.Add(4 * time.Minute)
	tempUntil := createdAt.Add(5 * time.Minute)
	sessionStart := createdAt.Add(-5 * time.Minute)
	sessionEnd := createdAt.Add(25 * time.Minute)

	entity := &dbent.Account{
		ID:                      42,
		Name:                    "mapping-account",
		Notes:                   &notes,
		Platform:                service.PlatformOpenAI,
		Type:                    service.AccountTypeAPIKey,
		Credentials:             storedCredentials,
		Extra:                   map[string]any{"region": "west", "enabled": true},
		ProxyID:                 &proxyID,
		ProxyFallbackOriginID:   &fallbackProxyID,
		Concurrency:             12,
		LoadFactor:              &loadFactor,
		Priority:                4,
		RateMultiplier:          1.25,
		Status:                  service.StatusActive,
		ErrorMessage:            &errorMessage,
		LastUsedAt:              &lastUsedAt,
		ExpiresAt:               &expiresAt,
		AutoPauseOnExpired:      true,
		CreatedAt:               createdAt,
		UpdatedAt:               updatedAt,
		Schedulable:             true,
		RateLimitedAt:           &rateLimitedAt,
		RateLimitResetAt:        &rateLimitResetAt,
		OverloadUntil:           &overloadUntil,
		TempUnschedulableUntil:  &tempUntil,
		TempUnschedulableReason: &tempReason,
		SessionWindowStart:      &sessionStart,
		SessionWindowEnd:        &sessionEnd,
		SessionWindowStatus:     &sessionStatus,
		ParentAccountID:         &parentID,
	}

	repo := &accountRepository{protector: protector}
	got, err := repo.accountEntityToService(entity)
	require.NoError(t, err)
	require.Equal(t, &service.Account{
		ID:                      42,
		Name:                    "mapping-account",
		Notes:                   &notes,
		Platform:                service.PlatformOpenAI,
		Type:                    service.AccountTypeAPIKey,
		Credentials:             map[string]any{"api_key": "mapping-test-secret", "nested": map[string]any{"region": "test"}},
		Extra:                   map[string]any{"region": "west", "enabled": true},
		ProxyID:                 &proxyID,
		ProxyFallbackOriginID:   &fallbackProxyID,
		Concurrency:             12,
		Priority:                4,
		RateMultiplier:          float64Ptr(1.25),
		LoadFactor:              &loadFactor,
		Status:                  service.StatusActive,
		ErrorMessage:            errorMessage,
		LastUsedAt:              &lastUsedAt,
		ExpiresAt:               &expiresAt,
		AutoPauseOnExpired:      true,
		CreatedAt:               createdAt,
		UpdatedAt:               updatedAt,
		Schedulable:             true,
		RateLimitedAt:           &rateLimitedAt,
		RateLimitResetAt:        &rateLimitResetAt,
		OverloadUntil:           &overloadUntil,
		TempUnschedulableUntil:  &tempUntil,
		TempUnschedulableReason: tempReason,
		SessionWindowStart:      &sessionStart,
		SessionWindowEnd:        &sessionEnd,
		SessionWindowStatus:     sessionStatus,
		ParentAccountID:         &parentID,
	}, got)

	got.Extra["region"] = "mutated"
	require.Equal(t, "west", entity.Extra["region"], "top-level Extra map must not alias the Ent entity")
	require.NotContains(t, got.Credentials, accountCredentialEnvelopeField)
}

func TestAccountEntityToServiceMappingFailsClosedWithoutCredentialProtector(t *testing.T) {
	repo := &accountRepository{}
	got, err := repo.accountEntityToService(&dbent.Account{ID: 9, Credentials: map[string]any{}})
	require.Nil(t, got)
	require.ErrorContains(t, err, "data-encryption keyring is required for account credentials")

	got, err = repo.accountEntityToService(nil)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestAccountEntityToServiceRedactedCharacterization(t *testing.T) {
	entity := &dbent.Account{
		ID:             12,
		Name:           "redacted-account",
		Credentials:    map[string]any{"api_key": "must-not-escape"},
		Extra:          map[string]any{"safe": true},
		RateMultiplier: 0.75,
	}

	got := accountEntityToServiceRedacted(entity)
	require.Equal(t, int64(12), got.ID)
	require.Empty(t, got.Credentials)
	require.Equal(t, map[string]any{"safe": true}, got.Extra)
	require.Equal(t, 0.75, *got.RateMultiplier)

	got.Extra["safe"] = false
	require.Equal(t, true, entity.Extra["safe"], "redacted projection must copy the top-level Extra map")
	require.Nil(t, accountEntityToServiceRedacted(nil))
}

func TestAccountMappingLoaderInputNormalizationCharacterization(t *testing.T) {
	require.Nil(t, uniquePositiveInt64s(nil))
	require.Equal(t, []int64{3, 2, 1}, uniquePositiveInt64s([]int64{0, 3, -1, 2, 3, 1, 2}))

	repo := &accountRepository{}
	accounts, err := repo.accountsToService(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, accounts)
	require.Empty(t, accounts)
}

func float64Ptr(value float64) *float64 {
	return &value
}
