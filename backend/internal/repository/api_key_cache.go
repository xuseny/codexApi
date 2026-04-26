package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	apiKeyRateLimitKeyPrefix   = "apikey:ratelimit:"
	apiKeyRateLimitDuration    = 24 * time.Hour
	apiKeyAuthCachePrefix      = "apikey:auth:"
	apiKeyDeviceLockKeyPrefix  = "apikey:device_lock:"
	apiKeyConcurrencyKeyPrefix = "apikey:concurrency:"
	authCacheInvalidateChannel = "auth:cache:invalidate"
)

// apiKeyRateLimitKey generates the Redis key for API key creation rate limiting.
func apiKeyRateLimitKey(userID int64) string {
	return fmt.Sprintf("%s%d", apiKeyRateLimitKeyPrefix, userID)
}

func apiKeyAuthCacheKey(key string) string {
	return fmt.Sprintf("%s%s", apiKeyAuthCachePrefix, key)
}

func apiKeyDeviceLockKey(keyID int64) string {
	return fmt.Sprintf("%s%d", apiKeyDeviceLockKeyPrefix, keyID)
}

func apiKeyConcurrencyKey(keyID int64) string {
	return fmt.Sprintf("%s%d", apiKeyConcurrencyKeyPrefix, keyID)
}

var acquireAPIKeyDeviceLockScript = redis.NewScript(`
	local key = KEYS[1]
	local fingerprint = ARGV[1]
	local payload = ARGV[2]
	local ttl = tonumber(ARGV[3])

	local existing = redis.call('GET', key)
	if not existing or existing == false then
		redis.call('PSETEX', key, ttl, payload)
		return cjson.encode({ acquired = true })
	end

	local ok, decoded = pcall(cjson.decode, existing)
	if ok and decoded and decoded.fingerprint == fingerprint then
		redis.call('PSETEX', key, ttl, payload)
		return cjson.encode({ acquired = true, current = decoded })
	end

	if ok and decoded then
		return cjson.encode({ acquired = false, current = decoded })
	end

	return cjson.encode({ acquired = false })
`)

type apiKeyCache struct {
	rdb *redis.Client
}

func NewAPIKeyCache(rdb *redis.Client) service.APIKeyCache {
	return &apiKeyCache{rdb: rdb}
}

func (c *apiKeyCache) GetCreateAttemptCount(ctx context.Context, userID int64) (int, error) {
	key := apiKeyRateLimitKey(userID)
	count, err := c.rdb.Get(ctx, key).Int()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return count, err
}

func (c *apiKeyCache) IncrementCreateAttemptCount(ctx context.Context, userID int64) error {
	key := apiKeyRateLimitKey(userID)
	pipe := c.rdb.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, apiKeyRateLimitDuration)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *apiKeyCache) DeleteCreateAttemptCount(ctx context.Context, userID int64) error {
	key := apiKeyRateLimitKey(userID)
	return c.rdb.Del(ctx, key).Err()
}

func (c *apiKeyCache) IncrementDailyUsage(ctx context.Context, apiKey string) error {
	return c.rdb.Incr(ctx, apiKey).Err()
}

func (c *apiKeyCache) SetDailyUsageExpiry(ctx context.Context, apiKey string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, apiKey, ttl).Err()
}

func (c *apiKeyCache) GetAuthCache(ctx context.Context, key string) (*service.APIKeyAuthCacheEntry, error) {
	val, err := c.rdb.Get(ctx, apiKeyAuthCacheKey(key)).Bytes()
	if err != nil {
		return nil, err
	}
	var entry service.APIKeyAuthCacheEntry
	if err := json.Unmarshal(val, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func (c *apiKeyCache) SetAuthCache(ctx context.Context, key string, entry *service.APIKeyAuthCacheEntry, ttl time.Duration) error {
	if entry == nil {
		return nil
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, apiKeyAuthCacheKey(key), payload, ttl).Err()
}

func (c *apiKeyCache) DeleteAuthCache(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, apiKeyAuthCacheKey(key)).Err()
}

func (c *apiKeyCache) AcquireAPIKeySlot(ctx context.Context, keyID int64, maxConcurrency int, requestID string, ttl time.Duration) (bool, error) {
	if keyID <= 0 || maxConcurrency <= 0 {
		return true, nil
	}
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	result, err := acquireScript.Run(ctx, c.rdb, []string{apiKeyConcurrencyKey(keyID)}, maxConcurrency, int(ttl.Seconds()), requestID).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (c *apiKeyCache) ReleaseAPIKeySlot(ctx context.Context, keyID int64, requestID string) error {
	if keyID <= 0 || requestID == "" {
		return nil
	}
	return c.rdb.ZRem(ctx, apiKeyConcurrencyKey(keyID), requestID).Err()
}

func (c *apiKeyCache) DeleteAPIKeySlots(ctx context.Context, keyID int64) error {
	if keyID <= 0 {
		return nil
	}
	return c.rdb.Del(ctx, apiKeyConcurrencyKey(keyID)).Err()
}

// PublishAuthCacheInvalidation publishes a cache invalidation message to all instances
func (c *apiKeyCache) PublishAuthCacheInvalidation(ctx context.Context, cacheKey string) error {
	return c.rdb.Publish(ctx, authCacheInvalidateChannel, cacheKey).Err()
}

// SubscribeAuthCacheInvalidation subscribes to cache invalidation messages
func (c *apiKeyCache) SubscribeAuthCacheInvalidation(ctx context.Context, handler func(cacheKey string)) error {
	pubsub := c.rdb.Subscribe(ctx, authCacheInvalidateChannel)

	// Verify subscription is working
	_, err := pubsub.Receive(ctx)
	if err != nil {
		_ = pubsub.Close()
		return fmt.Errorf("subscribe to auth cache invalidation: %w", err)
	}

	go func() {
		defer func() {
			if err := pubsub.Close(); err != nil {
				log.Printf("Warning: failed to close auth cache invalidation pubsub: %v", err)
			}
		}()

		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				if msg != nil {
					handler(msg.Payload)
				}
			}
		}
	}()

	return nil
}

type apiKeyDeviceLockAcquireResult struct {
	Acquired bool                      `json:"acquired"`
	Current  *service.APIKeyDeviceLock `json:"current,omitempty"`
}

func (c *apiKeyCache) AcquireDeviceLock(ctx context.Context, keyID int64, lock *service.APIKeyDeviceLock, ttl time.Duration) (*service.APIKeyDeviceLock, bool, error) {
	if lock == nil || keyID <= 0 {
		return nil, true, nil
	}

	payload, err := json.Marshal(lock)
	if err != nil {
		return nil, false, err
	}

	if ttl <= 0 {
		ttl = time.Minute
	}

	rawResult, err := acquireAPIKeyDeviceLockScript.Run(
		ctx,
		c.rdb,
		[]string{apiKeyDeviceLockKey(keyID)},
		lock.Fingerprint,
		string(payload),
		ttl.Milliseconds(),
	).Result()
	if err != nil {
		return nil, false, err
	}
	raw, ok := rawResult.(string)
	if !ok {
		return nil, false, fmt.Errorf("unexpected api key device lock result type %T", rawResult)
	}

	var result apiKeyDeviceLockAcquireResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, false, err
	}
	return result.Current, result.Acquired, nil
}

func (c *apiKeyCache) GetDeviceLock(ctx context.Context, keyID int64) (*service.APIKeyDeviceLock, error) {
	if keyID <= 0 {
		return nil, nil
	}

	val, err := c.rdb.Get(ctx, apiKeyDeviceLockKey(keyID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var lock service.APIKeyDeviceLock
	if err := json.Unmarshal(val, &lock); err != nil {
		return nil, err
	}
	return &lock, nil
}

func (c *apiKeyCache) DeleteDeviceLock(ctx context.Context, keyID int64) error {
	if keyID <= 0 {
		return nil
	}
	return c.rdb.Del(ctx, apiKeyDeviceLockKey(keyID)).Err()
}
