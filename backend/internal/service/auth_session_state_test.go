package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type atomicRefreshTokenCacheTestDouble struct {
	getData       *RefreshTokenData
	getErr        error
	revoked       bool
	revocationErr error
	issued        bool
}

func (c *atomicRefreshTokenCacheTestDouble) StoreRefreshToken(context.Context, string, *RefreshTokenData, time.Duration) error {
	return nil
}
func (c *atomicRefreshTokenCacheTestDouble) GetRefreshToken(context.Context, string) (*RefreshTokenData, error) {
	return c.getData, c.getErr
}
func (c *atomicRefreshTokenCacheTestDouble) DeleteRefreshToken(context.Context, string) error {
	return nil
}
func (c *atomicRefreshTokenCacheTestDouble) DeleteUserRefreshTokens(context.Context, int64) error {
	return nil
}
func (c *atomicRefreshTokenCacheTestDouble) DeleteTokenFamily(context.Context, string) error {
	return nil
}
func (c *atomicRefreshTokenCacheTestDouble) AddToUserTokenSet(context.Context, int64, string, time.Duration) error {
	return nil
}
func (c *atomicRefreshTokenCacheTestDouble) AddToFamilyTokenSet(context.Context, string, string, time.Duration) error {
	return nil
}
func (c *atomicRefreshTokenCacheTestDouble) GetUserTokenHashes(context.Context, int64) ([]string, error) {
	return nil, nil
}
func (c *atomicRefreshTokenCacheTestDouble) GetFamilyTokenHashes(context.Context, string) ([]string, error) {
	return nil, nil
}
func (c *atomicRefreshTokenCacheTestDouble) IsTokenInFamily(context.Context, string, string) (bool, error) {
	return false, nil
}
func (c *atomicRefreshTokenCacheTestDouble) IssueRefreshToken(context.Context, string, *RefreshTokenData, time.Duration) error {
	c.issued = true
	return nil
}
func (c *atomicRefreshTokenCacheTestDouble) RotateRefreshToken(context.Context, string, *RefreshTokenData, string, *RefreshTokenData, time.Duration) error {
	return nil
}
func (c *atomicRefreshTokenCacheTestDouble) RevokeTokenFamily(context.Context, string, time.Duration) error {
	return nil
}
func (c *atomicRefreshTokenCacheTestDouble) RevokeUserTokenFamilies(context.Context, int64, time.Duration) error {
	return nil
}
func (c *atomicRefreshTokenCacheTestDouble) IsTokenFamilyRevoked(context.Context, string) (bool, error) {
	return c.revoked, c.revocationErr
}

func newSessionStateTestAuth(cache RefreshTokenCache) *AuthService {
	return NewAuthService(nil, nil, nil, cache, &config.Config{JWT: config.JWTConfig{
		Secret:                   "session-state-test-secret",
		AccessTokenExpireMinutes: 15,
		RefreshTokenExpireDays:   30,
	}}, nil, nil, nil, nil, nil, nil, nil, nil)
}

func TestValidateAccessTokenRejectsRevokedFamily(t *testing.T) {
	cache := &atomicRefreshTokenCacheTestDouble{revoked: true}
	auth := newSessionStateTestAuth(cache)
	token, err := auth.generateAccessToken(&User{ID: 42, Email: "user@example.com", Role: RoleUser}, "family-1", "")
	require.NoError(t, err)

	_, err = auth.ValidateAccessToken(context.Background(), token)
	require.ErrorIs(t, err, ErrTokenRevoked)
}

func TestValidateAccessTokenFailsClosedWhenRedisUnavailable(t *testing.T) {
	cache := &atomicRefreshTokenCacheTestDouble{revocationErr: errors.New("redis unavailable")}
	auth := newSessionStateTestAuth(cache)
	token, err := auth.generateAccessToken(&User{ID: 42, Email: "user@example.com", Role: RoleUser}, "family-1", "")
	require.NoError(t, err)

	_, err = auth.ValidateAccessToken(context.Background(), token)
	require.ErrorIs(t, err, ErrServiceUnavailable)
}

func TestRefreshTokenPairReturnsServiceUnavailableOnRedisReadFailure(t *testing.T) {
	cache := &atomicRefreshTokenCacheTestDouble{getErr: errors.New("redis unavailable")}
	auth := newSessionStateTestAuth(cache)

	_, err := auth.RefreshTokenPair(context.Background(), "rt_known-format")
	require.ErrorIs(t, err, ErrServiceUnavailable)
}

func TestGenerateTokenPairUsesAtomicIssue(t *testing.T) {
	cache := &atomicRefreshTokenCacheTestDouble{}
	auth := newSessionStateTestAuth(cache)
	_, err := auth.GenerateTokenPair(context.Background(), &User{ID: 42, Email: "user@example.com", Role: RoleUser}, "")
	require.NoError(t, err)
	require.True(t, cache.issued)
}
