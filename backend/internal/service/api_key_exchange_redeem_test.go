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

type apiKeyExchangeRedeemRepoStub struct {
	record            *APIKeyExchangeCode
	amount            float64
	err               error
	summary           *APIKeyExchangeUsageSummary
	lastExchangeCode  string
	lastRedeemCode    string
}

func (s *apiKeyExchangeRedeemRepoStub) CreateBatch(context.Context, []APIKeyExchangeCode) error {
	panic("unexpected CreateBatch call")
}

func (s *apiKeyExchangeRedeemRepoStub) List(context.Context, pagination.PaginationParams, APIKeyExchangeCodeListFilters) ([]APIKeyExchangeCode, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *apiKeyExchangeRedeemRepoStub) GetByID(context.Context, int64) (*APIKeyExchangeCode, error) {
	panic("unexpected GetByID call")
}

func (s *apiKeyExchangeRedeemRepoStub) GetByCode(context.Context, string) (*APIKeyExchangeCode, error) {
	panic("unexpected GetByCode call")
}

func (s *apiKeyExchangeRedeemRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}

func (s *apiKeyExchangeRedeemRepoStub) Resolve(context.Context, string, string, string, string) (*APIKeyExchangeCode, string, error) {
	panic("unexpected Resolve call")
}

func (s *apiKeyExchangeRedeemRepoStub) RedeemQuota(_ context.Context, exchangeCode string, redeemCode string) (*APIKeyExchangeCode, float64, error) {
	s.lastExchangeCode = exchangeCode
	s.lastRedeemCode = redeemCode
	if s.err != nil {
		return nil, 0, s.err
	}
	if s.record == nil {
		return nil, 0, ErrAPIKeyExchangeOrphanedAPIKey
	}
	clone := *s.record
	if s.record.APIKey != nil {
		apiKeyClone := *s.record.APIKey
		clone.APIKey = &apiKeyClone
	}
	if s.record.Group != nil {
		groupClone := *s.record.Group
		clone.Group = &groupClone
	}
	return &clone, s.amount, nil
}

func (s *apiKeyExchangeRedeemRepoStub) GetUsageSummary(context.Context, int64, time.Time, time.Time) (*APIKeyExchangeUsageSummary, error) {
	if s.summary != nil {
		return s.summary, nil
	}
	return &APIKeyExchangeUsageSummary{}, nil
}

type apiKeyExchangeRedeemCacheStub struct {
	deletedAuthCache []string
}

func (s *apiKeyExchangeRedeemCacheStub) GetCreateAttemptCount(context.Context, int64) (int, error) {
	return 0, nil
}

func (s *apiKeyExchangeRedeemCacheStub) IncrementCreateAttemptCount(context.Context, int64) error {
	return nil
}

func (s *apiKeyExchangeRedeemCacheStub) DeleteCreateAttemptCount(context.Context, int64) error {
	return nil
}

func (s *apiKeyExchangeRedeemCacheStub) IncrementDailyUsage(context.Context, string) error {
	return nil
}

func (s *apiKeyExchangeRedeemCacheStub) SetDailyUsageExpiry(context.Context, string, time.Duration) error {
	return nil
}

func (s *apiKeyExchangeRedeemCacheStub) GetAuthCache(context.Context, string) (*APIKeyAuthCacheEntry, error) {
	return nil, nil
}

func (s *apiKeyExchangeRedeemCacheStub) SetAuthCache(context.Context, string, *APIKeyAuthCacheEntry, time.Duration) error {
	return nil
}

func (s *apiKeyExchangeRedeemCacheStub) DeleteAuthCache(_ context.Context, key string) error {
	s.deletedAuthCache = append(s.deletedAuthCache, key)
	return nil
}

func (s *apiKeyExchangeRedeemCacheStub) PublishAuthCacheInvalidation(context.Context, string) error {
	return nil
}

func (s *apiKeyExchangeRedeemCacheStub) SubscribeAuthCacheInvalidation(context.Context, func(string)) error {
	return nil
}

func (s *apiKeyExchangeRedeemCacheStub) AcquireDeviceLock(context.Context, int64, *APIKeyDeviceLock, time.Duration) (*APIKeyDeviceLock, bool, error) {
	return nil, true, nil
}

func (s *apiKeyExchangeRedeemCacheStub) GetDeviceLock(context.Context, int64) (*APIKeyDeviceLock, error) {
	return nil, nil
}

func (s *apiKeyExchangeRedeemCacheStub) DeleteDeviceLock(context.Context, int64) error {
	return nil
}

func TestAPIKeyExchangeService_RedeemQuota_Success(t *testing.T) {
	apiKeyID := int64(42)
	repo := &apiKeyExchangeRedeemRepoStub{
		record: &APIKeyExchangeCode{
			Code:   "ABCD-EFGH-IJKL-MNOP",
			Status: APIKeyExchangeStatusActivated,
			Group:  &Group{ID: 7, Name: "g", Platform: PlatformOpenAI},
			APIKey: &APIKey{
				ID:        apiKeyID,
				Key:       "sk-test",
				Name:      "Exchange-Key",
				Status:    StatusActive,
				Quota:     25,
				QuotaUsed: 12,
			},
			APIKeyID: func() *int64 { v := apiKeyID; return &v }(),
		},
		amount: 8.5,
		summary: &APIKeyExchangeUsageSummary{
			TotalRequests:   5,
			TodayTokens:     4096,
			TodayActualCost: 1.25,
			TotalActualCost: 9.75,
		},
	}
	cache := &apiKeyExchangeRedeemCacheStub{}
	svc := NewAPIKeyExchangeService(repo, nil, &APIKeyService{cache: cache})

	result, err := svc.RedeemQuota(context.Background(), " abcd-efgh-ijkl-mnop ", "quota-code-1", "Asia/Shanghai")
	require.NoError(t, err)
	require.Equal(t, "ABCD-EFGH-IJKL-MNOP", repo.lastExchangeCode)
	require.Equal(t, "QUOTA-CODE-1", repo.lastRedeemCode)
	require.Equal(t, 8.5, result.Amount)
	require.NotNil(t, result.Exchange)
	require.Equal(t, apiKeyID, result.Exchange.APIKeyID)
	require.Equal(t, 25.0, result.Exchange.Quota)
	require.Equal(t, 12.0, result.Exchange.QuotaUsed)
	require.Equal(t, int64(4096), result.Exchange.TodayTokens)
	require.Equal(t, int64(5), result.Exchange.TotalRequests)
	require.Len(t, cache.deletedAuthCache, 1)
}

func TestAPIKeyExchangeService_RedeemQuota_RepoError(t *testing.T) {
	repo := &apiKeyExchangeRedeemRepoStub{err: errors.New("db down")}
	svc := NewAPIKeyExchangeService(repo, nil, &APIKeyService{})

	_, err := svc.RedeemQuota(context.Background(), "ABCD-EFGH-IJKL-MNOP", "quota-code-1", "Asia/Shanghai")
	require.ErrorContains(t, err, "db down")
}
