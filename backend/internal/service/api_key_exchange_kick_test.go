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

type apiKeyExchangeKickRepoStub struct {
	record *APIKeyExchangeCode
	err    error
}

func (s *apiKeyExchangeKickRepoStub) CreateBatch(context.Context, []APIKeyExchangeCode) error {
	panic("unexpected CreateBatch call")
}

func (s *apiKeyExchangeKickRepoStub) List(context.Context, pagination.PaginationParams, APIKeyExchangeCodeListFilters) ([]APIKeyExchangeCode, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *apiKeyExchangeKickRepoStub) GetByID(context.Context, int64) (*APIKeyExchangeCode, error) {
	panic("unexpected GetByID call")
}

func (s *apiKeyExchangeKickRepoStub) GetByCode(ctx context.Context, code string) (*APIKeyExchangeCode, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.record == nil {
		return nil, ErrAPIKeyExchangeCodeNotFound
	}
	clone := *s.record
	if s.record.APIKey != nil {
		apiKeyClone := *s.record.APIKey
		clone.APIKey = &apiKeyClone
	}
	return &clone, nil
}

func (s *apiKeyExchangeKickRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}

func (s *apiKeyExchangeKickRepoStub) Resolve(context.Context, string, string, string, string) (*APIKeyExchangeCode, string, error) {
	panic("unexpected Resolve call")
}

func (s *apiKeyExchangeKickRepoStub) GetUsageSummary(context.Context, int64, time.Time, time.Time) (*APIKeyExchangeUsageSummary, error) {
	panic("unexpected GetUsageSummary call")
}

type apiKeyDeviceDeleteCacheStub struct {
	deletedIDs []int64
}

func (s *apiKeyDeviceDeleteCacheStub) GetCreateAttemptCount(context.Context, int64) (int, error) {
	return 0, nil
}

func (s *apiKeyDeviceDeleteCacheStub) IncrementCreateAttemptCount(context.Context, int64) error {
	return nil
}

func (s *apiKeyDeviceDeleteCacheStub) DeleteCreateAttemptCount(context.Context, int64) error {
	return nil
}

func (s *apiKeyDeviceDeleteCacheStub) IncrementDailyUsage(context.Context, string) error {
	return nil
}

func (s *apiKeyDeviceDeleteCacheStub) SetDailyUsageExpiry(context.Context, string, time.Duration) error {
	return nil
}

func (s *apiKeyDeviceDeleteCacheStub) GetAuthCache(context.Context, string) (*APIKeyAuthCacheEntry, error) {
	return nil, nil
}

func (s *apiKeyDeviceDeleteCacheStub) SetAuthCache(context.Context, string, *APIKeyAuthCacheEntry, time.Duration) error {
	return nil
}

func (s *apiKeyDeviceDeleteCacheStub) DeleteAuthCache(context.Context, string) error {
	return nil
}

func (s *apiKeyDeviceDeleteCacheStub) PublishAuthCacheInvalidation(context.Context, string) error {
	return nil
}

func (s *apiKeyDeviceDeleteCacheStub) SubscribeAuthCacheInvalidation(context.Context, func(string)) error {
	return nil
}

func (s *apiKeyDeviceDeleteCacheStub) AcquireDeviceLock(context.Context, int64, *APIKeyDeviceLock, time.Duration) (*APIKeyDeviceLock, bool, error) {
	return nil, true, nil
}

func (s *apiKeyDeviceDeleteCacheStub) GetDeviceLock(context.Context, int64) (*APIKeyDeviceLock, error) {
	return nil, nil
}

func (s *apiKeyDeviceDeleteCacheStub) DeleteDeviceLock(_ context.Context, keyID int64) error {
	s.deletedIDs = append(s.deletedIDs, keyID)
	return nil
}

func TestAPIKeyExchangeService_KickOffline_Success(t *testing.T) {
	apiKeyID := int64(123)
	repo := &apiKeyExchangeKickRepoStub{
		record: &APIKeyExchangeCode{
			Code:     "ABCD-EFGH-IJKL-MNOP",
			Status:   APIKeyExchangeStatusActivated,
			APIKeyID: &apiKeyID,
			APIKey: &APIKey{
				ID:   apiKeyID,
				Name: "Exchange-Key",
			},
		},
	}
	cache := &apiKeyDeviceDeleteCacheStub{}
	apiKeyService := &APIKeyService{cache: cache}
	svc := NewAPIKeyExchangeService(repo, nil, apiKeyService)

	result, err := svc.KickOffline(context.Background(), " abcd-efgh-ijkl-mnop ")
	require.NoError(t, err)
	require.Equal(t, apiKeyID, result.APIKeyID)
	require.Equal(t, "Exchange-Key", result.APIKeyName)
	require.True(t, result.Released)
	require.Equal(t, []int64{apiKeyID}, cache.deletedIDs)
}

func TestAPIKeyExchangeService_KickOffline_UnusedCode(t *testing.T) {
	repo := &apiKeyExchangeKickRepoStub{
		record: &APIKeyExchangeCode{
			Code:   "ABCD-EFGH-IJKL-MNOP",
			Status: APIKeyExchangeStatusUnused,
		},
	}
	svc := NewAPIKeyExchangeService(repo, nil, &APIKeyService{cache: &apiKeyDeviceDeleteCacheStub{}})

	_, err := svc.KickOffline(context.Background(), "ABCD-EFGH-IJKL-MNOP")
	require.ErrorIs(t, err, ErrAPIKeyExchangeCodeNotActivated)
}

func TestAPIKeyExchangeService_KickOffline_RepoError(t *testing.T) {
	repo := &apiKeyExchangeKickRepoStub{err: errors.New("db down")}
	svc := NewAPIKeyExchangeService(repo, nil, &APIKeyService{cache: &apiKeyDeviceDeleteCacheStub{}})

	_, err := svc.KickOffline(context.Background(), "ABCD-EFGH-IJKL-MNOP")
	require.ErrorContains(t, err, "db down")
}
