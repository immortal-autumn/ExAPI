package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userStoreUnavailableRepoStub struct{}

func (userStoreUnavailableRepoStub) CreateProcessing(context.Context, *service.IdempotencyRecord) (bool, error) {
	return false, errors.New("store unavailable")
}
func (userStoreUnavailableRepoStub) GetByScopeAndKeyHash(context.Context, string, string) (*service.IdempotencyRecord, error) {
	return nil, errors.New("store unavailable")
}
func (userStoreUnavailableRepoStub) TryReclaim(context.Context, int64, string, time.Time, time.Time, time.Time) (bool, error) {
	return false, errors.New("store unavailable")
}
func (userStoreUnavailableRepoStub) ExtendProcessingLock(context.Context, int64, string, time.Time, time.Time) (bool, error) {
	return false, errors.New("store unavailable")
}
func (userStoreUnavailableRepoStub) MarkSucceeded(context.Context, int64, int, string, time.Time) error {
	return errors.New("store unavailable")
}
func (userStoreUnavailableRepoStub) MarkFailedRetryable(context.Context, int64, string, time.Time, time.Time) error {
	return errors.New("store unavailable")
}
func (userStoreUnavailableRepoStub) DeleteExpired(context.Context, time.Time, int) (int64, error) {
	return 0, errors.New("store unavailable")
}

type userMemoryIdempotencyRepoStub struct {
	mu     sync.Mutex
	nextID int64
	data   map[string]*service.IdempotencyRecord
}

func newUserMemoryIdempotencyRepoStub() *userMemoryIdempotencyRepoStub {
	return &userMemoryIdempotencyRepoStub{
		nextID: 1,
		data:   make(map[string]*service.IdempotencyRecord),
	}
}

func (r *userMemoryIdempotencyRepoStub) key(scope, keyHash string) string {
	return scope + "|" + keyHash
}

func (r *userMemoryIdempotencyRepoStub) clone(in *service.IdempotencyRecord) *service.IdempotencyRecord {
	if in == nil {
		return nil
	}
	out := *in
	if in.LockedUntil != nil {
		v := *in.LockedUntil
		out.LockedUntil = &v
	}
	if in.ResponseBody != nil {
		v := *in.ResponseBody
		out.ResponseBody = &v
	}
	if in.ResponseStatus != nil {
		v := *in.ResponseStatus
		out.ResponseStatus = &v
	}
	if in.ErrorReason != nil {
		v := *in.ErrorReason
		out.ErrorReason = &v
	}
	return &out
}

func (r *userMemoryIdempotencyRepoStub) CreateProcessing(_ context.Context, record *service.IdempotencyRecord) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := r.key(record.Scope, record.IdempotencyKeyHash)
	if _, ok := r.data[k]; ok {
		return false, nil
	}
	cp := r.clone(record)
	cp.ID = r.nextID
	r.nextID++
	r.data[k] = cp
	record.ID = cp.ID
	return true, nil
}

func (r *userMemoryIdempotencyRepoStub) GetByScopeAndKeyHash(_ context.Context, scope, keyHash string) (*service.IdempotencyRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.clone(r.data[r.key(scope, keyHash)]), nil
}

func (r *userMemoryIdempotencyRepoStub) TryReclaim(_ context.Context, id int64, fromStatus string, now, newLockedUntil, newExpiresAt time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.data {
		if rec.ID != id {
			continue
		}
		if rec.Status != fromStatus {
			return false, nil
		}
		if rec.LockedUntil != nil && rec.LockedUntil.After(now) {
			return false, nil
		}
		rec.Status = service.IdempotencyStatusProcessing
		rec.LockedUntil = &newLockedUntil
		rec.ExpiresAt = newExpiresAt
		rec.ErrorReason = nil
		return true, nil
	}
	return false, nil
}

func (r *userMemoryIdempotencyRepoStub) ExtendProcessingLock(_ context.Context, id int64, requestFingerprint string, newLockedUntil, newExpiresAt time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.data {
		if rec.ID != id {
			continue
		}
		if rec.Status != service.IdempotencyStatusProcessing || rec.RequestFingerprint != requestFingerprint {
			return false, nil
		}
		rec.LockedUntil = &newLockedUntil
		rec.ExpiresAt = newExpiresAt
		return true, nil
	}
	return false, nil
}

func (r *userMemoryIdempotencyRepoStub) MarkSucceeded(_ context.Context, id int64, responseStatus int, responseBody string, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.data {
		if rec.ID != id {
			continue
		}
		rec.Status = service.IdempotencyStatusSucceeded
		rec.LockedUntil = nil
		rec.ExpiresAt = expiresAt
		rec.ResponseStatus = &responseStatus
		rec.ResponseBody = &responseBody
		rec.ErrorReason = nil
		return nil
	}
	return nil
}

func (r *userMemoryIdempotencyRepoStub) MarkFailedRetryable(_ context.Context, id int64, errorReason string, lockedUntil, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.data {
		if rec.ID != id {
			continue
		}
		rec.Status = service.IdempotencyStatusFailedRetryable
		rec.LockedUntil = &lockedUntil
		rec.ExpiresAt = expiresAt
		rec.ErrorReason = &errorReason
		return nil
	}
	return nil
}

func (r *userMemoryIdempotencyRepoStub) DeleteExpired(_ context.Context, _ time.Time, _ int) (int64, error) {
	return 0, nil
}

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

func withUserSubject(userID int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
		c.Next()
	}
}

func TestExecuteUserIdempotentJSONFallbackWithoutCoordinator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.SetDefaultIdempotencyCoordinator(nil)

	var executed int
	router := gin.New()
	router.Use(withUserSubject(1))
	router.POST("/idempotent", func(c *gin.Context) {
		executeUserIdempotentJSON(c, "user.test.scope", map[string]any{"a": 1}, time.Minute, func(ctx context.Context) (any, error) {
			executed++
			return gin.H{"ok": true}, nil
		})
	})

	req := httptest.NewRequest(http.MethodPost, "/idempotent", bytes.NewBufferString(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, executed)
}

func TestExecuteUserIdempotentJSONFailCloseOnStoreUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(userStoreUnavailableRepoStub{}, service.DefaultIdempotencyConfig()))
	t.Cleanup(func() {
		service.SetDefaultIdempotencyCoordinator(nil)
	})

	var executed int
	router := gin.New()
	router.Use(withUserSubject(2))
	router.POST("/idempotent", func(c *gin.Context) {
		executeUserIdempotentJSON(c, "user.test.scope", map[string]any{"a": 1}, time.Minute, func(ctx context.Context) (any, error) {
			executed++
			return gin.H{"ok": true}, nil
		})
	})

	req := httptest.NewRequest(http.MethodPost, "/idempotent", bytes.NewBufferString(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "k1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, 0, executed)
}

func TestExecuteUserIdempotentJSONConcurrentRetrySingleSideEffectAndReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newUserMemoryIdempotencyRepoStub()
	cfg := service.DefaultIdempotencyConfig()
	cfg.ProcessingTimeout = 2 * time.Second
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(repo, cfg))
	t.Cleanup(func() {
		service.SetDefaultIdempotencyCoordinator(nil)
	})

	var executed atomic.Int32
	router := gin.New()
	router.Use(withUserSubject(3))
	router.POST("/idempotent", func(c *gin.Context) {
		executeUserIdempotentJSON(c, "user.test.scope", map[string]any{"a": 1}, time.Minute, func(ctx context.Context) (any, error) {
			executed.Add(1)
			time.Sleep(80 * time.Millisecond)
			return gin.H{"ok": true}, nil
		})
	})

	call := func() (int, http.Header) {
		req := httptest.NewRequest(http.MethodPost, "/idempotent", bytes.NewBufferString(`{"a":1}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "same-user-key")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code, rec.Header()
	}

	var status1, status2 int
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); status1, _ = call() }()
	go func() { defer wg.Done(); status2, _ = call() }()
	wg.Wait()

	require.Contains(t, []int{http.StatusOK, http.StatusConflict}, status1)
	require.Contains(t, []int{http.StatusOK, http.StatusConflict}, status2)
	require.Equal(t, int32(1), executed.Load())

	status3, headers3 := call()
	require.Equal(t, http.StatusOK, status3)
	require.Equal(t, "true", headers3.Get("X-Idempotency-Replayed"))
	require.Equal(t, int32(1), executed.Load())
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
