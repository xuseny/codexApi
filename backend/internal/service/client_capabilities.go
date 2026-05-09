package service

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

const (
	ClientProfileCodex              = "codex"
	ClientProfileClaudeCode         = "claude-code"
	ClientProfileOpenCodeOpenAI     = "opencode-openai"
	ClientProfileOpenCodeAnthropic  = "opencode-anthropic"
	ClientProfileOpenAIChat         = "openai-chat"
	ClientProfileOpenAIResponses    = "openai-responses"
	ClientProfileAnthropicMessages  = "anthropic-messages"
	ClientProfileUnknown            = "unknown"
	WireProtocolOpenAIResponses     = "openai_responses"
	WireProtocolAnthropicMessages   = "anthropic_messages"
	WireProtocolOpenAIChat          = "openai_chat_completions"
	ToolBridgeNative                = "native"
	ToolBridgePromptBridge          = "prompt_bridge"
	ToolBridgeNone                  = "none"
	ModelCapabilitySourceConfigured = "configured"
	ModelCapabilitySourceCatalog    = "catalog"
)

// ClientProfile describes the client dialect the gateway should emulate.
// Editors execute their own local tools; this profile only declares the wire
// format the gateway must preserve or synthesize.
type ClientProfile struct {
	ID                  string   `json:"id"`
	DisplayName         string   `json:"display_name"`
	WireProtocol        string   `json:"wire_protocol"`
	Provider            string   `json:"provider"`
	SupportsReasoning   bool     `json:"supports_reasoning"`
	SupportsTools       bool     `json:"supports_tools"`
	SupportsStreaming   bool     `json:"supports_streaming"`
	RequiresClientTools bool     `json:"requires_client_tools"`
	Notes               []string `json:"notes,omitempty"`
}

type ModelCapability struct {
	ID                     string   `json:"id"`
	DisplayName            string   `json:"display_name"`
	Provider               string   `json:"provider"`
	Protocols              []string `json:"protocols"`
	SupportsReasoning      bool     `json:"supports_reasoning"`
	SupportsTools          bool     `json:"supports_tools"`
	SupportsStreaming      bool     `json:"supports_streaming"`
	SupportsAttachments    bool     `json:"supports_attachments,omitempty"`
	DefaultReasoningEffort string   `json:"default_reasoning_effort,omitempty"`
	ReasoningEfforts       []string `json:"reasoning_efforts,omitempty"`
	ToolBridge             string   `json:"tool_bridge"`
	Source                 string   `json:"source"`
}

type ClientCapabilitySnapshot struct {
	GroupPlatform         string            `json:"group_platform"`
	AllowMessagesDispatch bool              `json:"allow_messages_dispatch"`
	ClientProfiles        []ClientProfile   `json:"client_profiles"`
	Models                []ModelCapability `json:"models"`
}

func DefaultClientProfiles() []ClientProfile {
	return []ClientProfile{
		{
			ID:                  ClientProfileCodex,
			DisplayName:         "Codex",
			WireProtocol:        WireProtocolOpenAIResponses,
			Provider:            PlatformOpenAI,
			SupportsReasoning:   true,
			SupportsTools:       true,
			SupportsStreaming:   true,
			RequiresClientTools: true,
			Notes: []string{
				"Codex executes local tools; the gateway must preserve OpenAI Responses tool events.",
			},
		},
		{
			ID:                  ClientProfileClaudeCode,
			DisplayName:         "Claude Code",
			WireProtocol:        WireProtocolAnthropicMessages,
			Provider:            PlatformAnthropic,
			SupportsReasoning:   true,
			SupportsTools:       true,
			SupportsStreaming:   true,
			RequiresClientTools: true,
			Notes: []string{
				"Claude Code executes local tools from Anthropic tool_use blocks.",
			},
		},
		{
			ID:                  ClientProfileOpenCodeOpenAI,
			DisplayName:         "OpenCode (OpenAI)",
			WireProtocol:        WireProtocolOpenAIResponses,
			Provider:            PlatformOpenAI,
			SupportsReasoning:   true,
			SupportsTools:       true,
			SupportsStreaming:   true,
			RequiresClientTools: true,
		},
		{
			ID:                  ClientProfileOpenCodeAnthropic,
			DisplayName:         "OpenCode (Anthropic)",
			WireProtocol:        WireProtocolAnthropicMessages,
			Provider:            PlatformAnthropic,
			SupportsReasoning:   true,
			SupportsTools:       true,
			SupportsStreaming:   true,
			RequiresClientTools: true,
		},
	}
}

func DetectClientProfile(r *http.Request, fallbackWireProtocol string) ClientProfile {
	profileID := ClientProfileUnknown
	path := ""
	ua := ""
	if r != nil {
		if r.URL != nil {
			path = strings.ToLower(strings.TrimSpace(r.URL.Path))
		}
		ua = strings.ToLower(strings.TrimSpace(r.UserAgent()))
	}

	switch {
	case strings.Contains(ua, "opencode") && strings.Contains(path, "/messages"):
		profileID = ClientProfileOpenCodeAnthropic
	case strings.Contains(ua, "opencode") && (strings.Contains(path, "/responses") || strings.Contains(path, "/chat/completions")):
		profileID = ClientProfileOpenCodeOpenAI
	case strings.Contains(ua, "claude") || strings.Contains(path, "/messages"):
		profileID = ClientProfileClaudeCode
	case strings.Contains(ua, "codex") || strings.Contains(path, "/backend-api/codex") || strings.Contains(path, "/responses"):
		profileID = ClientProfileCodex
	case strings.Contains(path, "/chat/completions"):
		profileID = ClientProfileOpenAIChat
	default:
		switch fallbackWireProtocol {
		case WireProtocolOpenAIResponses:
			profileID = ClientProfileOpenAIResponses
		case WireProtocolAnthropicMessages:
			profileID = ClientProfileAnthropicMessages
		case WireProtocolOpenAIChat:
			profileID = ClientProfileOpenAIChat
		}
	}

	if profile, ok := ClientProfileByID(profileID); ok {
		return profile
	}
	return ClientProfile{
		ID:                profileID,
		DisplayName:       profileID,
		WireProtocol:      fallbackWireProtocol,
		SupportsStreaming: true,
	}
}

func ClientProfileByID(id string) (ClientProfile, bool) {
	for _, profile := range DefaultClientProfiles() {
		if profile.ID == id {
			return profile, true
		}
	}
	switch id {
	case ClientProfileOpenAIResponses:
		return ClientProfile{
			ID:                ClientProfileOpenAIResponses,
			DisplayName:       "OpenAI Responses",
			WireProtocol:      WireProtocolOpenAIResponses,
			Provider:          PlatformOpenAI,
			SupportsReasoning: true,
			SupportsTools:     true,
			SupportsStreaming: true,
		}, true
	case ClientProfileAnthropicMessages:
		return ClientProfile{
			ID:                ClientProfileAnthropicMessages,
			DisplayName:       "Anthropic Messages",
			WireProtocol:      WireProtocolAnthropicMessages,
			Provider:          PlatformAnthropic,
			SupportsReasoning: true,
			SupportsTools:     true,
			SupportsStreaming: true,
		}, true
	case ClientProfileOpenAIChat:
		return ClientProfile{
			ID:                ClientProfileOpenAIChat,
			DisplayName:       "OpenAI Chat Completions",
			WireProtocol:      WireProtocolOpenAIChat,
			Provider:          PlatformOpenAI,
			SupportsTools:     true,
			SupportsStreaming: true,
		}, true
	default:
		return ClientProfile{}, false
	}
}

func BuildClientCapabilitySnapshot(platform string, allowMessagesDispatch bool) ClientCapabilitySnapshot {
	platform = strings.TrimSpace(platform)
	return ClientCapabilitySnapshot{
		GroupPlatform:         platform,
		AllowMessagesDispatch: allowMessagesDispatch,
		ClientProfiles:        SupportedClientProfilesForPlatform(platform, allowMessagesDispatch),
		Models:                ModelCapabilitiesForPlatform(platform, allowMessagesDispatch),
	}
}

func SupportedClientProfilesForPlatform(platform string, allowMessagesDispatch bool) []ClientProfile {
	all := map[string]ClientProfile{}
	for _, profile := range DefaultClientProfiles() {
		all[profile.ID] = profile
	}
	pick := func(ids ...string) []ClientProfile {
		out := make([]ClientProfile, 0, len(ids))
		for _, id := range ids {
			if profile, ok := all[id]; ok {
				out = append(out, profile)
			}
		}
		return out
	}

	switch platform {
	case PlatformOpenAI:
		ids := []string{ClientProfileCodex, ClientProfileOpenCodeOpenAI}
		if allowMessagesDispatch {
			ids = append(ids, ClientProfileClaudeCode, ClientProfileOpenCodeAnthropic)
		}
		return pick(ids...)
	case PlatformWindsurf:
		return pick(ClientProfileCodex, ClientProfileClaudeCode, ClientProfileOpenCodeOpenAI, ClientProfileOpenCodeAnthropic)
	case PlatformKiro, PlatformAnthropic:
		return pick(ClientProfileClaudeCode, ClientProfileOpenCodeAnthropic)
	case PlatformGemini:
		return nil
	default:
		return pick(ClientProfileCodex, ClientProfileClaudeCode, ClientProfileOpenCodeOpenAI, ClientProfileOpenCodeAnthropic)
	}
}

func ModelCapabilitiesForPlatform(platform string, allowMessagesDispatch bool) []ModelCapability {
	switch platform {
	case PlatformWindsurf:
		return windsurfModelCapabilities()
	case PlatformAnthropic, PlatformKiro:
		return anthropicModelCapabilities()
	case PlatformOpenAI:
		caps := openAIModelCapabilities()
		if allowMessagesDispatch {
			caps = append(caps, anthropicModelCapabilities()...)
		}
		return caps
	default:
		return openAIModelCapabilities()
	}
}

func openAIModelCapabilities() []ModelCapability {
	out := make([]ModelCapability, 0, len(openai.DefaultModels))
	for _, model := range openai.DefaultModels {
		out = append(out, ModelCapability{
			ID:                     model.ID,
			DisplayName:            firstNonEmpty(model.DisplayName, model.ID),
			Provider:               PlatformOpenAI,
			Protocols:              []string{WireProtocolOpenAIResponses, WireProtocolOpenAIChat},
			SupportsReasoning:      openAIModelSupportsReasoning(model.ID),
			SupportsTools:          true,
			SupportsStreaming:      true,
			SupportsAttachments:    !strings.HasPrefix(strings.ToLower(model.ID), "gpt-image-"),
			DefaultReasoningEffort: defaultReasoningEffortForModel(model.ID),
			ReasoningEfforts:       reasoningEffortsForModel(model.ID),
			ToolBridge:             ToolBridgeNative,
			Source:                 ModelCapabilitySourceConfigured,
		})
	}
	return out
}

func anthropicModelCapabilities() []ModelCapability {
	out := make([]ModelCapability, 0, len(claude.DefaultModels))
	for _, model := range claude.DefaultModels {
		out = append(out, ModelCapability{
			ID:                     model.ID,
			DisplayName:            firstNonEmpty(model.DisplayName, model.ID),
			Provider:               PlatformAnthropic,
			Protocols:              []string{WireProtocolAnthropicMessages},
			SupportsReasoning:      true,
			SupportsTools:          true,
			SupportsStreaming:      true,
			SupportsAttachments:    true,
			DefaultReasoningEffort: "high",
			ReasoningEfforts:       []string{"low", "medium", "high", "xhigh"},
			ToolBridge:             ToolBridgeNative,
			Source:                 ModelCapabilitySourceConfigured,
		})
	}
	return out
}

func windsurfModelCapabilities() []ModelCapability {
	out := make([]ModelCapability, 0, len(windsurfModelCatalog))
	for _, entry := range windsurfModelCatalog {
		if entry.Deprecated {
			continue
		}
		provider := strings.ToLower(strings.TrimSpace(entry.OwnedBy))
		protocols := []string{WireProtocolOpenAIResponses, WireProtocolOpenAIChat}
		if provider == PlatformAnthropic || strings.HasPrefix(strings.ToLower(entry.ID), "claude") {
			protocols = []string{WireProtocolOpenAIResponses, WireProtocolAnthropicMessages, WireProtocolOpenAIChat}
		}
		out = append(out, ModelCapability{
			ID:                     entry.ID,
			DisplayName:            firstNonEmpty(entry.DisplayName, entry.ID),
			Provider:               firstNonEmpty(provider, PlatformWindsurf),
			Protocols:              protocols,
			SupportsReasoning:      windsurfModelSupportsReasoning(entry.ID),
			SupportsTools:          true,
			SupportsStreaming:      true,
			SupportsAttachments:    true,
			DefaultReasoningEffort: defaultReasoningEffortForModel(entry.ID),
			ReasoningEfforts:       reasoningEffortsForModel(entry.ID),
			ToolBridge:             ToolBridgePromptBridge,
			Source:                 ModelCapabilitySourceCatalog,
		})
	}
	return out
}

func openAIModelSupportsReasoning(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(normalized, "gpt-5") ||
		strings.HasPrefix(normalized, "o3") ||
		strings.HasPrefix(normalized, "o4") ||
		strings.Contains(normalized, "codex")
}

func windsurfModelSupportsReasoning(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(normalized, "thinking") ||
		strings.HasPrefix(normalized, "gpt-5") ||
		strings.HasPrefix(normalized, "o3") ||
		strings.HasPrefix(normalized, "o4") ||
		strings.Contains(normalized, "opus-4-7") ||
		strings.Contains(normalized, "sonnet-4.6")
}

func defaultReasoningEffortForModel(model string) string {
	if openAIModelSupportsReasoning(model) || windsurfModelSupportsReasoning(model) {
		return "high"
	}
	return ""
}

func reasoningEffortsForModel(model string) []string {
	if defaultReasoningEffortForModel(model) == "" {
		return nil
	}
	return []string{"low", "medium", "high", "xhigh"}
}
