package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	refreshTokenKeyPrefix         = "refresh_token:"
	consumedRefreshTokenKeyPrefix = "refresh_token_consumed:"
	userRefreshTokensPrefix       = "user_refresh_tokens:"
	tokenFamilyPrefix             = "token_family:"
	revokedTokenFamilyKeyPrefix   = "token_family_revoked:"
)

var issueRefreshTokenScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[4]) == 1 then return -1 end
if redis.call('EXISTS', KEYS[1]) == 1 or redis.call('EXISTS', KEYS[5]) == 1 then return -2 end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
redis.call('SADD', KEYS[2], ARGV[3])
redis.call('PEXPIRE', KEYS[2], ARGV[2])
redis.call('SADD', KEYS[3], ARGV[3])
redis.call('PEXPIRE', KEYS[3], ARGV[2])
return 1
`)

var rotateRefreshTokenScript = redis.NewScript(`
local function revoke_family()
  local members = redis.call('SMEMBERS', KEYS[4])
  for _, hash in ipairs(members) do
    redis.call('DEL', ARGV[6] .. hash)
    redis.call('SREM', KEYS[3], hash)
  end
  redis.call('DEL', KEYS[4])
  redis.call('SET', KEYS[7], '1', 'PX', ARGV[5])
end

if redis.call('EXISTS', KEYS[7]) == 1 then return -1 end
local current = redis.call('GET', KEYS[1])
if not current then
  if redis.call('EXISTS', KEYS[5]) == 1 then
    revoke_family()
    return -2
  end
  return -3
end
if current ~= ARGV[1] then return -4 end
if redis.call('EXISTS', KEYS[2]) == 1 or redis.call('EXISTS', KEYS[6]) == 1 then return -4 end

redis.call('DEL', KEYS[1])
redis.call('SREM', KEYS[3], ARGV[3])
redis.call('SREM', KEYS[4], ARGV[3])
redis.call('SET', KEYS[5], ARGV[1], 'PX', ARGV[5])
redis.call('SET', KEYS[2], ARGV[2], 'PX', ARGV[5])
redis.call('SADD', KEYS[3], ARGV[4])
redis.call('PEXPIRE', KEYS[3], ARGV[5])
redis.call('SADD', KEYS[4], ARGV[4])
redis.call('PEXPIRE', KEYS[4], ARGV[5])
return 1
`)

var revokeTokenFamilyScript = redis.NewScript(`
local members = redis.call('SMEMBERS', KEYS[1])
for _, hash in ipairs(members) do
  local value = redis.call('GET', ARGV[2] .. hash)
  if value then
    local ok, data = pcall(cjson.decode, value)
    if ok and data['user_id'] then
      redis.call('SREM', ARGV[3] .. tostring(data['user_id']), hash)
    end
  end
  redis.call('DEL', ARGV[2] .. hash)
end
redis.call('DEL', KEYS[1])
redis.call('SET', KEYS[2], '1', 'PX', ARGV[1])
return #members
`)

var revokeUserTokenFamiliesScript = redis.NewScript(`
local members = redis.call('SMEMBERS', KEYS[1])
local families = {}
for _, hash in ipairs(members) do
  local value = redis.call('GET', ARGV[2] .. hash)
  if value then
    local ok, data = pcall(cjson.decode, value)
    if ok and data['family_id'] then families[data['family_id']] = true end
  end
end
for family, _ in pairs(families) do
  local family_key = ARGV[3] .. family
  local family_members = redis.call('SMEMBERS', family_key)
  for _, hash in ipairs(family_members) do
    redis.call('DEL', ARGV[2] .. hash)
    redis.call('SREM', KEYS[1], hash)
  end
  redis.call('DEL', family_key)
  redis.call('SET', ARGV[4] .. family, '1', 'PX', ARGV[1])
end
redis.call('DEL', KEYS[1])
return #members
`)

// refreshTokenKey generates the Redis key for a refresh token.
func refreshTokenKey(tokenHash string) string {
	return refreshTokenKeyPrefix + tokenHash
}

// userRefreshTokensKey generates the Redis key for user's token set.
func userRefreshTokensKey(userID int64) string {
	return fmt.Sprintf("%s%d", userRefreshTokensPrefix, userID)
}

// tokenFamilyKey generates the Redis key for token family set.
func tokenFamilyKey(familyID string) string {
	return tokenFamilyPrefix + familyID
}

func consumedRefreshTokenKey(tokenHash string) string {
	return consumedRefreshTokenKeyPrefix + tokenHash
}

func revokedTokenFamilyKey(familyID string) string {
	return revokedTokenFamilyKeyPrefix + familyID
}

type refreshTokenCache struct {
	rdb *redis.Client
}

// NewRefreshTokenCache creates a new RefreshTokenCache implementation.
func NewRefreshTokenCache(rdb *redis.Client) service.RefreshTokenCache {
	return &refreshTokenCache{rdb: rdb}
}

func (c *refreshTokenCache) StoreRefreshToken(ctx context.Context, tokenHash string, data *service.RefreshTokenData, ttl time.Duration) error {
	key := refreshTokenKey(tokenHash)
	val, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal refresh token data: %w", err)
	}
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

func (c *refreshTokenCache) GetRefreshToken(ctx context.Context, tokenHash string) (*service.RefreshTokenData, error) {
	values, err := c.rdb.MGet(ctx, refreshTokenKey(tokenHash), consumedRefreshTokenKey(tokenHash)).Result()
	if err != nil {
		return nil, err
	}
	var val string
	var stateErr error
	if values[0] != nil {
		val, _ = values[0].(string)
	} else if values[1] != nil {
		val, _ = values[1].(string)
		stateErr = service.ErrRefreshTokenConsumed
	} else {
		return nil, service.ErrRefreshTokenNotFound
	}
	var data service.RefreshTokenData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("unmarshal refresh token data: %w", err)
	}
	return &data, stateErr
}

func (c *refreshTokenCache) IssueRefreshToken(ctx context.Context, tokenHash string, data *service.RefreshTokenData, ttl time.Duration) error {
	if data == nil || ttl < time.Millisecond {
		return fmt.Errorf("invalid refresh token issue state")
	}
	val, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal refresh token data: %w", err)
	}
	result, err := issueRefreshTokenScript.Run(ctx, c.rdb, []string{
		refreshTokenKey(tokenHash),
		userRefreshTokensKey(data.UserID),
		tokenFamilyKey(data.FamilyID),
		revokedTokenFamilyKey(data.FamilyID),
		consumedRefreshTokenKey(tokenHash),
	}, val, ttl.Milliseconds(), tokenHash).Int()
	if err != nil {
		return err
	}
	switch result {
	case -1:
		return service.ErrRefreshTokenFamilyRevoked
	case -2:
		return fmt.Errorf("refresh token hash collision")
	default:
		return nil
	}
}

func (c *refreshTokenCache) RotateRefreshToken(ctx context.Context, parentHash string, parent *service.RefreshTokenData, childHash string, child *service.RefreshTokenData, ttl time.Duration) error {
	if parent == nil || child == nil || ttl < time.Millisecond {
		return fmt.Errorf("invalid refresh token rotation state")
	}
	parentVal, err := json.Marshal(parent)
	if err != nil {
		return fmt.Errorf("marshal parent refresh token data: %w", err)
	}
	childVal, err := json.Marshal(child)
	if err != nil {
		return fmt.Errorf("marshal child refresh token data: %w", err)
	}
	result, err := rotateRefreshTokenScript.Run(ctx, c.rdb, []string{
		refreshTokenKey(parentHash),
		refreshTokenKey(childHash),
		userRefreshTokensKey(parent.UserID),
		tokenFamilyKey(parent.FamilyID),
		consumedRefreshTokenKey(parentHash),
		consumedRefreshTokenKey(childHash),
		revokedTokenFamilyKey(parent.FamilyID),
	}, parentVal, childVal, parentHash, childHash, ttl.Milliseconds(), refreshTokenKeyPrefix).Int()
	if err != nil {
		return err
	}
	switch result {
	case -1:
		return service.ErrRefreshTokenFamilyRevoked
	case -2:
		return service.ErrRefreshTokenConsumed
	case -3:
		return service.ErrRefreshTokenNotFound
	case -4:
		return fmt.Errorf("refresh token state conflict")
	default:
		return nil
	}
}

func (c *refreshTokenCache) RevokeTokenFamily(ctx context.Context, familyID string, ttl time.Duration) error {
	if familyID == "" || ttl < time.Millisecond {
		return fmt.Errorf("invalid refresh token family revocation state")
	}
	return revokeTokenFamilyScript.Run(ctx, c.rdb, []string{
		tokenFamilyKey(familyID),
		revokedTokenFamilyKey(familyID),
	}, ttl.Milliseconds(), refreshTokenKeyPrefix, userRefreshTokensPrefix).Err()
}

func (c *refreshTokenCache) RevokeUserTokenFamilies(ctx context.Context, userID int64, ttl time.Duration) error {
	if userID <= 0 || ttl < time.Millisecond {
		return fmt.Errorf("invalid user refresh token revocation state")
	}
	return revokeUserTokenFamiliesScript.Run(ctx, c.rdb, []string{userRefreshTokensKey(userID)},
		ttl.Milliseconds(), refreshTokenKeyPrefix, tokenFamilyPrefix, revokedTokenFamilyKeyPrefix).Err()
}

func (c *refreshTokenCache) IsTokenFamilyRevoked(ctx context.Context, familyID string) (bool, error) {
	count, err := c.rdb.Exists(ctx, revokedTokenFamilyKey(familyID)).Result()
	return count != 0, err
}

func (c *refreshTokenCache) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	key := refreshTokenKey(tokenHash)
	return c.rdb.Del(ctx, key).Err()
}

func (c *refreshTokenCache) DeleteUserRefreshTokens(ctx context.Context, userID int64) error {
	// Get all token hashes for this user
	tokenHashes, err := c.GetUserTokenHashes(ctx, userID)
	if err != nil && err != redis.Nil {
		return fmt.Errorf("get user token hashes: %w", err)
	}

	if len(tokenHashes) == 0 {
		return nil
	}

	// Build keys to delete
	keys := make([]string, 0, len(tokenHashes)+1)
	for _, hash := range tokenHashes {
		keys = append(keys, refreshTokenKey(hash))
	}
	keys = append(keys, userRefreshTokensKey(userID))

	// Delete all keys in a pipeline
	pipe := c.rdb.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (c *refreshTokenCache) DeleteTokenFamily(ctx context.Context, familyID string) error {
	// Get all token hashes in this family
	tokenHashes, err := c.GetFamilyTokenHashes(ctx, familyID)
	if err != nil && err != redis.Nil {
		return fmt.Errorf("get family token hashes: %w", err)
	}

	if len(tokenHashes) == 0 {
		return nil
	}

	// Build keys to delete
	keys := make([]string, 0, len(tokenHashes)+1)
	for _, hash := range tokenHashes {
		keys = append(keys, refreshTokenKey(hash))
	}
	keys = append(keys, tokenFamilyKey(familyID))

	// Delete all keys in a pipeline
	pipe := c.rdb.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (c *refreshTokenCache) AddToUserTokenSet(ctx context.Context, userID int64, tokenHash string, ttl time.Duration) error {
	key := userRefreshTokensKey(userID)
	pipe := c.rdb.Pipeline()
	pipe.SAdd(ctx, key, tokenHash)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *refreshTokenCache) AddToFamilyTokenSet(ctx context.Context, familyID string, tokenHash string, ttl time.Duration) error {
	key := tokenFamilyKey(familyID)
	pipe := c.rdb.Pipeline()
	pipe.SAdd(ctx, key, tokenHash)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *refreshTokenCache) GetUserTokenHashes(ctx context.Context, userID int64) ([]string, error) {
	key := userRefreshTokensKey(userID)
	return c.rdb.SMembers(ctx, key).Result()
}

func (c *refreshTokenCache) GetFamilyTokenHashes(ctx context.Context, familyID string) ([]string, error) {
	key := tokenFamilyKey(familyID)
	return c.rdb.SMembers(ctx, key).Result()
}

func (c *refreshTokenCache) IsTokenInFamily(ctx context.Context, familyID string, tokenHash string) (bool, error) {
	key := tokenFamilyKey(familyID)
	return c.rdb.SIsMember(ctx, key, tokenHash).Result()
}
