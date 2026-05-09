//go:build unit

package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildSchedulerMetadataAccount_KeepsOpenAIWSFlags(t *testing.T) {
	account := service.Account{
		ID:       42,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Extra: map[string]any{
			"openai_oauth_responses_websockets_v2_enabled": true,
			"openai_oauth_responses_websockets_v2_mode":    service.OpenAIWSIngressModePassthrough,
			"openai_ws_force_http":                         true,
			"mixed_scheduling":                             true,
			"unused_large_field":                           "drop-me",
		},
	}

	got := buildSchedulerMetadataAccount(account)

	require.Equal(t, true, got.Extra["openai_oauth_responses_websockets_v2_enabled"])
	require.Equal(t, service.OpenAIWSIngressModePassthrough, got.Extra["openai_oauth_responses_websockets_v2_mode"])
	require.Equal(t, true, got.Extra["openai_ws_force_http"])
	require.Equal(t, true, got.Extra["mixed_scheduling"])
	require.Nil(t, got.Extra["unused_large_field"])
}

func TestBuildSchedulerMetadataAccount_KeepsWindsurfRoutingFlag(t *testing.T) {
	account := service.Account{
		ID:       6335,
		Platform: service.PlatformWindsurf,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"windsurf_builtin": true,
			"access_token":     "secret-token",
		},
	}

	got := buildSchedulerMetadataAccount(account)

	require.True(t, got.GetCredentialBool("windsurf_builtin"))
	require.True(t, got.IsWindsurfBuiltinOAuth())
	require.Empty(t, got.GetCredential("access_token"))
}

func TestBuildSchedulerMetadataAccount_KeepsRPMFields(t *testing.T) {
	account := service.Account{
		ID:       7,
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeOAuth,
		Extra: map[string]any{
			"base_rpm":          15,
			"rpm_strategy":      "sticky_exempt",
			"rpm_sticky_buffer": 5,
			"unused_large_key":  "drop-me",
		},
	}

	got := buildSchedulerMetadataAccount(account)

	require.Equal(t, 15, got.GetBaseRPM())
	require.Equal(t, "sticky_exempt", got.GetRPMStrategy())
	require.Equal(t, 5, got.GetRPMStickyBuffer())
	require.Nil(t, got.Extra["unused_large_key"])
}
