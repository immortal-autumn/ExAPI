//go:build unit

package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountTestService_AntigravityUnsupportedModelIsClassifiedAndRedacted(t *testing.T) {
	ensureGinTestMode()

	account := &Account{
		ID:          71,
		Name:        "antigravity-oauth",
		Platform:    PlatformAntigravity,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "access-token-secret",
			"project_id":    "project-71",
			"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"model_mapping": map[string]any{"claude-stale": "claude-stale"},
		},
	}
	repo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"status":"INVALID_ARGUMENT","message":"model not found: provider-secret"}}`)),
	}}
	tokenProvider := NewAntigravityTokenProvider(repo, nil, nil)
	gateway := &AntigravityGatewayService{
		accountRepo:   repo,
		tokenProvider: tokenProvider,
		httpUpstream:  upstream,
	}
	svc := &AccountTestService{accountRepo: repo, antigravityGatewayService: gateway}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/71/test", nil)
	err := svc.TestAccountConnection(ctx, account.ID, "claude-stale", "", AccountTestModeDefault)
	require.Error(t, err)
	_, reason := classifyAccountTestProbeError(err)
	require.Equal(t, AccountTestProbeReasonModelUnsupported, reason)
	require.NotContains(t, recorder.Body.String(), "provider-secret")
	require.NotContains(t, recorder.Body.String(), "access-token-secret")
	require.Contains(t, recorder.Body.String(), "model_unsupported")
}
