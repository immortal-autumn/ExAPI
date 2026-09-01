//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type apiKeyCreateRepoStub struct {
	createCalls int
	existsCalls int
}

func (r *apiKeyCreateRepoStub) Create(_ context.Context, key *service.APIKey) error {
	r.createCalls++
	key.ID = 42
	key.KeyPrefix = "sk-test-"
	key.CreatedAt = time.Now().UTC()
	key.UpdatedAt = key.CreatedAt
	return nil
}
func (r *apiKeyCreateRepoStub) GetByID(context.Context, int64) (*service.APIKey, error) {
	panic("unexpected GetByID call")
}
func (r *apiKeyCreateRepoStub) GetKeyAndOwnerID(context.Context, int64) (string, int64, error) {
	panic("unexpected GetKeyAndOwnerID call")
}
func (r *apiKeyCreateRepoStub) GetByKey(context.Context, string) (*service.APIKey, error) {
	panic("unexpected GetByKey call")
}
func (r *apiKeyCreateRepoStub) GetByKeyForAuth(context.Context, string) (*service.APIKey, error) {
	panic("unexpected GetByKeyForAuth call")
}
func (r *apiKeyCreateRepoStub) Update(context.Context, *service.APIKey, service.APIKeyUpdateFields) error {
	panic("unexpected Update call")
}
func (r *apiKeyCreateRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (r *apiKeyCreateRepoStub) DeleteWithAudit(context.Context, int64) error {
	panic("unexpected DeleteWithAudit call")
}
func (r *apiKeyCreateRepoStub) ListByUserID(context.Context, int64, pagination.PaginationParams, service.APIKeyListFilters) ([]service.APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserID call")
}
func (r *apiKeyCreateRepoStub) VerifyOwnership(context.Context, int64, []int64) ([]int64, error) {
	panic("unexpected VerifyOwnership call")
}
func (r *apiKeyCreateRepoStub) CountByUserID(context.Context, int64) (int64, error) {
	panic("unexpected CountByUserID call")
}
func (r *apiKeyCreateRepoStub) ExistsByKey(_ context.Context, _ string) (bool, error) {
	r.existsCalls++
	return false, nil
}
func (r *apiKeyCreateRepoStub) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]service.APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByGroupID call")
}
func (r *apiKeyCreateRepoStub) SearchAPIKeys(context.Context, int64, string, int) ([]service.APIKey, error) {
	panic("unexpected SearchAPIKeys call")
}
func (r *apiKeyCreateRepoStub) ClearGroupIDByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected ClearGroupIDByGroupID call")
}
func (r *apiKeyCreateRepoStub) UpdateGroupIDByUserAndGroup(context.Context, int64, int64, int64) (int64, error) {
	panic("unexpected UpdateGroupIDByUserAndGroup call")
}
func (r *apiKeyCreateRepoStub) CountByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected CountByGroupID call")
}
func (r *apiKeyCreateRepoStub) IncrementQuotaUsed(context.Context, int64, float64) (float64, error) {
	panic("unexpected IncrementQuotaUsed call")
}
func (r *apiKeyCreateRepoStub) UpdateLastUsed(context.Context, int64, time.Time) error {
	panic("unexpected UpdateLastUsed call")
}
func (r *apiKeyCreateRepoStub) IncrementRateLimitUsage(context.Context, int64, float64) error {
	panic("unexpected IncrementRateLimitUsage call")
}
func (r *apiKeyCreateRepoStub) ResetRateLimitWindows(context.Context, int64) error {
	panic("unexpected ResetRateLimitWindows call")
}
func (r *apiKeyCreateRepoStub) GetRateLimitData(context.Context, int64) (*service.APIKeyRateLimitData, error) {
	panic("unexpected GetRateLimitData call")
}

func TestAPIKeyHandlerCreateIdempotencyRedactsSecretOnReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.SetDefaultIdempotencyCoordinator(nil)
	t.Cleanup(func() {
		service.SetDefaultIdempotencyCoordinator(nil)
	})

	repo := newUserMemoryIdempotencyRepoStub()
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(repo, service.IdempotencyConfig{
		DefaultTTL:           time.Hour,
		SystemOperationTTL:   time.Hour,
		ProcessingTimeout:    time.Second,
		FailedRetryBackoff:   time.Second,
		MaxStoredResponseLen: 64 * 1024,
		ObserveOnly:          false,
	}))

	apiKeyRepo := &apiKeyCreateRepoStub{}
	userRepo := &userHandlerRepoStub{
		user: &service.User{
			ID:       11,
			Email:    "operator@example.com",
			Username: "operator",
			Status:   service.StatusActive,
			Role:     service.RoleUser,
		},
	}
	handler := NewAPIKeyHandler(service.NewAPIKeyService(apiKeyRepo, userRepo, nil, nil, nil, nil, &config.Config{}))

	const requestBody = `{"name":"operator-key","custom_key":"sk-test-created-secret"}`
	doRequest := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys", strings.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "create-key-1")

		c, _ := gin.CreateTestContext(rec)
		c.Request = req
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 11})

		handler.Create(c)
		return rec
	}

	first := doRequest()
	require.Equal(t, http.StatusOK, first.Code)
	require.Empty(t, first.Header().Get("X-Idempotency-Replayed"))

	var firstPayload struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &firstPayload))
	require.Equal(t, "sk-test-created-secret", firstPayload.Data["key"])
	require.Equal(t, "operator-key", firstPayload.Data["name"])

	stored := repo.data["user.api_keys.create|"+service.HashIdempotencyKey("create-key-1")]
	require.NotNil(t, stored)
	require.NotNil(t, stored.ResponseBody)
	require.NotContains(t, *stored.ResponseBody, "sk-test-created-secret")

	second := doRequest()
	require.Equal(t, http.StatusOK, second.Code)
	require.Equal(t, "true", second.Header().Get("X-Idempotency-Replayed"))
	require.Equal(t, 1, apiKeyRepo.createCalls)
	require.Equal(t, 1, apiKeyRepo.existsCalls)

	var secondPayload struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &secondPayload))
	_, hasKey := secondPayload.Data["key"]
	require.False(t, hasKey)
	require.NotContains(t, second.Body.String(), "sk-test-created-secret")
}
