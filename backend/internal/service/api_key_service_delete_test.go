//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type apiKeyRepoStub struct {
	getByIDFn          func(ctx context.Context, id int64) (*APIKey, error)
	getKeyAndOwnerIDFn func(ctx context.Context, id int64) (string, int64, error)
	verifyOwnershipFn  func(ctx context.Context, userID int64, apiKeyIDs []int64) ([]int64, error)
	deleteFn           func(ctx context.Context, id int64) error
	updateLastUsedFn   func(ctx context.Context, id int64, usedAt time.Time) error
	deletedIDs         []int64
	touchedIDs         []int64
	touchedUsedAts     []time.Time
}

func (s *apiKeyRepoStub) Create(ctx context.Context, key *APIKey) error {
	panic("unexpected Create call")
}

func (s *apiKeyRepoStub) GetByID(ctx context.Context, id int64) (*APIKey, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, id)
	}
	panic("unexpected GetByID call")
}

func (s *apiKeyRepoStub) GetKeyAndOwnerID(ctx context.Context, id int64) (string, int64, error) {
	if s.getKeyAndOwnerIDFn != nil {
		return s.getKeyAndOwnerIDFn(ctx, id)
	}
	panic("unexpected GetKeyAndOwnerID call")
}

func (s *apiKeyRepoStub) GetByKey(ctx context.Context, key string) (*APIKey, error) {
	panic("unexpected GetByKey call")
}

func (s *apiKeyRepoStub) GetByKeyForAuth(ctx context.Context, key string) (*APIKey, error) {
	panic("unexpected GetByKeyForAuth call")
}

func (s *apiKeyRepoStub) Update(ctx context.Context, key *APIKey) error {
	panic("unexpected Update call")
}

func (s *apiKeyRepoStub) Delete(ctx context.Context, id int64) error {
	s.deletedIDs = append(s.deletedIDs, id)
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func (s *apiKeyRepoStub) ListByUserID(ctx context.Context, userID int64, params pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserID call")
}

func (s *apiKeyRepoStub) VerifyOwnership(ctx context.Context, userID int64, apiKeyIDs []int64) ([]int64, error) {
	if s.verifyOwnershipFn != nil {
		return s.verifyOwnershipFn(ctx, userID, apiKeyIDs)
	}
	panic("unexpected VerifyOwnership call")
}

func (s *apiKeyRepoStub) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	panic("unexpected CountByUserID call")
}

func (s *apiKeyRepoStub) ExistsByKey(ctx context.Context, key string) (bool, error) {
	panic("unexpected ExistsByKey call")
}

func (s *apiKeyRepoStub) ListByGroupID(ctx context.Context, groupID int64, params pagination.PaginationParams) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByGroupID call")
}

func (s *apiKeyRepoStub) SearchAPIKeys(ctx context.Context, userID int64, keyword string, limit int) ([]APIKey, error) {
	panic("unexpected SearchAPIKeys call")
}

func (s *apiKeyRepoStub) ClearGroupIDByGroupID(ctx context.Context, groupID int64) (int64, error) {
	panic("unexpected ClearGroupIDByGroupID call")
}

func (s *apiKeyRepoStub) UpdateGroupIDByUserAndGroup(ctx context.Context, userID, oldGroupID, newGroupID int64) (int64, error) {
	panic("unexpected UpdateGroupIDByUserAndGroup call")
}

func (s *apiKeyRepoStub) CountByGroupID(ctx context.Context, groupID int64) (int64, error) {
	panic("unexpected CountByGroupID call")
}

func (s *apiKeyRepoStub) ListKeysByUserID(ctx context.Context, userID int64) ([]string, error) {
	panic("unexpected ListKeysByUserID call")
}

func (s *apiKeyRepoStub) ListKeysByGroupID(ctx context.Context, groupID int64) ([]string, error) {
	panic("unexpected ListKeysByGroupID call")
}

func (s *apiKeyRepoStub) IncrementQuotaUsed(ctx context.Context, id int64, amount float64) (float64, error) {
	panic("unexpected IncrementQuotaUsed call")
}

func (s *apiKeyRepoStub) UpdateLastUsed(ctx context.Context, id int64, usedAt time.Time) error {
	s.touchedIDs = append(s.touchedIDs, id)
	s.touchedUsedAts = append(s.touchedUsedAts, usedAt)
	if s.updateLastUsedFn != nil {
		return s.updateLastUsedFn(ctx, id, usedAt)
	}
	return nil
}

func (s *apiKeyRepoStub) IncrementRateLimitUsage(ctx context.Context, id int64, cost float64) error {
	panic("unexpected IncrementRateLimitUsage call")
}

func (s *apiKeyRepoStub) ResetRateLimitWindows(ctx context.Context, id int64) error {
	panic("unexpected ResetRateLimitWindows call")
}

func (s *apiKeyRepoStub) GetRateLimitData(ctx context.Context, id int64) (*APIKeyRateLimitData, error) {
	panic("unexpected GetRateLimitData call")
}

type apiKeyCacheStub struct {
	invalidated    []int64
	deleteAuthKeys []string
}

func (s *apiKeyCacheStub) GetCreateAttemptCount(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}

func (s *apiKeyCacheStub) IncrementCreateAttemptCount(ctx context.Context, userID int64) error {
	return nil
}

func (s *apiKeyCacheStub) DeleteCreateAttemptCount(ctx context.Context, userID int64) error {
	s.invalidated = append(s.invalidated, userID)
	return nil
}

func (s *apiKeyCacheStub) IncrementDailyUsage(ctx context.Context, apiKey string) error {
	return nil
}

func (s *apiKeyCacheStub) SetDailyUsageExpiry(ctx context.Context, apiKey string, ttl time.Duration) error {
	return nil
}

func (s *apiKeyCacheStub) GetAuthCache(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error) {
	return nil, nil
}

func (s *apiKeyCacheStub) SetAuthCache(ctx context.Context, key string, entry *APIKeyAuthCacheEntry, ttl time.Duration) error {
	return nil
}

func (s *apiKeyCacheStub) DeleteAuthCache(ctx context.Context, key string) error {
	s.deleteAuthKeys = append(s.deleteAuthKeys, key)
	return nil
}

func (s *apiKeyCacheStub) PublishAuthCacheInvalidation(ctx context.Context, cacheKey string) error {
	return nil
}

func (s *apiKeyCacheStub) SubscribeAuthCacheInvalidation(ctx context.Context, handler func(cacheKey string)) error {
	return nil
}

func (s *apiKeyCacheStub) AcquireDeviceLock(ctx context.Context, keyID int64, lock *APIKeyDeviceLock, ttl time.Duration) (*APIKeyDeviceLock, bool, error) {
	return nil, true, nil
}

func (s *apiKeyCacheStub) GetDeviceLock(ctx context.Context, keyID int64) (*APIKeyDeviceLock, error) {
	return nil, nil
}

func (s *apiKeyCacheStub) DeleteDeviceLock(ctx context.Context, keyID int64) error {
	return nil
}

func TestApiKeyService_Delete_OwnerMismatch(t *testing.T) {
	repo := &apiKeyRepoStub{
		getKeyAndOwnerIDFn: func(ctx context.Context, id int64) (string, int64, error) {
			return "k", 1, nil
		},
	}
	cache := &apiKeyCacheStub{}
	svc := &APIKeyService{apiKeyRepo: repo, cache: cache}

	err := svc.Delete(context.Background(), 10, 2)
	require.ErrorIs(t, err, ErrInsufficientPerms)
	require.Empty(t, repo.deletedIDs)
	require.Empty(t, cache.invalidated)
	require.Empty(t, cache.deleteAuthKeys)
}

func TestApiKeyService_Delete_Success(t *testing.T) {
	repo := &apiKeyRepoStub{
		getKeyAndOwnerIDFn: func(ctx context.Context, id int64) (string, int64, error) {
			require.Equal(t, int64(42), id)
			return "k", 7, nil
		},
	}
	cache := &apiKeyCacheStub{}
	svc := &APIKeyService{apiKeyRepo: repo, cache: cache}
	svc.lastUsedTouchL1.Store(int64(42), time.Now())

	err := svc.Delete(context.Background(), 42, 7)
	require.NoError(t, err)
	require.Equal(t, []int64{42}, repo.deletedIDs)
	require.Equal(t, []int64{7}, cache.invalidated)
	require.Equal(t, []string{svc.authCacheKey("k")}, cache.deleteAuthKeys)
	_, exists := svc.lastUsedTouchL1.Load(int64(42))
	require.False(t, exists, "delete should clear touch debounce cache")
}

func TestApiKeyService_Delete_NotFound(t *testing.T) {
	repo := &apiKeyRepoStub{
		getKeyAndOwnerIDFn: func(ctx context.Context, id int64) (string, int64, error) {
			return "", 0, ErrAPIKeyNotFound
		},
	}
	cache := &apiKeyCacheStub{}
	svc := &APIKeyService{apiKeyRepo: repo, cache: cache}

	err := svc.Delete(context.Background(), 99, 1)
	require.ErrorIs(t, err, ErrAPIKeyNotFound)
	require.Empty(t, repo.deletedIDs)
	require.Empty(t, cache.invalidated)
	require.Empty(t, cache.deleteAuthKeys)
}

func TestApiKeyService_Delete_DeleteFails(t *testing.T) {
	repo := &apiKeyRepoStub{
		getKeyAndOwnerIDFn: func(ctx context.Context, id int64) (string, int64, error) {
			require.Equal(t, int64(3), id)
			return "k", 3, nil
		},
		deleteFn: func(ctx context.Context, id int64) error {
			return errors.New("delete failed")
		},
	}
	cache := &apiKeyCacheStub{}
	svc := &APIKeyService{apiKeyRepo: repo, cache: cache}

	err := svc.Delete(context.Background(), 3, 3)
	require.Error(t, err)
	require.ErrorContains(t, err, "delete api key")
	require.Equal(t, []int64{3}, repo.deletedIDs)
	require.Equal(t, []int64{3}, cache.invalidated)
	require.Equal(t, []string{svc.authCacheKey("k")}, cache.deleteAuthKeys)
}

func TestApiKeyService_BatchDelete_Success(t *testing.T) {
	repo := &apiKeyRepoStub{
		verifyOwnershipFn: func(ctx context.Context, userID int64, apiKeyIDs []int64) ([]int64, error) {
			require.Equal(t, int64(7), userID)
			require.Equal(t, []int64{11, 12}, apiKeyIDs)
			return []int64{11, 12}, nil
		},
		getKeyAndOwnerIDFn: func(ctx context.Context, id int64) (string, int64, error) {
			switch id {
			case 11:
				return "k11", 7, nil
			case 12:
				return "k12", 7, nil
			default:
				return "", 0, ErrAPIKeyNotFound
			}
		},
	}
	cache := &apiKeyCacheStub{}
	svc := &APIKeyService{apiKeyRepo: repo, cache: cache}

	deleted, err := svc.BatchDelete(context.Background(), []int64{11, 12}, 7)
	require.NoError(t, err)
	require.EqualValues(t, 2, deleted)
	require.Equal(t, []int64{11, 12}, repo.deletedIDs)
	require.Equal(t, []int64{7, 7}, cache.invalidated)
	require.Equal(t, []string{svc.authCacheKey("k11"), svc.authCacheKey("k12")}, cache.deleteAuthKeys)
}

func TestApiKeyService_BatchDelete_SkipsUnownedKeys(t *testing.T) {
	repo := &apiKeyRepoStub{
		verifyOwnershipFn: func(ctx context.Context, userID int64, apiKeyIDs []int64) ([]int64, error) {
			require.Equal(t, int64(9), userID)
			require.Equal(t, []int64{21, 22, 23}, apiKeyIDs)
			return []int64{21, 23}, nil
		},
		getKeyAndOwnerIDFn: func(ctx context.Context, id int64) (string, int64, error) {
			switch id {
			case 21:
				return "k21", 9, nil
			case 23:
				return "k23", 9, nil
			default:
				return "", 0, ErrAPIKeyNotFound
			}
		},
	}
	cache := &apiKeyCacheStub{}
	svc := &APIKeyService{apiKeyRepo: repo, cache: cache}

	deleted, err := svc.BatchDelete(context.Background(), []int64{21, 22, 23}, 9)
	require.NoError(t, err)
	require.EqualValues(t, 2, deleted)
	require.Equal(t, []int64{21, 23}, repo.deletedIDs)
	require.Equal(t, []int64{9, 9}, cache.invalidated)
	require.Equal(t, []string{svc.authCacheKey("k21"), svc.authCacheKey("k23")}, cache.deleteAuthKeys)
}
