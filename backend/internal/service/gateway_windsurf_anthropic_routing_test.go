package service

import (
	"context"
	"testing"
	"time"
)

func TestGatewayServiceWindsurfBuiltinOAuthAllowedForAnthropicMessagesOnly(t *testing.T) {
	svc := &GatewayService{}
	account := &Account{
		Platform: PlatformWindsurf,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"windsurf_builtin": true,
		},
	}

	if svc.isAccountAllowedForPlatform(context.Background(), account, PlatformAnthropic, false) {
		t.Fatal("windsurf account should not be allowed for generic anthropic routing")
	}

	ctx := WithWindsurfAnthropicMessagesRouting(context.Background(), true)
	if !svc.isAccountAllowedForPlatform(ctx, account, PlatformAnthropic, false) {
		t.Fatal("windsurf builtin oauth account should be allowed for anthropic messages bridge")
	}
}

func TestMixedSchedulingQueryPlatformsAnthropicIncludesWindsurf(t *testing.T) {
	platforms := mixedSchedulingQueryPlatforms(PlatformAnthropic)
	seen := make(map[string]bool, len(platforms))
	for _, platform := range platforms {
		seen[platform] = true
	}

	for _, platform := range []string{PlatformAnthropic, PlatformAntigravity, PlatformWindsurf} {
		if !seen[platform] {
			t.Fatalf("expected mixed anthropic platforms to include %s, got %v", platform, platforms)
		}
	}
}

func TestSchedulerPlatformsForWindsurfAccountRebuildIncludesAnthropic(t *testing.T) {
	platforms := schedulerPlatformsForAccountRebuild(&Account{Platform: PlatformWindsurf})
	seen := make(map[string]int, len(platforms))
	for _, platform := range platforms {
		seen[platform]++
	}

	for _, platform := range []string{PlatformWindsurf, PlatformOpenAI, PlatformAnthropic} {
		if seen[platform] != 1 {
			t.Fatalf("expected windsurf account rebuild platforms to include %s once, got %v", platform, platforms)
		}
	}
}

func TestWindsurfBuiltinAccountSupportsRequestedModel(t *testing.T) {
	account := &Account{
		Platform:    PlatformWindsurf,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"windsurf_builtin": true},
	}

	if !windsurfBuiltinAccountSupportsRequestedModel(account, "claude-opus-4-7") {
		t.Fatal("expected windsurf builtin account to support claude-opus-4-7 alias")
	}
	if windsurfBuiltinAccountSupportsRequestedModel(account, "not-a-windsurf-model") {
		t.Fatal("expected windsurf builtin account to reject unknown model without mapping")
	}

	account.Credentials["model_mapping"] = map[string]any{
		"custom-claude": "claude-opus-4-7-xhigh",
		"bad-model":     "not-a-windsurf-model",
	}
	if !windsurfBuiltinAccountSupportsRequestedModel(account, "custom-claude") {
		t.Fatal("expected custom mapping to a windsurf model to be supported")
	}
	if windsurfBuiltinAccountSupportsRequestedModel(account, "bad-model") {
		t.Fatal("expected mapping to unknown windsurf model to be rejected")
	}
	if windsurfBuiltinAccountSupportsRequestedModel(account, "claude-opus-4-7") {
		t.Fatal("expected non-whitelisted model to be rejected when model_mapping is configured")
	}
}

func TestSchedulerSnapshotRefreshesEmptyAnthropicWindsurfBucket(t *testing.T) {
	service := &SchedulerSnapshotService{}
	ctx := WithWindsurfAnthropicMessagesRouting(context.Background(), true)
	bucket := SchedulerBucket{GroupID: 1, Platform: PlatformAnthropic, Mode: SchedulerModeMixed}

	if !service.shouldRefreshEmptySchedulerSnapshot(ctx, bucket, true) {
		t.Fatal("expected empty anthropic mixed snapshot to refresh during windsurf anthropic routing")
	}
	if service.shouldRefreshEmptySchedulerSnapshot(context.Background(), bucket, true) {
		t.Fatal("expected normal anthropic routing to trust empty snapshot")
	}
}

func TestSchedulerRebuildBucketsWithDefaultsIncludesAnthropicMixed(t *testing.T) {
	cache := &windsurfRoutingSchedulerCache{
		buckets: []SchedulerBucket{{GroupID: 0, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}},
	}
	service := NewSchedulerSnapshotService(cache, nil, nil, nil, nil)

	buckets, err := service.rebuildBucketsWithDefaults(context.Background())
	if err != nil {
		t.Fatalf("rebuildBucketsWithDefaults error: %v", err)
	}

	seen := make(map[string]bool, len(buckets))
	for _, bucket := range buckets {
		seen[bucket.String()] = true
	}
	if !seen[(SchedulerBucket{GroupID: 0, Platform: PlatformAnthropic, Mode: SchedulerModeMixed}).String()] {
		t.Fatalf("expected default anthropic mixed bucket to be merged, got %v", buckets)
	}
}

type windsurfRoutingSchedulerCache struct {
	buckets []SchedulerBucket
}

func (c *windsurfRoutingSchedulerCache) GetSnapshot(context.Context, SchedulerBucket) ([]*Account, bool, error) {
	return nil, false, nil
}

func (c *windsurfRoutingSchedulerCache) SetSnapshot(context.Context, SchedulerBucket, []Account) error {
	return nil
}

func (c *windsurfRoutingSchedulerCache) GetAccount(context.Context, int64) (*Account, error) {
	return nil, nil
}

func (c *windsurfRoutingSchedulerCache) SetAccount(context.Context, *Account) error {
	return nil
}

func (c *windsurfRoutingSchedulerCache) DeleteAccount(context.Context, int64) error {
	return nil
}

func (c *windsurfRoutingSchedulerCache) UpdateLastUsed(context.Context, map[int64]time.Time) error {
	return nil
}

func (c *windsurfRoutingSchedulerCache) TryLockBucket(context.Context, SchedulerBucket, time.Duration) (bool, error) {
	return true, nil
}

func (c *windsurfRoutingSchedulerCache) ListBuckets(context.Context) ([]SchedulerBucket, error) {
	return c.buckets, nil
}

func (c *windsurfRoutingSchedulerCache) GetOutboxWatermark(context.Context) (int64, error) {
	return 0, nil
}

func (c *windsurfRoutingSchedulerCache) SetOutboxWatermark(context.Context, int64) error {
	return nil
}
