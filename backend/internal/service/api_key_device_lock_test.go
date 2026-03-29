//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type apiKeyDeviceInfoCacheStub struct {
	lock *APIKeyDeviceLock
}

func (s *apiKeyDeviceInfoCacheStub) GetCreateAttemptCount(context.Context, int64) (int, error) {
	return 0, nil
}

func (s *apiKeyDeviceInfoCacheStub) IncrementCreateAttemptCount(context.Context, int64) error {
	return nil
}

func (s *apiKeyDeviceInfoCacheStub) DeleteCreateAttemptCount(context.Context, int64) error {
	return nil
}

func (s *apiKeyDeviceInfoCacheStub) IncrementDailyUsage(context.Context, string) error {
	return nil
}

func (s *apiKeyDeviceInfoCacheStub) SetDailyUsageExpiry(context.Context, string, time.Duration) error {
	return nil
}

func (s *apiKeyDeviceInfoCacheStub) GetAuthCache(context.Context, string) (*APIKeyAuthCacheEntry, error) {
	return nil, nil
}

func (s *apiKeyDeviceInfoCacheStub) SetAuthCache(context.Context, string, *APIKeyAuthCacheEntry, time.Duration) error {
	return nil
}

func (s *apiKeyDeviceInfoCacheStub) DeleteAuthCache(context.Context, string) error {
	return nil
}

func (s *apiKeyDeviceInfoCacheStub) PublishAuthCacheInvalidation(context.Context, string) error {
	return nil
}

func (s *apiKeyDeviceInfoCacheStub) SubscribeAuthCacheInvalidation(context.Context, func(string)) error {
	return nil
}

func (s *apiKeyDeviceInfoCacheStub) AcquireDeviceLock(context.Context, int64, *APIKeyDeviceLock, time.Duration) (*APIKeyDeviceLock, bool, error) {
	return nil, true, nil
}

func (s *apiKeyDeviceInfoCacheStub) GetDeviceLock(context.Context, int64) (*APIKeyDeviceLock, error) {
	return s.lock, nil
}

func (s *apiKeyDeviceInfoCacheStub) DeleteDeviceLock(context.Context, int64) error {
	return nil
}

func TestAPIKeyService_GetOnlineDeviceInfo(t *testing.T) {
	now := time.Now().UTC()
	svc := &APIKeyService{
		cache: &apiKeyDeviceInfoCacheStub{
			lock: &APIKeyDeviceLock{
				DeviceLabel: "IP 1.2.3.4 / codex_cli_rs",
				ClientIP:    "1.2.3.4",
				UserAgent:   "codex_cli_rs/0.104.0",
				UpdatedAt:   now,
			},
		},
	}

	info, err := svc.GetOnlineDeviceInfo(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, "IP 1.2.3.4 / codex_cli_rs", info.DeviceLabel)
	require.Equal(t, "1.2.3.4", info.ClientIP)
	require.Equal(t, "codex_cli_rs/0.104.0", info.UserAgent)
	require.NotNil(t, info.UpdatedAt)
	require.WithinDuration(t, now, *info.UpdatedAt, time.Second)
}
