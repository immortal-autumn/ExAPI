package service

import (
	"context"
	"errors"
)

// InvalidateAuthCacheByKey 清除指定 API Key 的认证缓存
func (s *APIKeyService) InvalidateAuthCacheByKey(ctx context.Context, key string) {
	if key == "" {
		return
	}
	cacheKey, err := s.authCacheKeyForRequest(ctx, key)
	if err != nil {
		_ = s.invalidateAllAuthCache(ctx)
		return
	}
	s.deleteAuthCache(ctx, cacheKey)
}

// InvalidateAuthCacheByUserID advances the durable global generation. This
// invalidates every L1/L2 lookup without enumerating or recovering raw keys.
func (s *APIKeyService) InvalidateAuthCacheByUserID(ctx context.Context, userID int64) {
	if userID <= 0 {
		return
	}
	_ = s.invalidateAllAuthCache(ctx)
}

// InvalidateAuthCacheByGroupID uses the same generation boundary. Group/user
// changes are rare; bounded stale Redis entries expire naturally and cannot be
// read after the generation changes.
func (s *APIKeyService) InvalidateAuthCacheByGroupID(ctx context.Context, groupID int64) {
	if groupID <= 0 {
		return
	}
	_ = s.invalidateAllAuthCache(ctx)
}

func (s *APIKeyService) invalidateAllAuthCache(ctx context.Context) error {
	var durableErr error
	if generations, ok := s.cache.(APIKeyAuthCacheGenerationStore); ok {
		generation, err := generations.IncrementAuthCacheGeneration(ctx)
		if err != nil {
			// Locally stop reusing old entries, but report the missing distributed
			// boundary so security-sensitive callers can fail before committing.
			s.authCacheEpoch.Add(1)
			durableErr = err
		} else {
			s.authCacheEpoch.Store(generation)
		}
	} else {
		s.authCacheEpoch.Add(1)
		durableErr = errors.New("authentication cache does not support durable generations")
	}
	if s.authCacheL1 != nil {
		s.authCacheL1.Clear()
	}
	if s.authNegativeCacheL1 != nil {
		s.authNegativeCacheL1.Clear()
	}
	if s.cache != nil {
		_ = s.cache.PublishAuthCacheInvalidation(ctx, authCacheInvalidateAll)
	}
	return durableErr
}
