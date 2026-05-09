package service

import (
	"context"
	"testing"
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
