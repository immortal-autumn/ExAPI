//go:build unit

package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type authRepoStub struct {
	getByKeyForAuth func(ctx context.Context, key string) (*APIKey, error)
	getByID         func(ctx context.Context, id int64) (*APIKey, error)
	getKeyAndOwner  func(ctx context.Context, id int64) (string, int64, error)
	update          func(ctx context.Context, key *APIKey) error
	deleteWithAudit func(ctx context.Context, id int64) error

	listKeysByUserID  func(ctx context.Context, userID int64) ([]string, error)
	listKeysByGroupID func(ctx context.Context, groupID int64) ([]string, error)
}

func (s *authRepoStub) Create(ctx context.Context, key *APIKey) error {
	panic("unexpected Create call")
}

func (s *authRepoStub) GetByID(ctx context.Context, id int64) (*APIKey, error) {
	if s.getByID != nil {
		return s.getByID(ctx, id)
	}
	panic("unexpected GetByID call")
}

func (s *authRepoStub) GetKeyAndOwnerID(ctx context.Context, id int64) (string, int64, error) {
	if s.getKeyAndOwner != nil {
		return s.getKeyAndOwner(ctx, id)
	}
	panic("unexpected GetKeyAndOwnerID call")
}

func (s *authRepoStub) GetByKey(ctx context.Context, key string) (*APIKey, error) {
	panic("unexpected GetByKey call")
}

func (s *authRepoStub) GetByKeyForAuth(ctx context.Context, key string) (*APIKey, error) {
	if s.getByKeyForAuth == nil {
		panic("unexpected GetByKeyForAuth call")
	}
	return s.getByKeyForAuth(ctx, key)
}

func (s *authRepoStub) Update(ctx context.Context, key *APIKey) error {
	if s.update != nil {
		return s.update(ctx, key)
	}
	panic("unexpected Update call")
}

func (s *authRepoStub) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (s *authRepoStub) DeleteWithAudit(ctx context.Context, id int64) error {
	if s.deleteWithAudit != nil {
		return s.deleteWithAudit(ctx, id)
	}
	panic("unexpected DeleteWithAudit call")
}

func (s *authRepoStub) ListByUserID(ctx context.Context, userID int64, params pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserID call")
}

func (s *authRepoStub) VerifyOwnership(ctx context.Context, userID int64, apiKeyIDs []int64) ([]int64, error) {
	panic("unexpected VerifyOwnership call")
}

func (s *authRepoStub) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	panic("unexpected CountByUserID call")
}

func (s *authRepoStub) ExistsByKey(ctx context.Context, key string) (bool, error) {
	panic("unexpected ExistsByKey call")
}

func (s *authRepoStub) ListByGroupID(ctx context.Context, groupID int64, params pagination.PaginationParams) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByGroupID call")
}

func (s *authRepoStub) SearchAPIKeys(ctx context.Context, userID int64, keyword string, limit int) ([]APIKey, error) {
	panic("unexpected SearchAPIKeys call")
}

func (s *authRepoStub) ClearGroupIDByGroupID(ctx context.Context, groupID int64) (int64, error) {
	panic("unexpected ClearGroupIDByGroupID call")
}
func (s *authRepoStub) UpdateGroupIDByUserAndGroup(ctx context.Context, userID, oldGroupID, newGroupID int64) (int64, error) {
	panic("unexpected UpdateGroupIDByUserAndGroup call")
}

func (s *authRepoStub) CountByGroupID(ctx context.Context, groupID int64) (int64, error) {
	panic("unexpected CountByGroupID call")
}

func (s *authRepoStub) ListKeysByUserID(ctx context.Context, userID int64) ([]string, error) {
	if s.listKeysByUserID == nil {
		panic("unexpected ListKeysByUserID call")
	}
	return s.listKeysByUserID(ctx, userID)
}

func (s *authRepoStub) ListKeysByGroupID(ctx context.Context, groupID int64) ([]string, error) {
	if s.listKeysByGroupID == nil {
		panic("unexpected ListKeysByGroupID call")
	}
	return s.listKeysByGroupID(ctx, groupID)
}

func (s *authRepoStub) IncrementQuotaUsed(ctx context.Context, id int64, amount float64) (float64, error) {
	panic("unexpected IncrementQuotaUsed call")
}

func (s *authRepoStub) UpdateLastUsed(ctx context.Context, id int64, usedAt time.Time) error {
	panic("unexpected UpdateLastUsed call")
}
func (s *authRepoStub) IncrementRateLimitUsage(ctx context.Context, id int64, cost float64) error {
	panic("unexpected IncrementRateLimitUsage call")
}
func (s *authRepoStub) ResetRateLimitWindows(ctx context.Context, id int64) error {
	panic("unexpected ResetRateLimitWindows call")
}
func (s *authRepoStub) GetRateLimitData(ctx context.Context, id int64) (*APIKeyRateLimitData, error) {
	panic("unexpected GetRateLimitData call")
}

type authCacheStub struct {
	getAuthCache   func(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error)
	setAuthKeys    []string
	deleteAuthKeys []string
	publishedKeys  []string
}

type sharedAuthCacheGenerationState struct {
	mu         sync.Mutex
	generation uint64
	entries    map[string]*APIKeyAuthCacheEntry
}

type distributedAuthCacheStub struct {
	*authCacheStub
	shared     *sharedAuthCacheGenerationState
	publishErr error
}

type unavailableAuthGenerationCache struct {
	*authCacheStub
}

func (s *unavailableAuthGenerationCache) GetAuthCacheGeneration(context.Context) (uint64, error) {
	return 0, errors.New("generation store unavailable")
}

func (s *unavailableAuthGenerationCache) IncrementAuthCacheGeneration(context.Context) (uint64, error) {
	return 0, errors.New("generation store unavailable")
}

type failedIncrementAuthGenerationCache struct {
	*authCacheStub
	generation uint64
}

func (s *failedIncrementAuthGenerationCache) GetAuthCacheGeneration(context.Context) (uint64, error) {
	return s.generation, nil
}

func (s *failedIncrementAuthGenerationCache) IncrementAuthCacheGeneration(context.Context) (uint64, error) {
	return 0, errors.New("generation increment failed")
}

func (s *distributedAuthCacheStub) GetAuthCache(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error) {
	s.shared.mu.Lock()
	defer s.shared.mu.Unlock()
	entry, ok := s.shared.entries[key]
	if !ok {
		return nil, redis.Nil
	}
	clone := *entry
	return &clone, nil
}

func (s *distributedAuthCacheStub) SetAuthCache(ctx context.Context, key string, entry *APIKeyAuthCacheEntry, ttl time.Duration) error {
	s.shared.mu.Lock()
	defer s.shared.mu.Unlock()
	clone := *entry
	s.shared.entries[key] = &clone
	return nil
}

func (s *distributedAuthCacheStub) GetAuthCacheGeneration(ctx context.Context) (uint64, error) {
	s.shared.mu.Lock()
	defer s.shared.mu.Unlock()
	return s.shared.generation, nil
}

func (s *distributedAuthCacheStub) IncrementAuthCacheGeneration(ctx context.Context) (uint64, error) {
	s.shared.mu.Lock()
	defer s.shared.mu.Unlock()
	s.shared.generation++
	return s.shared.generation, nil
}

func (s *distributedAuthCacheStub) PublishAuthCacheInvalidation(ctx context.Context, cacheKey string) error {
	return s.publishErr
}

func (s *authCacheStub) GetCreateAttemptCount(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}

func (s *authCacheStub) IncrementCreateAttemptCount(ctx context.Context, userID int64) error {
	return nil
}

func (s *authCacheStub) DeleteCreateAttemptCount(ctx context.Context, userID int64) error {
	return nil
}

func (s *authCacheStub) IncrementDailyUsage(ctx context.Context, apiKey string) error {
	return nil
}

func (s *authCacheStub) SetDailyUsageExpiry(ctx context.Context, apiKey string, ttl time.Duration) error {
	return nil
}

func (s *authCacheStub) GetAuthCache(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error) {
	if s.getAuthCache == nil {
		return nil, redis.Nil
	}
	return s.getAuthCache(ctx, key)
}

func (s *authCacheStub) SetAuthCache(ctx context.Context, key string, entry *APIKeyAuthCacheEntry, ttl time.Duration) error {
	s.setAuthKeys = append(s.setAuthKeys, key)
	return nil
}

func (s *authCacheStub) DeleteAuthCache(ctx context.Context, key string) error {
	s.deleteAuthKeys = append(s.deleteAuthKeys, key)
	return nil
}

func (s *authCacheStub) PublishAuthCacheInvalidation(ctx context.Context, cacheKey string) error {
	s.publishedKeys = append(s.publishedKeys, cacheKey)
	return nil
}

func (s *authCacheStub) SubscribeAuthCacheInvalidation(ctx context.Context, handler func(cacheKey string)) error {
	return nil
}

func TestAPIKeyService_GetByKey_UsesL2Cache(t *testing.T) {
	cache := &authCacheStub{}
	repo := &authRepoStub{
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			return nil, errors.New("unexpected repo call")
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L2TTLSeconds:       60,
			NegativeTTLSeconds: 30,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)

	groupID := int64(9)
	cacheEntry := &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{
			Version:  apiKeyAuthSnapshotVersion,
			APIKeyID: 1,
			UserID:   2,
			GroupID:  &groupID,
			Status:   StatusActive,
			User: APIKeyAuthUserSnapshot{
				ID:          2,
				Status:      StatusActive,
				Role:        RoleUser,
				Balance:     10,
				Concurrency: 3,
			},
			Group: &APIKeyAuthGroupSnapshot{
				ID:                  groupID,
				Name:                "g",
				Platform:            PlatformAnthropic,
				Status:              StatusActive,
				SubscriptionType:    SubscriptionTypeStandard,
				RateMultiplier:      1,
				ModelRoutingEnabled: true,
				ModelRouting: map[string][]int64{
					"claude-opus-*": {1, 2},
				},
			},
		},
	}
	cache.getAuthCache = func(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error) {
		return cacheEntry, nil
	}

	apiKey, err := svc.GetByKey(context.Background(), "k1")
	require.NoError(t, err)
	require.Equal(t, int64(1), apiKey.ID)
	require.Equal(t, int64(2), apiKey.User.ID)
	require.Equal(t, groupID, apiKey.Group.ID)
	require.True(t, apiKey.Group.ModelRoutingEnabled)
	require.Equal(t, map[string][]int64{"claude-opus-*": {1, 2}}, apiKey.Group.ModelRouting)
}

func TestAPIKeyService_SnapshotRoundTrip_PreservesMessagesDispatchModelConfig(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})
	groupID := int64(9)
	apiKey := &APIKey{
		ID:      1,
		UserID:  2,
		GroupID: &groupID,
		Key:     "k-roundtrip",
		Name:    "Audit Key",
		Status:  StatusActive,
		User: &User{
			ID:          2,
			Status:      StatusActive,
			Role:        RoleUser,
			Balance:     10,
			Concurrency: 3,
		},
		Group: &Group{
			ID:                    groupID,
			Name:                  "openai",
			Platform:              PlatformOpenAI,
			Status:                StatusActive,
			SubscriptionType:      SubscriptionTypeStandard,
			RateMultiplier:        1,
			AllowMessagesDispatch: true,
			DefaultMappedModel:    "gpt-5.4",
			MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
				OpusMappedModel:   "gpt-5.4-nano",
				SonnetMappedModel: "gpt-5.3-codex",
				HaikuMappedModel:  "gpt-5.4-mini",
				ExactModelMappings: map[string]string{
					"claude-sonnet-4.5": "gpt-5.4-nano",
				},
			},
		},
	}

	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	roundTrip := svc.snapshotToAPIKey(apiKey.Key, snapshot)

	require.NotNil(t, roundTrip)
	require.Equal(t, apiKey.Name, roundTrip.Name)
	require.NotNil(t, roundTrip.Group)
	require.Equal(t, apiKey.Group.MessagesDispatchModelConfig, roundTrip.Group.MessagesDispatchModelConfig)
}

func TestAPIKeyService_GetByKey_IgnoresLegacyAuthCacheSnapshotWithoutMessagesDispatchConfig(t *testing.T) {
	cache := &authCacheStub{}
	var repoCalls int32
	repo := &authRepoStub{
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			atomic.AddInt32(&repoCalls, 1)
			groupID := int64(9)
			return &APIKey{
				ID:      1,
				UserID:  2,
				GroupID: &groupID,
				Status:  StatusActive,
				User: &User{
					ID:          2,
					Status:      StatusActive,
					Role:        RoleUser,
					Balance:     10,
					Concurrency: 3,
				},
				Group: &Group{
					ID:                    groupID,
					Name:                  "openai",
					Platform:              PlatformOpenAI,
					Status:                StatusActive,
					Hydrated:              true,
					SubscriptionType:      SubscriptionTypeStandard,
					RateMultiplier:        1,
					AllowMessagesDispatch: true,
					DefaultMappedModel:    "gpt-5.4",
					MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
						OpusMappedModel: "gpt-5.4-nano",
					},
				},
			}, nil
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L2TTLSeconds: 60,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)

	groupID := int64(9)
	cache.getAuthCache = func(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error) {
		return &APIKeyAuthCacheEntry{
			Snapshot: &APIKeyAuthSnapshot{
				APIKeyID: 1,
				UserID:   2,
				GroupID:  &groupID,
				Status:   StatusActive,
				User: APIKeyAuthUserSnapshot{
					ID:          2,
					Status:      StatusActive,
					Role:        RoleUser,
					Balance:     10,
					Concurrency: 3,
				},
				Group: &APIKeyAuthGroupSnapshot{
					ID:                    groupID,
					Name:                  "openai",
					Platform:              PlatformOpenAI,
					Status:                StatusActive,
					SubscriptionType:      SubscriptionTypeStandard,
					RateMultiplier:        1,
					AllowMessagesDispatch: true,
					DefaultMappedModel:    "gpt-5.4",
				},
			},
		}, nil
	}

	apiKey, err := svc.GetByKey(context.Background(), "k-legacy")
	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&repoCalls))
	require.NotNil(t, apiKey.Group)
	require.Equal(t, "gpt-5.4-nano", apiKey.Group.MessagesDispatchModelConfig.OpusMappedModel)
}

func TestAPIKeyService_InvalidationByUserAndGroupDoesNotEnumerateRawKeys(t *testing.T) {
	repo := &authRepoStub{
		listKeysByUserID: func(context.Context, int64) ([]string, error) {
			t.Fatal("user invalidation enumerated raw API keys")
			return nil, nil
		},
		listKeysByGroupID: func(context.Context, int64) ([]string, error) {
			t.Fatal("group invalidation enumerated raw API keys")
			return nil, nil
		},
	}
	cache := &authCacheStub{}
	cfg := &config.Config{APIKeyAuth: config.APIKeyAuthCacheConfig{L1Size: 100, L1TTLSeconds: 60, L2TTLSeconds: 60}}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)

	before := svc.authCacheKey("test-key")
	svc.InvalidateAuthCacheByUserID(context.Background(), 7)
	afterUser := svc.authCacheKey("test-key")
	svc.InvalidateAuthCacheByGroupID(context.Background(), 9)
	afterGroup := svc.authCacheKey("test-key")

	require.NotEqual(t, before, afterUser)
	require.NotEqual(t, afterUser, afterGroup)
}

func TestAPIKeyService_GetByKey_NegativeCache(t *testing.T) {
	cache := &authCacheStub{}
	repo := &authRepoStub{
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			return nil, errors.New("unexpected repo call")
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L2TTLSeconds:       60,
			NegativeTTLSeconds: 30,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)
	cache.getAuthCache = func(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error) {
		return &APIKeyAuthCacheEntry{NotFound: true}, nil
	}

	_, err := svc.GetByKey(context.Background(), "missing")
	require.ErrorIs(t, err, ErrAPIKeyNotFound)
}

func TestAPIKeyService_GetByKey_CacheMissStoresL2(t *testing.T) {
	cache := &authCacheStub{}
	repo := &authRepoStub{
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			return &APIKey{
				ID:     5,
				UserID: 7,
				Status: StatusActive,
				User: &User{
					ID:          7,
					Status:      StatusActive,
					Role:        RoleUser,
					Balance:     12,
					Concurrency: 2,
				},
			}, nil
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L2TTLSeconds:       60,
			NegativeTTLSeconds: 30,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)
	cache.getAuthCache = func(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error) {
		return nil, redis.Nil
	}

	apiKey, err := svc.GetByKey(context.Background(), "k2")
	require.NoError(t, err)
	require.Equal(t, int64(5), apiKey.ID)
	require.Len(t, cache.setAuthKeys, 1)
}

func TestAPIKeyService_GetByKey_UsesL1Cache(t *testing.T) {
	var calls int32
	cache := &authCacheStub{}
	repo := &authRepoStub{
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			atomic.AddInt32(&calls, 1)
			return &APIKey{
				ID:     21,
				UserID: 3,
				Status: StatusActive,
				User: &User{
					ID:          3,
					Status:      StatusActive,
					Role:        RoleUser,
					Balance:     5,
					Concurrency: 2,
				},
			}, nil
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L1Size:       1000,
			L1TTLSeconds: 60,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)
	require.NotNil(t, svc.authCacheL1)

	_, err := svc.GetByKey(context.Background(), "k-l1")
	require.NoError(t, err)
	svc.authCacheL1.Wait()
	cacheKey := svc.authCacheKey("k-l1")
	_, ok := svc.authCacheL1.Get(cacheKey)
	require.True(t, ok)
	_, err = svc.GetByKey(context.Background(), "k-l1")
	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestAPIKeyService_InvalidateAuthCacheByUserID(t *testing.T) {
	cache := &authCacheStub{}
	svc := NewAPIKeyService(&authRepoStub{}, nil, nil, nil, nil, cache, &config.Config{})
	before := svc.authCacheKey("test-key")

	svc.InvalidateAuthCacheByUserID(context.Background(), 7)
	require.NotEqual(t, before, svc.authCacheKey("test-key"))
	require.Equal(t, []string{authCacheInvalidateAll}, cache.publishedKeys)
	require.Empty(t, cache.deleteAuthKeys)
}

func TestAPIKeyService_DurableGenerationRejectsStaleProtectedKeyAfterMissedPubSubAndRestart(t *testing.T) {
	ctx := context.Background()
	shared := &sharedAuthCacheGenerationState{entries: make(map[string]*APIKeyAuthCacheEntry)}
	var exhausted atomic.Bool
	var repoCalls atomic.Int32
	newService := func() *APIKeyService {
		cache := &distributedAuthCacheStub{
			authCacheStub: &authCacheStub{},
			shared:        shared,
			publishErr:    errors.New("simulated missed pubsub delivery"),
		}
		repo := &authRepoStub{getByKeyForAuth: func(context.Context, string) (*APIKey, error) {
			repoCalls.Add(1)
			status := StatusActive
			if exhausted.Load() {
				status = StatusAPIKeyQuotaExhausted
			}
			return &APIKey{
				ID: 1, UserID: 7, Status: status,
				User: &User{ID: 7, Status: StatusActive, Role: RoleUser},
			}, nil
		}}
		return NewAPIKeyService(repo, nil, nil, nil, nil, cache, &config.Config{
			APIKeyAuth: config.APIKeyAuthCacheConfig{L2TTLSeconds: 300},
		})
	}

	instanceA := newService()
	instanceB := newService()
	instanceA.authCacheEpoch.Store(41)
	instanceB.authCacheEpoch.Store(99)
	active, err := instanceB.GetByKey(ctx, "protected-key")
	require.NoError(t, err)
	require.Equal(t, StatusActive, active.Status)
	require.Equal(t, int32(1), repoCalls.Load())

	exhausted.Store(true)
	instanceA.InvalidateAuthCacheByUserID(ctx, 7)

	reloaded, err := instanceB.GetByKey(ctx, "protected-key")
	require.NoError(t, err)
	require.Equal(t, StatusAPIKeyQuotaExhausted, reloaded.Status)
	require.Equal(t, int32(2), repoCalls.Load(), "peer must reload despite missing Pub/Sub")

	restartedB := newService()
	restartedB.authCacheEpoch.Store(0)
	afterRestart, err := restartedB.GetByKey(ctx, "protected-key")
	require.NoError(t, err)
	require.Equal(t, StatusAPIKeyQuotaExhausted, afterRestart.Status)
	require.Equal(t, int32(2), repoCalls.Load(), "restart may reuse only the current durable-generation entry")
}

func TestAPIKeyService_ProtectedUpdateCannotCachePreMutationSnapshotInNewGeneration(t *testing.T) {
	ctx := context.Background()
	shared := &sharedAuthCacheGenerationState{entries: make(map[string]*APIKeyAuthCacheEntry)}
	updateEntered := make(chan struct{})
	allowUpdate := make(chan struct{})
	var disabled atomic.Bool
	var peerRepoCalls atomic.Int32

	mutationRepo := &authRepoStub{
		getByID: func(context.Context, int64) (*APIKey, error) {
			return &APIKey{ID: 1, UserID: 7, Key: "__hmac__protected", Status: StatusAPIKeyActive}, nil
		},
		update: func(_ context.Context, key *APIKey) error {
			close(updateEntered)
			<-allowUpdate
			disabled.Store(key.Status == StatusAPIKeyDisabled)
			return nil
		},
	}
	peerRepo := &authRepoStub{getByKeyForAuth: func(context.Context, string) (*APIKey, error) {
		peerRepoCalls.Add(1)
		status := StatusAPIKeyActive
		if disabled.Load() {
			status = StatusAPIKeyDisabled
		}
		return &APIKey{
			ID: 1, UserID: 7, Status: status,
			User: &User{ID: 7, Status: StatusActive, Role: RoleUser},
		}, nil
	}}
	newCache := func() *distributedAuthCacheStub {
		return &distributedAuthCacheStub{authCacheStub: &authCacheStub{}, shared: shared}
	}
	mutator := NewAPIKeyService(mutationRepo, nil, nil, nil, nil, newCache(), &config.Config{})
	peer := NewAPIKeyService(peerRepo, nil, nil, nil, nil, newCache(), &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{L2TTLSeconds: 300},
	})

	status := StatusAPIKeyDisabled
	updateDone := make(chan error, 1)
	go func() {
		_, err := mutator.Update(ctx, 1, 7, UpdateAPIKeyRequest{Status: &status})
		updateDone <- err
	}()
	<-updateEntered

	beforeCommit, err := peer.GetByKey(ctx, "protected-key")
	require.NoError(t, err)
	require.Equal(t, StatusAPIKeyActive, beforeCommit.Status)
	require.Equal(t, int32(1), peerRepoCalls.Load())

	close(allowUpdate)
	require.NoError(t, <-updateDone)
	afterCommit, err := peer.GetByKey(ctx, "protected-key")
	require.NoError(t, err)
	require.Equal(t, StatusAPIKeyDisabled, afterCommit.Status)
	require.Equal(t, int32(2), peerRepoCalls.Load(), "post-mutation generation must make the raced snapshot unreachable")
}

func TestAPIKeyService_ProtectedDeleteCannotCachePreMutationSnapshotInNewGeneration(t *testing.T) {
	ctx := context.Background()
	shared := &sharedAuthCacheGenerationState{entries: make(map[string]*APIKeyAuthCacheEntry)}
	deleteEntered := make(chan struct{})
	allowDelete := make(chan struct{})
	var deleted atomic.Bool
	var peerRepoCalls atomic.Int32

	mutationRepo := &authRepoStub{
		getKeyAndOwner: func(context.Context, int64) (string, int64, error) {
			return "__hmac__protected", 7, nil
		},
		deleteWithAudit: func(context.Context, int64) error {
			close(deleteEntered)
			<-allowDelete
			deleted.Store(true)
			return nil
		},
	}
	peerRepo := &authRepoStub{getByKeyForAuth: func(context.Context, string) (*APIKey, error) {
		peerRepoCalls.Add(1)
		if deleted.Load() {
			return nil, ErrAPIKeyNotFound
		}
		return &APIKey{
			ID: 1, UserID: 7, Status: StatusAPIKeyActive,
			User: &User{ID: 7, Status: StatusActive, Role: RoleUser},
		}, nil
	}}
	newCache := func() *distributedAuthCacheStub {
		return &distributedAuthCacheStub{authCacheStub: &authCacheStub{}, shared: shared}
	}
	mutator := NewAPIKeyService(mutationRepo, nil, nil, nil, nil, newCache(), &config.Config{})
	peer := NewAPIKeyService(peerRepo, nil, nil, nil, nil, newCache(), &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{L2TTLSeconds: 300},
	})

	deleteDone := make(chan error, 1)
	go func() { deleteDone <- mutator.Delete(ctx, 1, 7) }()
	<-deleteEntered

	beforeCommit, err := peer.GetByKey(ctx, "protected-key")
	require.NoError(t, err)
	require.Equal(t, StatusAPIKeyActive, beforeCommit.Status)
	require.Equal(t, int32(1), peerRepoCalls.Load())

	close(allowDelete)
	require.NoError(t, <-deleteDone)
	_, err = peer.GetByKey(ctx, "protected-key")
	require.ErrorIs(t, err, ErrAPIKeyNotFound)
	require.Equal(t, int32(2), peerRepoCalls.Load(), "post-delete generation must make the raced snapshot unreachable")
}

func TestAPIKeyService_GenerationReadFailureBypassesAuthenticationCaches(t *testing.T) {
	cache := &unavailableAuthGenerationCache{authCacheStub: &authCacheStub{
		getAuthCache: func(context.Context, string) (*APIKeyAuthCacheEntry, error) {
			t.Fatal("generation failure must bypass L1/L2 cache reads")
			return nil, nil
		},
	}}
	var repoCalls atomic.Int32
	repo := &authRepoStub{getByKeyForAuth: func(context.Context, string) (*APIKey, error) {
		repoCalls.Add(1)
		return &APIKey{
			ID: 1, UserID: 7, Status: StatusAPIKeyQuotaExhausted,
			User: &User{ID: 7, Status: StatusActive, Role: RoleUser},
		}, nil
	}}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{L1Size: 100, L1TTLSeconds: 300, L2TTLSeconds: 300},
	})

	key, err := svc.GetByKey(context.Background(), "protected-key")
	require.NoError(t, err)
	require.Equal(t, StatusAPIKeyQuotaExhausted, key.Status)
	require.Equal(t, int32(1), repoCalls.Load())
}

func TestAPIKeyService_FiniteQuotaCacheCannotReauthorizeAfterGenerationIncrementFailure(t *testing.T) {
	stale := &APIKeyAuthCacheEntry{Snapshot: &APIKeyAuthSnapshot{
		Version:   apiKeyAuthSnapshotVersion,
		APIKeyID:  1,
		UserID:    7,
		Status:    StatusActive,
		Quota:     10,
		QuotaUsed: 9,
		User:      APIKeyAuthUserSnapshot{ID: 7, Status: StatusActive, Role: RoleUser},
	}}
	cache := &failedIncrementAuthGenerationCache{
		authCacheStub: &authCacheStub{getAuthCache: func(context.Context, string) (*APIKeyAuthCacheEntry, error) {
			return stale, nil
		}},
		generation: 7,
	}
	var repoCalls atomic.Int32
	repo := &authRepoStub{getByKeyForAuth: func(context.Context, string) (*APIKey, error) {
		repoCalls.Add(1)
		return &APIKey{
			ID: 1, UserID: 7, Status: StatusAPIKeyQuotaExhausted, Quota: 10, QuotaUsed: 10,
			User: &User{ID: 7, Status: StatusActive, Role: RoleUser},
		}, nil
	}}
	mutationInstance := NewAPIKeyService(repo, nil, nil, nil, nil, cache, &config.Config{})
	peerInstance := NewAPIKeyService(repo, nil, nil, nil, nil, cache, &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{L1Size: 100, L1TTLSeconds: 300, L2TTLSeconds: 300},
	})

	require.Error(t, mutationInstance.invalidateAllAuthCache(context.Background()))
	key, err := peerInstance.GetByKey(context.Background(), "protected-key")
	require.NoError(t, err)
	require.Equal(t, StatusAPIKeyQuotaExhausted, key.Status)
	require.Equal(t, int32(1), repoCalls.Load(), "finite-quota credentials must bypass stale authentication snapshots")
}

func TestAPIKeyService_InvalidateAuthCacheByGroupID(t *testing.T) {
	cache := &authCacheStub{}
	svc := NewAPIKeyService(&authRepoStub{}, nil, nil, nil, nil, cache, &config.Config{})
	before := svc.authCacheKey("test-key")

	svc.InvalidateAuthCacheByGroupID(context.Background(), 9)
	require.NotEqual(t, before, svc.authCacheKey("test-key"))
	require.Equal(t, []string{authCacheInvalidateAll}, cache.publishedKeys)
	require.Empty(t, cache.deleteAuthKeys)
}

func TestAPIKeyService_InvalidateAuthCacheByKey(t *testing.T) {
	cache := &authCacheStub{}
	repo := &authRepoStub{
		listKeysByUserID: func(ctx context.Context, userID int64) ([]string, error) {
			return nil, nil
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L2TTLSeconds: 60,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)

	svc.InvalidateAuthCacheByKey(context.Background(), "k1")
	require.Len(t, cache.deleteAuthKeys, 1)
}

func TestAPIKeyService_GetByKey_CachesNegativeOnRepoMiss(t *testing.T) {
	var repoCalls atomic.Int32
	cache := &authCacheStub{}
	repo := &authRepoStub{
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			repoCalls.Add(1)
			return nil, ErrAPIKeyNotFound
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			L1Size:             100,
			L1TTLSeconds:       60,
			L2TTLSeconds:       60,
			NegativeTTLSeconds: 30,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)
	cache.getAuthCache = func(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error) {
		return nil, redis.Nil
	}

	_, err := svc.GetByKey(context.Background(), "missing")
	require.ErrorIs(t, err, ErrAPIKeyNotFound)
	require.Empty(t, cache.setAuthKeys, "attacker-controlled misses must not be written to Redis")
	svc.authNegativeCacheL1.Wait()
	_, err = svc.GetByKey(context.Background(), "missing")
	require.ErrorIs(t, err, ErrAPIKeyNotFound)
	require.Equal(t, int32(1), repoCalls.Load())
}

func TestAPIKeyService_GetByKeyRejectsInvalidLengthBeforeCaches(t *testing.T) {
	var cacheCalls atomic.Int32
	cache := &authCacheStub{getAuthCache: func(context.Context, string) (*APIKeyAuthCacheEntry, error) {
		cacheCalls.Add(1)
		return nil, redis.Nil
	}}
	repo := &authRepoStub{getByKeyForAuth: func(context.Context, string) (*APIKey, error) {
		t.Fatal("invalid credential reached repository")
		return nil, nil
	}}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, &config.Config{APIKeyAuth: config.APIKeyAuthCacheConfig{L2TTLSeconds: 60}})

	for _, key := range []string{"", strings.Repeat("x", MaxAPIKeyCredentialBytes+1)} {
		_, err := svc.GetByKey(context.Background(), key)
		require.ErrorIs(t, err, ErrAPIKeyNotFound)
	}
	require.Zero(t, cacheCalls.Load())
}

func TestAPIKeyService_GetByKeyAllowsMaximumLength(t *testing.T) {
	key := strings.Repeat("x", MaxAPIKeyCredentialBytes)
	var repoCalls atomic.Int32
	repo := &authRepoStub{getByKeyForAuth: func(_ context.Context, got string) (*APIKey, error) {
		repoCalls.Add(1)
		require.Equal(t, key, got)
		return nil, ErrAPIKeyNotFound
	}}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{})
	_, err := svc.GetByKey(context.Background(), key)
	require.ErrorIs(t, err, ErrAPIKeyNotFound)
	require.Equal(t, int32(1), repoCalls.Load())
}

func TestAPIKeyService_AuthLookupBulkheadRejectsExcessMisses(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	repo := &authRepoStub{getByKeyForAuth: func(context.Context, string) (*APIKey, error) {
		close(entered)
		<-release
		return nil, ErrAPIKeyNotFound
	}}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{APIKeyAuth: config.APIKeyAuthCacheConfig{LookupConcurrency: 1}})

	done := make(chan error, 1)
	go func() {
		_, err := svc.GetByKey(context.Background(), "first")
		done <- err
	}()
	<-entered

	_, err := svc.GetByKey(context.Background(), "second")
	require.ErrorIs(t, err, ErrAPIKeyAuthOverloaded)
	metrics := svc.AuthLookupMetrics()
	require.Equal(t, uint64(2), metrics.Total)
	require.Equal(t, uint64(1), metrics.Rejected)
	require.Equal(t, int64(1), metrics.InFlight)
	require.Equal(t, 1, metrics.Capacity)

	close(release)
	require.ErrorIs(t, <-done, ErrAPIKeyNotFound)
}

func TestAPIKeyService_GetByKey_SingleflightCollapses(t *testing.T) {
	var calls int32
	cache := &authCacheStub{}
	repo := &authRepoStub{
		getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
			atomic.AddInt32(&calls, 1)
			time.Sleep(50 * time.Millisecond)
			return &APIKey{
				ID:     11,
				UserID: 2,
				Status: StatusActive,
				User: &User{
					ID:          2,
					Status:      StatusActive,
					Role:        RoleUser,
					Balance:     1,
					Concurrency: 1,
				},
			}, nil
		},
	}
	cfg := &config.Config{
		APIKeyAuth: config.APIKeyAuthCacheConfig{
			Singleflight: true,
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, cfg)

	start := make(chan struct{})
	wg := sync.WaitGroup{}
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			_, err := svc.GetByKey(context.Background(), "k1")
			errs[idx] = err
		}(i)
	}
	close(start)
	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}
