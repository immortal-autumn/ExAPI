//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type accountTestProbeRepo struct {
	*mockAccountRepoForGemini
	updates         map[string]any
	writeContextErr error
}

type accountTestProbeModelSourceStub struct {
	model string
	ok    bool
}

func (s accountTestProbeModelSourceStub) CachedAntigravityProbeModel(int64) (string, bool) {
	return s.model, s.ok
}

func TestResolveAccountTestProbeModelUsesFreshAntigravitySnapshotOnlyForImplicitModel(t *testing.T) {
	t.Parallel()

	account := &Account{ID: 42, Platform: PlatformAntigravity, Type: AccountTypeOAuth}
	source := accountTestProbeModelSourceStub{model: "gemini-2.5-pro", ok: true}

	require.Equal(t, "gemini-2.5-pro", resolveAccountTestProbeModelWithSource(source, account, "", AccountTestModeDefault))
	// An operator-selected model is authoritative, even when the fresh snapshot
	// advertises a different model.
	require.Equal(t, "claude-sonnet-4-5", resolveAccountTestProbeModelWithSource(source, account, "claude-sonnet-4-5", AccountTestModeDefault))
	whitelisted := &Account{ID: 44, Platform: PlatformAntigravity, Type: AccountTypeOAuth, Credentials: map[string]any{
		"model_mapping": map[string]any{"claude-sonnet-4-5": "claude-sonnet-4-5"},
	}}
	require.Equal(t, "claude-sonnet-4-5", resolveAccountTestProbeModelWithSource(source, whitelisted, "", AccountTestModeDefault), "unsupported fresh models must not bypass the account whitelist")
	// Other platforms retain their existing fixed defaults and never consult the
	// Antigravity-only source.
	openAI := &Account{ID: 43, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	require.NotEqual(t, "gemini-2.5-pro", resolveAccountTestProbeModelWithSource(source, openAI, "", AccountTestModeDefault))
}

func (r *accountTestProbeRepo) UpdateAccountTestProbe(ctx context.Context, _ *Account, snapshot map[string]any) error {
	r.writeContextErr = ctx.Err()
	r.updates = map[string]any{AccountTestProbeExtraKey: snapshot}
	return nil
}

func TestClassifyAccountTestProbeError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		statusCode int
		reason     string
	}{
		{
			name:       "provider quota exhaustion",
			err:        errors.New("API 返回 429: RESOURCE_EXHAUSTED"),
			statusCode: 429,
			reason:     AccountTestProbeReasonQuotaExhausted,
		},
		{
			name:   "antigravity friendly rate limit",
			err:    errors.New("该账号模型 claude-sonnet-4-5 当前限流中，请稍后重试"),
			reason: AccountTestProbeReasonQuotaExhausted,
		},
		{
			name:   "authentication failure",
			err:    errors.New("获取 access_token 失败: invalid_grant"),
			reason: AccountTestProbeReasonAuthFailed,
		},
		{
			name:   "network failure",
			err:    errors.New("upstream request failed after retries"),
			reason: AccountTestProbeReasonRequestFailed,
		},
		{
			name:       "unsupported advertised model",
			err:        newAccountTestProbeHTTPError(http.StatusBadRequest, []byte(`{"error":{"status":"INVALID_ARGUMENT","message":"model not found: provider-secret"}}`), "claude-stale"),
			statusCode: http.StatusBadRequest,
			reason:     AccountTestProbeReasonModelUnsupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statusCode, reason := classifyAccountTestProbeError(tt.err)
			require.Equal(t, tt.statusCode, statusCode)
			require.Equal(t, tt.reason, reason)
		})
	}
}

func TestAccountTestProbeHTTPErrorDoesNotExposeProviderBody(t *testing.T) {
	t.Parallel()

	providerBody := []byte(`{"error":{"status":"RESOURCE_EXHAUSTED","message":"quota exhausted; access_token=provider-secret"}}`)
	err := newAccountTestProbeHTTPError(http.StatusTooManyRequests, providerBody, "claude-sonnet-4-5")
	require.Equal(t, AccountTestProbeReasonQuotaExhausted, func() string {
		_, reason := classifyAccountTestProbeError(err)
		return reason
	}())
	require.NotContains(t, err.Error(), "provider-secret")
	require.NotContains(t, err.Error(), "RESOURCE_EXHAUSTED")
	require.Contains(t, err.Error(), "429")
}

func TestSanitizeAccountTestErrorMessageRedactsCredentialLikeFields(t *testing.T) {
	t.Parallel()

	message := sanitizeAccountTestErrorMessage(`provider failed: access_token=secret-a authorization:Bearer secret-b api_key secret-c`)
	require.NotContains(t, message, "secret-a")
	require.NotContains(t, message, "secret-b")
	require.NotContains(t, message, "secret-c")
	require.NotContains(t, message, "access_token")
}

func TestAccountTestProbeCredentialIdentityIgnoresOAuthRotation(t *testing.T) {
	t.Parallel()

	before := AccountTestProbeCredentialIdentity(map[string]any{
		"access_token":  "old-access",
		"refresh_token": "stable-refresh",
		"expires_at":    "2026-08-19T12:00:00Z",
		"project_id":    "project",
	})
	after := AccountTestProbeCredentialIdentity(map[string]any{
		"access_token":  "new-access",
		"refresh_token": "stable-refresh",
		"expires_at":    "2026-08-19T13:00:00Z",
		"project_id":    "project",
	})
	require.Equal(t, before, after)

	reauthorized := AccountTestProbeCredentialIdentity(map[string]any{
		"access_token":  "new-access",
		"refresh_token": "new-refresh",
		"project_id":    "project",
	})
	require.NotEqual(t, before, reauthorized)
}

func TestPersistAccountTestProbeIsCredentialFreeAndDoesNotChangeScheduling(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:          22,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"access_token": "secret"},
		Extra:       map[string]any{"allow_overages": false},
	}
	repo := &accountTestProbeRepo{mockAccountRepoForGemini: &mockAccountRepoForGemini{}}
	svc := &AccountTestService{accountRepo: repo}

	svc.persistAccountTestProbe(
		context.Background(),
		account,
		"claude-sonnet-4-5",
		errors.New("API 返回 429: {\"error\":{\"status\":\"RESOURCE_EXHAUSTED\"}}"),
	)

	snapshot, ok := account.Extra[AccountTestProbeExtraKey].(map[string]any)
	require.True(t, ok)
	require.Equal(t, AccountTestProbeStatusFailed, snapshot["status"])
	require.Equal(t, AccountTestProbeReasonQuotaExhausted, snapshot["reason"])
	require.Equal(t, 429, snapshot["http_status"])
	require.NotContains(t, snapshot, "error")
	require.NotContains(t, snapshot, "access_token")
	require.Equal(t, true, account.Schedulable)
	require.Equal(t, StatusActive, account.Status)
	require.Equal(t, snapshot, repo.updates[AccountTestProbeExtraKey])

	checkedAt, ok := snapshot["checked_at"].(string)
	require.True(t, ok)
	_, err := time.Parse(time.RFC3339, checkedAt)
	require.NoError(t, err)
}

func TestBackgroundAccountTestDoesNotOverwriteManualProbe(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       22,
		Status:   StatusActive,
		Platform: PlatformAnthropic,
		Extra:    map[string]any{"synthetic_ui_test": true},
	}
	repo := &accountTestProbeRepo{mockAccountRepoForGemini: &mockAccountRepoForGemini{
		accountsByID: map[int64]*Account{account.ID: account},
	}}
	svc := &AccountTestService{accountRepo: repo}

	newContext := func(ctx context.Context) *gin.Context {
		recorder := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(recorder)
		ginCtx.Request = httptest.NewRequest(http.MethodPost, "/account-test", nil).WithContext(ctx)
		return ginCtx
	}

	require.NoError(t, svc.testAccountConnection(newContext(context.Background()), account.ID, "", "", AccountTestModeDefault, false))
	require.Nil(t, repo.updates, "scheduled/background tests must not replace the manual probe snapshot")

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	require.NoError(t, svc.TestAccountConnection(newContext(canceledCtx), account.ID, "", "", AccountTestModeDefault))
	require.NotNil(t, repo.updates)
	require.NoError(t, repo.writeContextErr, "manual probe persistence must survive request cancellation")
}
