package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectClientProfile(t *testing.T) {
	t.Run("codex responses", func(t *testing.T) {
		req := mustProfileRequest(t, http.MethodPost, "/v1/responses", "codex-cli/1.0")
		profile := DetectClientProfile(req, WireProtocolOpenAIResponses)
		require.Equal(t, ClientProfileCodex, profile.ID)
		require.Equal(t, WireProtocolOpenAIResponses, profile.WireProtocol)
	})

	t.Run("opencode anthropic", func(t *testing.T) {
		req := mustProfileRequest(t, http.MethodPost, "/v1/messages", "opencode/0.1")
		profile := DetectClientProfile(req, WireProtocolAnthropicMessages)
		require.Equal(t, ClientProfileOpenCodeAnthropic, profile.ID)
		require.Equal(t, WireProtocolAnthropicMessages, profile.WireProtocol)
	})

	t.Run("claude code", func(t *testing.T) {
		req := mustProfileRequest(t, http.MethodPost, "/v1/messages", "claude-code/2.0")
		profile := DetectClientProfile(req, WireProtocolAnthropicMessages)
		require.Equal(t, ClientProfileClaudeCode, profile.ID)
	})
}

func TestBuildClientCapabilitySnapshot_Windsurf(t *testing.T) {
	snapshot := BuildClientCapabilitySnapshot(PlatformWindsurf, false)

	require.Equal(t, PlatformWindsurf, snapshot.GroupPlatform)
	require.Len(t, snapshot.ClientProfiles, 4)

	var gpt55, claude47 *ModelCapability
	for i := range snapshot.Models {
		switch snapshot.Models[i].ID {
		case "gpt-5.5":
			gpt55 = &snapshot.Models[i]
		case "claude-opus-4-7-medium":
			claude47 = &snapshot.Models[i]
		}
	}

	require.NotNil(t, gpt55)
	require.Equal(t, ToolBridgePromptBridge, gpt55.ToolBridge)
	require.Contains(t, gpt55.Protocols, WireProtocolOpenAIResponses)
	require.True(t, gpt55.SupportsTools)

	require.NotNil(t, claude47)
	require.Contains(t, claude47.Protocols, WireProtocolAnthropicMessages)
	require.Equal(t, PlatformAnthropic, claude47.Provider)
}

func TestSupportedClientProfilesForOpenAIHonorsMessagesDispatch(t *testing.T) {
	withoutDispatch := SupportedClientProfilesForPlatform(PlatformOpenAI, false)
	requireProfileIDs(t, withoutDispatch, []string{ClientProfileCodex, ClientProfileOpenCodeOpenAI})

	withDispatch := SupportedClientProfilesForPlatform(PlatformOpenAI, true)
	requireProfileIDs(t, withDispatch, []string{
		ClientProfileCodex,
		ClientProfileOpenCodeOpenAI,
		ClientProfileClaudeCode,
		ClientProfileOpenCodeAnthropic,
	})
}

func mustProfileRequest(t *testing.T, method, target, userAgent string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, target, nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", userAgent)
	return req
}

func requireProfileIDs(t *testing.T, profiles []ClientProfile, want []string) {
	t.Helper()
	got := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		got = append(got, profile.ID)
	}
	require.Equal(t, want, got)
}
