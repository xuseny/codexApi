package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type apiKeyConcurrencyTestCache struct {
	acquired bool
	err      error

	acquireCalls int
	acquireKeyID int64
	acquireMax   int
	acquireReqID string
	acquireTTL   time.Duration

	releaseCalls int
	releaseKeyID int64
	releaseReqID string
}

func (c *apiKeyConcurrencyTestCache) GetCreateAttemptCount(context.Context, int64) (int, error) {
	return 0, nil
}

func (c *apiKeyConcurrencyTestCache) IncrementCreateAttemptCount(context.Context, int64) error {
	return nil
}

func (c *apiKeyConcurrencyTestCache) DeleteCreateAttemptCount(context.Context, int64) error {
	return nil
}

func (c *apiKeyConcurrencyTestCache) IncrementDailyUsage(context.Context, string) error {
	return nil
}

func (c *apiKeyConcurrencyTestCache) SetDailyUsageExpiry(context.Context, string, time.Duration) error {
	return nil
}

func (c *apiKeyConcurrencyTestCache) GetAuthCache(context.Context, string) (*APIKeyAuthCacheEntry, error) {
	return nil, redis.Nil
}

func (c *apiKeyConcurrencyTestCache) SetAuthCache(context.Context, string, *APIKeyAuthCacheEntry, time.Duration) error {
	return nil
}

func (c *apiKeyConcurrencyTestCache) DeleteAuthCache(context.Context, string) error {
	return nil
}

func (c *apiKeyConcurrencyTestCache) PublishAuthCacheInvalidation(context.Context, string) error {
	return nil
}

func (c *apiKeyConcurrencyTestCache) SubscribeAuthCacheInvalidation(context.Context, func(string)) error {
	return nil
}

func (c *apiKeyConcurrencyTestCache) AcquireDeviceLock(context.Context, int64, *APIKeyDeviceLock, time.Duration) (*APIKeyDeviceLock, bool, error) {
	return nil, true, nil
}

func (c *apiKeyConcurrencyTestCache) GetDeviceLock(context.Context, int64) (*APIKeyDeviceLock, error) {
	return nil, nil
}

func (c *apiKeyConcurrencyTestCache) DeleteDeviceLock(context.Context, int64) error {
	return nil
}

func (c *apiKeyConcurrencyTestCache) AcquireAPIKeySlot(ctx context.Context, keyID int64, maxConcurrency int, requestID string, ttl time.Duration) (bool, error) {
	c.acquireCalls++
	c.acquireKeyID = keyID
	c.acquireMax = maxConcurrency
	c.acquireReqID = requestID
	c.acquireTTL = ttl
	return c.acquired, c.err
}

func (c *apiKeyConcurrencyTestCache) ReleaseAPIKeySlot(ctx context.Context, keyID int64, requestID string) error {
	c.releaseCalls++
	c.releaseKeyID = keyID
	c.releaseReqID = requestID
	return nil
}

func (c *apiKeyConcurrencyTestCache) DeleteAPIKeySlots(context.Context, int64) error {
	return nil
}

func TestAPIKeyServiceAcquireConcurrencySlot(t *testing.T) {
	t.Run("limit zero bypasses cache", func(t *testing.T) {
		cache := &apiKeyConcurrencyTestCache{acquired: true}
		svc := NewAPIKeyService(nil, nil, nil, nil, nil, cache, &config.Config{})

		release, err := svc.AcquireConcurrencySlot(context.Background(), &APIKey{ID: 10, ConcurrencyLimit: 0})

		require.NoError(t, err)
		require.NotNil(t, release)
		release()
		require.Zero(t, cache.acquireCalls)
		require.Zero(t, cache.releaseCalls)
	})

	t.Run("acquired slot is released", func(t *testing.T) {
		cache := &apiKeyConcurrencyTestCache{acquired: true}
		svc := NewAPIKeyService(nil, nil, nil, nil, nil, cache, &config.Config{
			Gateway: config.GatewayConfig{ConcurrencySlotTTLMinutes: 2},
		})

		release, err := svc.AcquireConcurrencySlot(context.Background(), &APIKey{ID: 11, ConcurrencyLimit: 5})
		require.NoError(t, err)
		require.NotNil(t, release)
		require.Equal(t, 1, cache.acquireCalls)
		require.Equal(t, int64(11), cache.acquireKeyID)
		require.Equal(t, 5, cache.acquireMax)
		require.NotEmpty(t, cache.acquireReqID)
		require.Equal(t, 2*time.Minute, cache.acquireTTL)

		release()
		require.Equal(t, 1, cache.releaseCalls)
		require.Equal(t, int64(11), cache.releaseKeyID)
		require.Equal(t, cache.acquireReqID, cache.releaseReqID)
	})

	t.Run("full slot returns concurrency error", func(t *testing.T) {
		cache := &apiKeyConcurrencyTestCache{acquired: false}
		svc := NewAPIKeyService(nil, nil, nil, nil, nil, cache, &config.Config{})

		release, err := svc.AcquireConcurrencySlot(context.Background(), &APIKey{ID: 12, ConcurrencyLimit: 1})

		require.Nil(t, release)
		require.ErrorIs(t, err, ErrAPIKeyConcurrencyExceeded)
		require.Equal(t, 1, cache.acquireCalls)
		require.Zero(t, cache.releaseCalls)
	})

	t.Run("cache error is returned", func(t *testing.T) {
		cacheErr := errors.New("redis down")
		cache := &apiKeyConcurrencyTestCache{err: cacheErr}
		svc := NewAPIKeyService(nil, nil, nil, nil, nil, cache, &config.Config{})

		release, err := svc.AcquireConcurrencySlot(context.Background(), &APIKey{ID: 13, ConcurrencyLimit: 1})

		require.Nil(t, release)
		require.ErrorIs(t, err, cacheErr)
	})
}
