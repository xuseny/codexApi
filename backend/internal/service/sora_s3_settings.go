package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// SoraS3Settings contains the runtime S3 configuration used by Sora media storage.
type SoraS3Settings struct {
	Enabled                  bool
	Endpoint                 string
	Region                   string
	Bucket                   string
	AccessKeyID              string
	SecretAccessKey          string
	Prefix                   string
	ForcePathStyle           bool
	CDNURL                   string
	DefaultStorageQuotaBytes int64
}

// GetSoraS3Settings loads Sora S3 settings from the settings repository.
func (s *SettingService) GetSoraS3Settings(ctx context.Context) (*SoraS3Settings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, fmt.Errorf("setting service not available")
	}

	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeySoraS3Enabled,
		SettingKeySoraS3Endpoint,
		SettingKeySoraS3Region,
		SettingKeySoraS3Bucket,
		SettingKeySoraS3AccessKeyID,
		SettingKeySoraS3SecretAccessKey,
		SettingKeySoraS3Prefix,
		SettingKeySoraS3ForcePathStyle,
		SettingKeySoraS3CDNURL,
		SettingKeySoraDefaultStorageQuotaBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("get sora s3 settings: %w", err)
	}

	quotaBytes := int64(0)
	if raw := strings.TrimSpace(values[SettingKeySoraDefaultStorageQuotaBytes]); raw != "" {
		if parsed, parseErr := strconv.ParseInt(raw, 10, 64); parseErr == nil && parsed > 0 {
			quotaBytes = parsed
		}
	}

	return &SoraS3Settings{
		Enabled:                  strings.TrimSpace(values[SettingKeySoraS3Enabled]) == "true",
		Endpoint:                 strings.TrimSpace(values[SettingKeySoraS3Endpoint]),
		Region:                   strings.TrimSpace(values[SettingKeySoraS3Region]),
		Bucket:                   strings.TrimSpace(values[SettingKeySoraS3Bucket]),
		AccessKeyID:              strings.TrimSpace(values[SettingKeySoraS3AccessKeyID]),
		SecretAccessKey:          strings.TrimSpace(values[SettingKeySoraS3SecretAccessKey]),
		Prefix:                   strings.TrimSpace(values[SettingKeySoraS3Prefix]),
		ForcePathStyle:           strings.TrimSpace(values[SettingKeySoraS3ForcePathStyle]) == "true",
		CDNURL:                   strings.TrimSpace(values[SettingKeySoraS3CDNURL]),
		DefaultStorageQuotaBytes: quotaBytes,
	}, nil
}
