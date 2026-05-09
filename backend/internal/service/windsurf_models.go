package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

const (
	windsurfDefaultTestModel = "gemini-2.5-flash"
	windsurfModelCreatedAt   = 1777593600
)

type windsurfCatalogEntry struct {
	ID          string
	DisplayName string
	OwnedBy     string
	Name        string
	EnumValue   int
	ModelUID    string
	Deprecated  bool
}

var windsurfModelCatalog = []windsurfCatalogEntry{
	{ID: "claude-4-sonnet", DisplayName: "claude-4-sonnet", OwnedBy: "anthropic", Name: "claude-4-sonnet", EnumValue: 281, ModelUID: "MODEL_CLAUDE_4_SONNET"},
	{ID: "claude-4-sonnet-thinking", DisplayName: "claude-4-sonnet-thinking", OwnedBy: "anthropic", Name: "claude-4-sonnet-thinking", EnumValue: 282, ModelUID: "MODEL_CLAUDE_4_SONNET_THINKING"},
	{ID: "claude-4-opus", DisplayName: "claude-4-opus", OwnedBy: "anthropic", Name: "claude-4-opus", EnumValue: 290, ModelUID: "MODEL_CLAUDE_4_OPUS"},
	{ID: "claude-4-opus-thinking", DisplayName: "claude-4-opus-thinking", OwnedBy: "anthropic", Name: "claude-4-opus-thinking", EnumValue: 291, ModelUID: "MODEL_CLAUDE_4_OPUS_THINKING"},
	{ID: "claude-4.1-opus", DisplayName: "claude-4.1-opus", OwnedBy: "anthropic", Name: "claude-4.1-opus", EnumValue: 328, ModelUID: "MODEL_CLAUDE_4_1_OPUS"},
	{ID: "claude-4.1-opus-thinking", DisplayName: "claude-4.1-opus-thinking", OwnedBy: "anthropic", Name: "claude-4.1-opus-thinking", EnumValue: 329, ModelUID: "MODEL_CLAUDE_4_1_OPUS_THINKING"},
	{ID: "claude-4.5-haiku", DisplayName: "claude-4.5-haiku", OwnedBy: "anthropic", Name: "claude-4.5-haiku", ModelUID: "MODEL_PRIVATE_11"},
	{ID: "claude-4.5-sonnet", DisplayName: "claude-4.5-sonnet", OwnedBy: "anthropic", Name: "claude-4.5-sonnet", EnumValue: 353, ModelUID: "MODEL_PRIVATE_2"},
	{ID: "claude-4.5-sonnet-thinking", DisplayName: "claude-4.5-sonnet-thinking", OwnedBy: "anthropic", Name: "claude-4.5-sonnet-thinking", EnumValue: 354, ModelUID: "MODEL_PRIVATE_3"},
	{ID: "claude-4.5-opus", DisplayName: "claude-4.5-opus", OwnedBy: "anthropic", Name: "claude-4.5-opus", EnumValue: 391, ModelUID: "MODEL_CLAUDE_4_5_OPUS"},
	{ID: "claude-4.5-opus-thinking", DisplayName: "claude-4.5-opus-thinking", OwnedBy: "anthropic", Name: "claude-4.5-opus-thinking", EnumValue: 392, ModelUID: "MODEL_CLAUDE_4_5_OPUS_THINKING"},
	{ID: "claude-sonnet-4.6", DisplayName: "claude-sonnet-4.6", OwnedBy: "anthropic", Name: "claude-sonnet-4.6", ModelUID: "claude-sonnet-4-6"},
	{ID: "claude-sonnet-4.6-thinking", DisplayName: "claude-sonnet-4.6-thinking", OwnedBy: "anthropic", Name: "claude-sonnet-4.6-thinking", ModelUID: "claude-sonnet-4-6-thinking"},
	{ID: "claude-sonnet-4.6-1m", DisplayName: "claude-sonnet-4.6-1m", OwnedBy: "anthropic", Name: "claude-sonnet-4.6-1m", ModelUID: "claude-sonnet-4-6-1m"},
	{ID: "claude-sonnet-4.6-thinking-1m", DisplayName: "claude-sonnet-4.6-thinking-1m", OwnedBy: "anthropic", Name: "claude-sonnet-4.6-thinking-1m", ModelUID: "claude-sonnet-4-6-thinking-1m"},
	{ID: "claude-opus-4.6", DisplayName: "claude-opus-4.6", OwnedBy: "anthropic", Name: "claude-opus-4.6", ModelUID: "claude-opus-4-6"},
	{ID: "claude-opus-4.6-thinking", DisplayName: "claude-opus-4.6-thinking", OwnedBy: "anthropic", Name: "claude-opus-4.6-thinking", ModelUID: "claude-opus-4-6-thinking"},
	{ID: "claude-opus-4-7-medium", DisplayName: "claude-opus-4-7-medium", OwnedBy: "anthropic", Name: "claude-opus-4-7-medium", ModelUID: "claude-opus-4-7-medium"},
	{ID: "claude-opus-4-7-low", DisplayName: "claude-opus-4-7-low", OwnedBy: "anthropic", Name: "claude-opus-4-7-low", ModelUID: "claude-opus-4-7-low"},
	{ID: "claude-opus-4-7-high", DisplayName: "claude-opus-4-7-high", OwnedBy: "anthropic", Name: "claude-opus-4-7-high", ModelUID: "claude-opus-4-7-high"},
	{ID: "claude-opus-4-7-xhigh", DisplayName: "claude-opus-4-7-xhigh", OwnedBy: "anthropic", Name: "claude-opus-4-7-xhigh", ModelUID: "claude-opus-4-7-xhigh"},
	{ID: "claude-opus-4-7-medium-thinking", DisplayName: "claude-opus-4-7-medium-thinking", OwnedBy: "anthropic", Name: "claude-opus-4-7-medium-thinking", ModelUID: "claude-opus-4-7-medium-thinking"},
	{ID: "claude-opus-4-7-high-thinking", DisplayName: "claude-opus-4-7-high-thinking", OwnedBy: "anthropic", Name: "claude-opus-4-7-high-thinking", ModelUID: "claude-opus-4-7-high-thinking"},
	{ID: "claude-opus-4-7-xhigh-thinking", DisplayName: "claude-opus-4-7-xhigh-thinking", OwnedBy: "anthropic", Name: "claude-opus-4-7-xhigh-thinking", ModelUID: "claude-opus-4-7-xhigh-thinking"},
	{ID: "claude-opus-4-7-max", DisplayName: "claude-opus-4-7-max", OwnedBy: "anthropic", Name: "claude-opus-4-7-max", ModelUID: "claude-opus-4-7-max"},
	{ID: "gpt-4o", DisplayName: "gpt-4o", OwnedBy: "openai", Name: "gpt-4o", EnumValue: 109, ModelUID: "MODEL_CHAT_GPT_4O_2024_08_06"},
	{ID: "gpt-4.1", DisplayName: "gpt-4.1", OwnedBy: "openai", Name: "gpt-4.1", EnumValue: 259, ModelUID: "MODEL_CHAT_GPT_4_1_2025_04_14"},
	{ID: "gpt-5", DisplayName: "gpt-5", OwnedBy: "openai", Name: "gpt-5", EnumValue: 340, ModelUID: "MODEL_PRIVATE_6"},
	{ID: "gpt-5-medium", DisplayName: "gpt-5-medium", OwnedBy: "openai", Name: "gpt-5-medium", ModelUID: "MODEL_PRIVATE_7"},
	{ID: "gpt-5-high", DisplayName: "gpt-5-high", OwnedBy: "openai", Name: "gpt-5-high", ModelUID: "MODEL_PRIVATE_8"},
	{ID: "gpt-5-codex", DisplayName: "gpt-5-codex", OwnedBy: "openai", Name: "gpt-5-codex", EnumValue: 346, ModelUID: "MODEL_CHAT_GPT_5_CODEX"},
	{ID: "gpt-5.1", DisplayName: "gpt-5.1", OwnedBy: "openai", Name: "gpt-5.1", ModelUID: "MODEL_PRIVATE_12"},
	{ID: "gpt-5.1-low", DisplayName: "gpt-5.1-low", OwnedBy: "openai", Name: "gpt-5.1-low", ModelUID: "MODEL_PRIVATE_13"},
	{ID: "gpt-5.1-medium", DisplayName: "gpt-5.1-medium", OwnedBy: "openai", Name: "gpt-5.1-medium", ModelUID: "MODEL_PRIVATE_14"},
	{ID: "gpt-5.1-high", DisplayName: "gpt-5.1-high", OwnedBy: "openai", Name: "gpt-5.1-high", ModelUID: "MODEL_PRIVATE_15"},
	{ID: "gpt-5.1-fast", DisplayName: "gpt-5.1-fast", OwnedBy: "openai", Name: "gpt-5.1-fast", ModelUID: "MODEL_PRIVATE_20"},
	{ID: "gpt-5.1-low-fast", DisplayName: "gpt-5.1-low-fast", OwnedBy: "openai", Name: "gpt-5.1-low-fast", ModelUID: "MODEL_PRIVATE_21"},
	{ID: "gpt-5.1-medium-fast", DisplayName: "gpt-5.1-medium-fast", OwnedBy: "openai", Name: "gpt-5.1-medium-fast", ModelUID: "MODEL_PRIVATE_22"},
	{ID: "gpt-5.1-high-fast", DisplayName: "gpt-5.1-high-fast", OwnedBy: "openai", Name: "gpt-5.1-high-fast", ModelUID: "MODEL_PRIVATE_23"},
	{ID: "gpt-5.1-codex-low", DisplayName: "gpt-5.1-codex-low", OwnedBy: "openai", Name: "gpt-5.1-codex-low", ModelUID: "MODEL_GPT_5_1_CODEX_LOW"},
	{ID: "gpt-5.1-codex-medium", DisplayName: "gpt-5.1-codex-medium", OwnedBy: "openai", Name: "gpt-5.1-codex-medium", ModelUID: "MODEL_PRIVATE_9"},
	{ID: "gpt-5.1-codex-mini-low", DisplayName: "gpt-5.1-codex-mini-low", OwnedBy: "openai", Name: "gpt-5.1-codex-mini-low", ModelUID: "MODEL_GPT_5_1_CODEX_MINI_LOW"},
	{ID: "gpt-5.1-codex-mini", DisplayName: "gpt-5.1-codex-mini", OwnedBy: "openai", Name: "gpt-5.1-codex-mini", ModelUID: "MODEL_PRIVATE_19"},
	{ID: "gpt-5.1-codex-max-low", DisplayName: "gpt-5.1-codex-max-low", OwnedBy: "openai", Name: "gpt-5.1-codex-max-low", ModelUID: "MODEL_GPT_5_1_CODEX_MAX_LOW"},
	{ID: "gpt-5.1-codex-max-medium", DisplayName: "gpt-5.1-codex-max-medium", OwnedBy: "openai", Name: "gpt-5.1-codex-max-medium", ModelUID: "MODEL_GPT_5_1_CODEX_MAX_MEDIUM"},
	{ID: "gpt-5.1-codex-max-high", DisplayName: "gpt-5.1-codex-max-high", OwnedBy: "openai", Name: "gpt-5.1-codex-max-high", ModelUID: "MODEL_GPT_5_1_CODEX_MAX_HIGH"},
	{ID: "gpt-5.2", DisplayName: "gpt-5.2", OwnedBy: "openai", Name: "gpt-5.2", EnumValue: 401, ModelUID: "MODEL_GPT_5_2_MEDIUM"},
	{ID: "gpt-5.2-none", DisplayName: "gpt-5.2-none", OwnedBy: "openai", Name: "gpt-5.2-none", ModelUID: "MODEL_GPT_5_2_NONE"},
	{ID: "gpt-5.2-low", DisplayName: "gpt-5.2-low", OwnedBy: "openai", Name: "gpt-5.2-low", EnumValue: 400, ModelUID: "MODEL_GPT_5_2_LOW"},
	{ID: "gpt-5.2-high", DisplayName: "gpt-5.2-high", OwnedBy: "openai", Name: "gpt-5.2-high", EnumValue: 402, ModelUID: "MODEL_GPT_5_2_HIGH"},
	{ID: "gpt-5.2-xhigh", DisplayName: "gpt-5.2-xhigh", OwnedBy: "openai", Name: "gpt-5.2-xhigh", EnumValue: 403, ModelUID: "MODEL_GPT_5_2_XHIGH"},
	{ID: "gpt-5.2-none-fast", DisplayName: "gpt-5.2-none-fast", OwnedBy: "openai", Name: "gpt-5.2-none-fast", ModelUID: "MODEL_GPT_5_2_NONE_PRIORITY"},
	{ID: "gpt-5.2-low-fast", DisplayName: "gpt-5.2-low-fast", OwnedBy: "openai", Name: "gpt-5.2-low-fast", ModelUID: "MODEL_GPT_5_2_LOW_PRIORITY"},
	{ID: "gpt-5.2-medium-fast", DisplayName: "gpt-5.2-medium-fast", OwnedBy: "openai", Name: "gpt-5.2-medium-fast", ModelUID: "MODEL_GPT_5_2_MEDIUM_PRIORITY"},
	{ID: "gpt-5.2-high-fast", DisplayName: "gpt-5.2-high-fast", OwnedBy: "openai", Name: "gpt-5.2-high-fast", ModelUID: "MODEL_GPT_5_2_HIGH_PRIORITY"},
	{ID: "gpt-5.2-xhigh-fast", DisplayName: "gpt-5.2-xhigh-fast", OwnedBy: "openai", Name: "gpt-5.2-xhigh-fast", ModelUID: "MODEL_GPT_5_2_XHIGH_PRIORITY"},
	{ID: "gpt-5.2-codex-low", DisplayName: "gpt-5.2-codex-low", OwnedBy: "openai", Name: "gpt-5.2-codex-low", ModelUID: "MODEL_GPT_5_2_CODEX_LOW"},
	{ID: "gpt-5.2-codex-medium", DisplayName: "gpt-5.2-codex-medium", OwnedBy: "openai", Name: "gpt-5.2-codex-medium", ModelUID: "MODEL_GPT_5_2_CODEX_MEDIUM"},
	{ID: "gpt-5.2-codex-high", DisplayName: "gpt-5.2-codex-high", OwnedBy: "openai", Name: "gpt-5.2-codex-high", ModelUID: "MODEL_GPT_5_2_CODEX_HIGH"},
	{ID: "gpt-5.2-codex-xhigh", DisplayName: "gpt-5.2-codex-xhigh", OwnedBy: "openai", Name: "gpt-5.2-codex-xhigh", ModelUID: "MODEL_GPT_5_2_CODEX_XHIGH"},
	{ID: "gpt-5.2-codex-low-fast", DisplayName: "gpt-5.2-codex-low-fast", OwnedBy: "openai", Name: "gpt-5.2-codex-low-fast", ModelUID: "MODEL_GPT_5_2_CODEX_LOW_PRIORITY"},
	{ID: "gpt-5.2-codex-medium-fast", DisplayName: "gpt-5.2-codex-medium-fast", OwnedBy: "openai", Name: "gpt-5.2-codex-medium-fast", ModelUID: "MODEL_GPT_5_2_CODEX_MEDIUM_PRIORITY"},
	{ID: "gpt-5.2-codex-high-fast", DisplayName: "gpt-5.2-codex-high-fast", OwnedBy: "openai", Name: "gpt-5.2-codex-high-fast", ModelUID: "MODEL_GPT_5_2_CODEX_HIGH_PRIORITY"},
	{ID: "gpt-5.2-codex-xhigh-fast", DisplayName: "gpt-5.2-codex-xhigh-fast", OwnedBy: "openai", Name: "gpt-5.2-codex-xhigh-fast", ModelUID: "MODEL_GPT_5_2_CODEX_XHIGH_PRIORITY"},
	{ID: "gpt-5.3-codex", DisplayName: "gpt-5.3-codex", OwnedBy: "openai", Name: "gpt-5.3-codex", ModelUID: "gpt-5-3-codex-medium"},
	{ID: "gpt-5.4-none", DisplayName: "gpt-5.4-none", OwnedBy: "openai", Name: "gpt-5.4-none", ModelUID: "gpt-5-4-none"},
	{ID: "gpt-5.4-low", DisplayName: "gpt-5.4-low", OwnedBy: "openai", Name: "gpt-5.4-low", ModelUID: "gpt-5-4-low"},
	{ID: "gpt-5.4-medium", DisplayName: "gpt-5.4-medium", OwnedBy: "openai", Name: "gpt-5.4-medium", ModelUID: "gpt-5-4-medium"},
	{ID: "gpt-5.4-high", DisplayName: "gpt-5.4-high", OwnedBy: "openai", Name: "gpt-5.4-high", ModelUID: "gpt-5-4-high"},
	{ID: "gpt-5.4-xhigh", DisplayName: "gpt-5.4-xhigh", OwnedBy: "openai", Name: "gpt-5.4-xhigh", ModelUID: "gpt-5-4-xhigh"},
	{ID: "gpt-5.4-mini-low", DisplayName: "gpt-5.4-mini-low", OwnedBy: "openai", Name: "gpt-5.4-mini-low", ModelUID: "gpt-5-4-mini-low"},
	{ID: "gpt-5.4-mini-medium", DisplayName: "gpt-5.4-mini-medium", OwnedBy: "openai", Name: "gpt-5.4-mini-medium", ModelUID: "gpt-5-4-mini-medium"},
	{ID: "gpt-5.4-mini-high", DisplayName: "gpt-5.4-mini-high", OwnedBy: "openai", Name: "gpt-5.4-mini-high", ModelUID: "gpt-5-4-mini-high"},
	{ID: "gpt-5.4-mini-xhigh", DisplayName: "gpt-5.4-mini-xhigh", OwnedBy: "openai", Name: "gpt-5.4-mini-xhigh", ModelUID: "gpt-5-4-mini-xhigh"},
	{ID: "gpt-5.5", DisplayName: "gpt-5.5", OwnedBy: "openai", Name: "gpt-5.5", ModelUID: "gpt-5-5-medium"},
	{ID: "gpt-5.5-none", DisplayName: "gpt-5.5-none", OwnedBy: "openai", Name: "gpt-5.5-none", ModelUID: "gpt-5-5-none"},
	{ID: "gpt-5.5-low", DisplayName: "gpt-5.5-low", OwnedBy: "openai", Name: "gpt-5.5-low", ModelUID: "gpt-5-5-low"},
	{ID: "gpt-5.5-medium", DisplayName: "gpt-5.5-medium", OwnedBy: "openai", Name: "gpt-5.5-medium", ModelUID: "gpt-5-5-medium"},
	{ID: "gpt-5.5-high", DisplayName: "gpt-5.5-high", OwnedBy: "openai", Name: "gpt-5.5-high", ModelUID: "gpt-5-5-high"},
	{ID: "gpt-5.5-xhigh", DisplayName: "gpt-5.5-xhigh", OwnedBy: "openai", Name: "gpt-5.5-xhigh", ModelUID: "gpt-5-5-xhigh"},
	{ID: "gpt-5.5-none-fast", DisplayName: "gpt-5.5-none-fast", OwnedBy: "openai", Name: "gpt-5.5-none-fast", ModelUID: "gpt-5-5-none-priority"},
	{ID: "gpt-5.5-low-fast", DisplayName: "gpt-5.5-low-fast", OwnedBy: "openai", Name: "gpt-5.5-low-fast", ModelUID: "gpt-5-5-low-priority"},
	{ID: "gpt-5.5-medium-fast", DisplayName: "gpt-5.5-medium-fast", OwnedBy: "openai", Name: "gpt-5.5-medium-fast", ModelUID: "gpt-5-5-medium-priority"},
	{ID: "gpt-5.5-high-fast", DisplayName: "gpt-5.5-high-fast", OwnedBy: "openai", Name: "gpt-5.5-high-fast", ModelUID: "gpt-5-5-high-priority"},
	{ID: "gpt-5.5-xhigh-fast", DisplayName: "gpt-5.5-xhigh-fast", OwnedBy: "openai", Name: "gpt-5.5-xhigh-fast", ModelUID: "gpt-5-5-xhigh-priority"},
	{ID: "gpt-5.3-codex-low", DisplayName: "gpt-5.3-codex-low", OwnedBy: "openai", Name: "gpt-5.3-codex-low", ModelUID: "gpt-5-3-codex-low"},
	{ID: "gpt-5.3-codex-high", DisplayName: "gpt-5.3-codex-high", OwnedBy: "openai", Name: "gpt-5.3-codex-high", ModelUID: "gpt-5-3-codex-high"},
	{ID: "gpt-5.3-codex-xhigh", DisplayName: "gpt-5.3-codex-xhigh", OwnedBy: "openai", Name: "gpt-5.3-codex-xhigh", ModelUID: "gpt-5-3-codex-xhigh"},
	{ID: "gpt-5.3-codex-low-fast", DisplayName: "gpt-5.3-codex-low-fast", OwnedBy: "openai", Name: "gpt-5.3-codex-low-fast", ModelUID: "gpt-5-3-codex-low-priority"},
	{ID: "gpt-5.3-codex-medium-fast", DisplayName: "gpt-5.3-codex-medium-fast", OwnedBy: "openai", Name: "gpt-5.3-codex-medium-fast", ModelUID: "gpt-5-3-codex-medium-priority"},
	{ID: "gpt-5.3-codex-high-fast", DisplayName: "gpt-5.3-codex-high-fast", OwnedBy: "openai", Name: "gpt-5.3-codex-high-fast", ModelUID: "gpt-5-3-codex-high-priority"},
	{ID: "gpt-5.3-codex-xhigh-fast", DisplayName: "gpt-5.3-codex-xhigh-fast", OwnedBy: "openai", Name: "gpt-5.3-codex-xhigh-fast", ModelUID: "gpt-5-3-codex-xhigh-priority"},
	{ID: "gpt-oss-120b", DisplayName: "gpt-oss-120b", OwnedBy: "openai", Name: "gpt-oss-120b", ModelUID: "MODEL_GPT_OSS_120B"},
	{ID: "o3-mini", DisplayName: "o3-mini", OwnedBy: "openai", Name: "o3-mini", EnumValue: 207},
	{ID: "o3", DisplayName: "o3", OwnedBy: "openai", Name: "o3", EnumValue: 218, ModelUID: "MODEL_CHAT_O3"},
	{ID: "o3-high", DisplayName: "o3-high", OwnedBy: "openai", Name: "o3-high", ModelUID: "MODEL_CHAT_O3_HIGH"},
	{ID: "o3-pro", DisplayName: "o3-pro", OwnedBy: "openai", Name: "o3-pro", EnumValue: 294},
	{ID: "o4-mini", DisplayName: "o4-mini", OwnedBy: "openai", Name: "o4-mini", EnumValue: 264},
	{ID: "gemini-2.5-pro", DisplayName: "gemini-2.5-pro", OwnedBy: "google", Name: "gemini-2.5-pro", EnumValue: 246, ModelUID: "MODEL_GOOGLE_GEMINI_2_5_PRO"},
	{ID: "gemini-2.5-flash", DisplayName: "gemini-2.5-flash", OwnedBy: "google", Name: "gemini-2.5-flash", EnumValue: 312, ModelUID: "MODEL_GOOGLE_GEMINI_2_5_FLASH"},
	{ID: "gemini-3.0-pro", DisplayName: "gemini-3.0-pro", OwnedBy: "google", Name: "gemini-3.0-pro", EnumValue: 412, ModelUID: "MODEL_GOOGLE_GEMINI_3_0_PRO_LOW"},
	{ID: "gemini-3.0-flash-minimal", DisplayName: "gemini-3.0-flash-minimal", OwnedBy: "google", Name: "gemini-3.0-flash-minimal", ModelUID: "MODEL_GOOGLE_GEMINI_3_0_FLASH_MINIMAL"},
	{ID: "gemini-3.0-flash-low", DisplayName: "gemini-3.0-flash-low", OwnedBy: "google", Name: "gemini-3.0-flash-low", ModelUID: "MODEL_GOOGLE_GEMINI_3_0_FLASH_LOW"},
	{ID: "gemini-3.0-flash", DisplayName: "gemini-3.0-flash", OwnedBy: "google", Name: "gemini-3.0-flash", EnumValue: 415, ModelUID: "MODEL_GOOGLE_GEMINI_3_0_FLASH_MEDIUM"},
	{ID: "gemini-3.0-flash-high", DisplayName: "gemini-3.0-flash-high", OwnedBy: "google", Name: "gemini-3.0-flash-high", ModelUID: "MODEL_GOOGLE_GEMINI_3_0_FLASH_HIGH"},
	{ID: "gemini-3.1-pro-low", DisplayName: "gemini-3.1-pro-low", OwnedBy: "google", Name: "gemini-3.1-pro-low", ModelUID: "gemini-3-1-pro-low"},
	{ID: "gemini-3.1-pro-high", DisplayName: "gemini-3.1-pro-high", OwnedBy: "google", Name: "gemini-3.1-pro-high", ModelUID: "gemini-3-1-pro-high"},
	{ID: "grok-3", DisplayName: "grok-3", OwnedBy: "xai", Name: "grok-3", EnumValue: 217, ModelUID: "MODEL_XAI_GROK_3"},
	{ID: "grok-3-mini-thinking", DisplayName: "grok-3-mini-thinking", OwnedBy: "xai", Name: "grok-3-mini-thinking", ModelUID: "MODEL_XAI_GROK_3_MINI_REASONING"},
	{ID: "grok-code-fast-1", DisplayName: "grok-code-fast-1", OwnedBy: "xai", Name: "grok-code-fast-1", ModelUID: "MODEL_PRIVATE_4"},
	{ID: "kimi-k2", DisplayName: "kimi-k2", OwnedBy: "moonshot", Name: "kimi-k2", EnumValue: 323, ModelUID: "MODEL_KIMI_K2"},
	{ID: "kimi-k2-thinking", DisplayName: "kimi-k2-thinking", OwnedBy: "moonshot", Name: "kimi-k2-thinking", EnumValue: 394, ModelUID: "MODEL_KIMI_K2_THINKING"},
	{ID: "kimi-k2.5", DisplayName: "kimi-k2.5", OwnedBy: "moonshot", Name: "kimi-k2.5", ModelUID: "kimi-k2-5"},
	{ID: "kimi-k2-6", DisplayName: "kimi-k2-6", OwnedBy: "moonshot", Name: "kimi-k2-6", ModelUID: "kimi-k2-6"},
	{ID: "glm-4.7", DisplayName: "glm-4.7", OwnedBy: "zhipu", Name: "glm-4.7", EnumValue: 417, ModelUID: "MODEL_GLM_4_7"},
	{ID: "glm-4.7-fast", DisplayName: "glm-4.7-fast", OwnedBy: "zhipu", Name: "glm-4.7-fast", EnumValue: 418, ModelUID: "MODEL_GLM_4_7_FAST"},
	{ID: "glm-5", DisplayName: "glm-5", OwnedBy: "zhipu", Name: "glm-5", ModelUID: "glm-5"},
	{ID: "glm-5.1", DisplayName: "glm-5.1", OwnedBy: "zhipu", Name: "glm-5.1", ModelUID: "glm-5-1"},
	{ID: "minimax-m2.5", DisplayName: "minimax-m2.5", OwnedBy: "minimax", Name: "minimax-m2.5", EnumValue: 419, ModelUID: "MODEL_MINIMAX_M2_1"},
	{ID: "swe-1.5", DisplayName: "swe-1.5", OwnedBy: "windsurf", Name: "swe-1.5", EnumValue: 377, ModelUID: "MODEL_SWE_1_5_SLOW"},
	{ID: "swe-1.5-fast", DisplayName: "swe-1.5-fast", OwnedBy: "windsurf", Name: "swe-1.5-fast", EnumValue: 359, ModelUID: "MODEL_SWE_1_5"},
	{ID: "swe-1.5-thinking", DisplayName: "swe-1.5-thinking", OwnedBy: "windsurf", Name: "swe-1.5-thinking", EnumValue: 369, ModelUID: "MODEL_SWE_1_5_THINKING"},
	{ID: "swe-1.6", DisplayName: "swe-1.6", OwnedBy: "windsurf", Name: "swe-1.6", EnumValue: 420, ModelUID: "MODEL_SWE_1_6"},
	{ID: "swe-1.6-fast", DisplayName: "swe-1.6-fast", OwnedBy: "windsurf", Name: "swe-1.6-fast", EnumValue: 421, ModelUID: "MODEL_SWE_1_6_FAST"},
}

var windsurfModelAliases = map[string]string{
	"claude-sonnet-4-5":          "claude-4.5-sonnet",
	"claude-sonnet-4-5-latest":   "claude-4.5-sonnet",
	"claude-sonnet-4-5-20250929": "claude-4.5-sonnet",
	"claude-sonnet-4.5":          "claude-4.5-sonnet",
	"claude-sonnet-4.5-thinking": "claude-4.5-sonnet-thinking",
	"claude-haiku-4-5":           "claude-4.5-haiku",
	"claude-haiku-4.5":           "claude-4.5-haiku",
	"claude-sonnet-4-6":          "claude-sonnet-4.6",
	"claude-sonnet-4-6-thinking": "claude-sonnet-4.6-thinking",
	"claude-opus-4-6":            "claude-opus-4.6",
	"claude-opus-4-6-thinking":   "claude-opus-4.6-thinking",
	"claude-opus-4-7":            "claude-opus-4-7-medium",
	"claude-opus-4.7":            "claude-opus-4-7-medium",
	"claude-opus-4.7-thinking":   "claude-opus-4-7-medium-thinking",
	"gpt-5.2-medium":             "gpt-5.2",
	"gpt-5.2-codex":              "gpt-5.2-codex-medium",
	"gpt-5.3-codex-medium":       "gpt-5.3-codex",
	"gpt-5.4":                    "gpt-5.4-medium",
	"gpt-5-4":                    "gpt-5.4-medium",
	"gpt-5-5":                    "gpt-5.5",
	"minimax-m2-5":               "minimax-m2.5",
	"swe-1-6":                    "swe-1.6",
	"swe-1-6-fast":               "swe-1.6-fast",
}

func buildWindsurfModelLookup() map[string]windsurfModelInfo {
	lookup := make(map[string]windsurfModelInfo, len(windsurfModelCatalog)*3)
	for _, entry := range windsurfModelCatalog {
		info := windsurfModelInfo{
			Name:       entry.Name,
			EnumValue:  entry.EnumValue,
			ModelUID:   entry.ModelUID,
			Provider:   entry.OwnedBy,
			Deprecated: entry.Deprecated,
		}
		lookup[strings.ToLower(entry.ID)] = info
		lookup[strings.ToLower(entry.Name)] = info
		if entry.ModelUID != "" {
			lookup[entry.ModelUID] = info
			lookup[strings.ToLower(entry.ModelUID)] = info
		}
	}
	return lookup
}

func DefaultWindsurfModels() []openai.Model {
	models := make([]openai.Model, 0, len(windsurfModelCatalog))
	for _, entry := range windsurfModelCatalog {
		if entry.Deprecated {
			continue
		}
		models = append(models, openai.Model{
			ID:          entry.ID,
			Object:      "model",
			Created:     windsurfModelCreatedAt,
			OwnedBy:     entry.OwnedBy,
			Type:        "model",
			DisplayName: entry.DisplayName,
		})
	}
	return models
}
