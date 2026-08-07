package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newRefreshTokenCacheTest(t *testing.T) (*refreshTokenCache, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return &refreshTokenCache{rdb: client}, server
}

func refreshTokenTestData(hash, family string, expiresAt time.Time) *service.RefreshTokenData {
	return &service.RefreshTokenData{
		UserID:       42,
		TokenVersion: 7,
		FamilyID:     family,
		BindingHash:  "binding",
		CreatedAt:    expiresAt.Add(-time.Hour),
		ExpiresAt:    expiresAt,
	}
}

func requireMiniRedisSetMember(t *testing.T, server *miniredis.Miniredis, key, member string, want bool) {
	t.Helper()
	got, err := server.SIsMember(key, member)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestRefreshTokenAtomicIssueAndRotation(t *testing.T) {
	cache, server := newRefreshTokenCacheTest(t)
	ctx := context.Background()
	ttl := time.Hour
	expiresAt := time.Now().Add(ttl).UTC().Truncate(time.Millisecond)
	parent := refreshTokenTestData("parent", "family-1", expiresAt)
	child := refreshTokenTestData("child", "family-1", expiresAt)

	require.NoError(t, cache.IssueRefreshToken(ctx, "parent", parent, ttl))
	require.True(t, server.Exists(refreshTokenKey("parent")))
	requireMiniRedisSetMember(t, server, userRefreshTokensKey(42), "parent", true)
	requireMiniRedisSetMember(t, server, tokenFamilyKey("family-1"), "parent", true)

	require.NoError(t, cache.RotateRefreshToken(ctx, "parent", parent, "child", child, ttl))
	require.False(t, server.Exists(refreshTokenKey("parent")))
	require.True(t, server.Exists(consumedRefreshTokenKey("parent")))
	require.True(t, server.Exists(refreshTokenKey("child")))
	requireMiniRedisSetMember(t, server, userRefreshTokensKey(42), "parent", false)
	requireMiniRedisSetMember(t, server, userRefreshTokensKey(42), "child", true)
	requireMiniRedisSetMember(t, server, tokenFamilyKey("family-1"), "parent", false)
	requireMiniRedisSetMember(t, server, tokenFamilyKey("family-1"), "child", true)

	consumed, err := cache.GetRefreshToken(ctx, "parent")
	require.ErrorIs(t, err, service.ErrRefreshTokenConsumed)
	require.Equal(t, "family-1", consumed.FamilyID)
}

func TestRefreshTokenReplayRevokesWholeFamilyAtomically(t *testing.T) {
	cache, server := newRefreshTokenCacheTest(t)
	ctx := context.Background()
	ttl := time.Hour
	expiresAt := time.Now().Add(ttl).UTC().Truncate(time.Millisecond)
	parent := refreshTokenTestData("parent", "family-1", expiresAt)
	child := refreshTokenTestData("child", "family-1", expiresAt)
	require.NoError(t, cache.IssueRefreshToken(ctx, "parent", parent, ttl))
	require.NoError(t, cache.RotateRefreshToken(ctx, "parent", parent, "child", child, ttl))

	err := cache.RotateRefreshToken(ctx, "parent", parent, "other", child, ttl)
	require.ErrorIs(t, err, service.ErrRefreshTokenConsumed)
	require.False(t, server.Exists(refreshTokenKey("child")))
	require.False(t, server.Exists(tokenFamilyKey("family-1")))
	revoked, err := cache.IsTokenFamilyRevoked(ctx, "family-1")
	require.NoError(t, err)
	require.True(t, revoked)

	err = cache.IssueRefreshToken(ctx, "new", child, ttl)
	require.ErrorIs(t, err, service.ErrRefreshTokenFamilyRevoked)
}

func TestRefreshTokenUnknownHashIsNotClassifiedAsReplay(t *testing.T) {
	cache, _ := newRefreshTokenCacheTest(t)
	data, err := cache.GetRefreshToken(context.Background(), "unknown")
	require.Nil(t, data)
	require.True(t, errors.Is(err, service.ErrRefreshTokenNotFound))
}

func TestRefreshTokenFamilyAndUserRevocationAreAtomic(t *testing.T) {
	cache, server := newRefreshTokenCacheTest(t)
	ctx := context.Background()
	ttl := time.Hour
	expiresAt := time.Now().Add(ttl).UTC().Truncate(time.Millisecond)
	first := refreshTokenTestData("first", "family-1", expiresAt)
	second := refreshTokenTestData("second", "family-2", expiresAt)
	require.NoError(t, cache.IssueRefreshToken(ctx, "first", first, ttl))
	require.NoError(t, cache.IssueRefreshToken(ctx, "second", second, ttl))

	require.NoError(t, cache.RevokeTokenFamily(ctx, "family-1", ttl))
	require.False(t, server.Exists(refreshTokenKey("first")))
	require.True(t, server.Exists(refreshTokenKey("second")))
	revoked, err := cache.IsTokenFamilyRevoked(ctx, "family-1")
	require.NoError(t, err)
	require.True(t, revoked)

	require.NoError(t, cache.RevokeUserTokenFamilies(ctx, 42, ttl))
	require.False(t, server.Exists(refreshTokenKey("second")))
	revoked, err = cache.IsTokenFamilyRevoked(ctx, "family-2")
	require.NoError(t, err)
	require.True(t, revoked)
}
