package service

import (
	"context"
)

// InvalidateAuthCacheByKey 清除指定 API Key 的认证缓存
func (s *APIKeyService) InvalidateAuthCacheByKey(ctx context.Context, key string) {
	if key == "" {
		return
	}
	cacheKey, err := s.authCacheKeyForRequest(ctx, key)
	if err != nil {
		s.invalidateAllAuthCache(ctx)
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
	s.invalidateAllAuthCache(ctx)
}

// InvalidateAuthCacheByGroupID uses the same generation boundary. Group/user
// changes are rare; bounded stale Redis entries expire naturally and cannot be
// read after the generation changes.
func (s *APIKeyService) InvalidateAuthCacheByGroupID(ctx context.Context, groupID int64) {
	if groupID <= 0 {
		return
	}
	s.invalidateAllAuthCache(ctx)
}

func (s *APIKeyService) invalidateAllAuthCache(ctx context.Context) {
	if generations, ok := s.cache.(APIKeyAuthCacheGenerationStore); ok {
		generation, err := generations.IncrementAuthCacheGeneration(ctx)
		if err != nil {
			// Locally stop reusing old entries. Other instances consult the durable
			// generation on every request and bypass caches if it is unavailable.
			s.authCacheEpoch.Add(1)
		} else {
			s.authCacheEpoch.Store(generation)
		}
	} else {
		s.authCacheEpoch.Add(1)
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
}
