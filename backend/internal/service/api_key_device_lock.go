package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const apiKeyDeviceLockTTL = 10 * time.Minute

var ErrAPIKeySingleDeviceLimit = infraerrors.Conflict(
	"API_KEY_SINGLE_DEVICE_LIMIT",
	"API key \u53ea\u80fd\u540c\u65f6\u5728\u7ebf\u4e00\u53f0\u8bbe\u5907\uff0c\u8bf7\u5148\u5728\u539f\u8bbe\u5907\u9000\u51fa\uff0c\u6216\u524d\u5f80 Key \u5151\u6362\u9875\u9762\u6267\u884c\u8e22\u4e0b\u7ebf\u3002",
)

type APIKeyDeviceLock struct {
	Fingerprint string    `json:"fingerprint"`
	DeviceLabel string    `json:"device_label,omitempty"`
	ClientIP    string    `json:"client_ip,omitempty"`
	UserAgent   string    `json:"user_agent,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type APIKeyOnlineDeviceInfo struct {
	DeviceLabel string     `json:"device_label"`
	ClientIP    string     `json:"client_ip,omitempty"`
	UserAgent   string     `json:"user_agent,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

func (s *APIKeyService) EnforceSingleDeviceLock(
	ctx context.Context,
	keyID int64,
	fingerprint string,
	deviceLabel string,
	clientIP string,
	userAgent string,
) error {
	if s == nil || s.cache == nil || keyID <= 0 {
		return nil
	}

	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return nil
	}

	lock := &APIKeyDeviceLock{
		Fingerprint: fingerprint,
		DeviceLabel: strings.TrimSpace(deviceLabel),
		ClientIP:    strings.TrimSpace(clientIP),
		UserAgent:   strings.TrimSpace(userAgent),
		UpdatedAt:   time.Now().UTC(),
	}

	current, acquired, err := s.cache.AcquireDeviceLock(ctx, keyID, lock, apiKeyDeviceLockTTL)
	if err != nil {
		// Fail open on auth path so Redis failures do not block requests.
		return nil
	}
	if acquired {
		return nil
	}

	appErr := ErrAPIKeySingleDeviceLimit
	if current != nil && strings.TrimSpace(current.DeviceLabel) != "" {
		appErr = appErr.WithMetadata(map[string]string{
			"online_device": current.DeviceLabel,
		})
	}
	return appErr
}

func (s *APIKeyService) ClearDeviceLock(ctx context.Context, keyID int64) error {
	if s == nil || s.cache == nil || keyID <= 0 {
		return nil
	}
	return s.cache.DeleteDeviceLock(ctx, keyID)
}

func (s *APIKeyService) GetOnlineDeviceInfo(ctx context.Context, keyID int64) (*APIKeyOnlineDeviceInfo, error) {
	if s == nil || s.cache == nil || keyID <= 0 {
		return nil, nil
	}

	lock, err := s.cache.GetDeviceLock(ctx, keyID)
	if err != nil || lock == nil {
		return nil, err
	}

	info := &APIKeyOnlineDeviceInfo{
		DeviceLabel: strings.TrimSpace(lock.DeviceLabel),
		ClientIP:    strings.TrimSpace(lock.ClientIP),
		UserAgent:   strings.TrimSpace(lock.UserAgent),
	}
	if !lock.UpdatedAt.IsZero() {
		updatedAt := lock.UpdatedAt
		info.UpdatedAt = &updatedAt
	}
	if info.DeviceLabel == "" && info.ClientIP == "" && info.UserAgent == "" {
		return nil, nil
	}
	return info, nil
}
