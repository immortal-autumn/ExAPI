package service

import (
	"context"
)

// InvalidateAuthCacheByKey 清除指定 API Key 的认证缓存
func (s *APIKeyService) InvalidateAuthCacheByKey(ctx context.Context, key string) {
	if key == "" {
		return
	}
	cacheKey := s.authCacheKey(key)
	s.deleteAuthCache(ctx, cacheKey)
}

// InvalidateAuthCacheByUserID advances the process-local generation. This
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
	s.authCacheEpoch.Add(1)
	if s.authCacheL1 != nil {
		s.authCacheL1.Clear()
	}
	if s.cache != nil {
		_ = s.cache.PublishAuthCacheInvalidation(ctx, authCacheInvalidateAll)
	}
}
