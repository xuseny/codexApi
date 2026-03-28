import type { GroupPlatform } from '@/types'

export type KeyExchangePresetId =
  | 'codex'
  | 'codex-ws'
  | 'claude-settings'
  | 'opencode'

export interface KeyExchangeConfigFile {
  fileName: string
  suggestedPath: string
  content: string
  hint?: string
}

export interface KeyExchangeConfigPreset {
  id: KeyExchangePresetId
  label: string
  description: string
  files: KeyExchangeConfigFile[]
}

export interface BuildKeyExchangeConfigInput {
  platform: GroupPlatform
  baseUrl: string
  apiKey: string
}

const ensureV1 = (value: string) => {
  const trimmed = value.replace(/\/+$/, '')
  return trimmed.endsWith('/v1') ? trimmed : `${trimmed}/v1`
}

const ensureV1beta = (value: string) => {
  const trimmed = value.replace(/\/+$/, '')
  return trimmed.endsWith('/v1beta') ? trimmed : `${trimmed}/v1beta`
}

function buildCodexFiles(baseUrl: string, apiKey: string, websocketMode: boolean): KeyExchangeConfigFile[] {
  const configDir = '~/.codex'
  const configContent = websocketMode
    ? `model_provider = "OpenAI"
model = "gpt-5.4"
review_model = "gpt-5.4"
model_reasoning_effort = "xhigh"
disable_response_storage = true
network_access = "enabled"
windows_wsl_setup_acknowledged = true
model_context_window = 1000000
model_auto_compact_token_limit = 900000

[model_providers.OpenAI]
name = "OpenAI"
base_url = "${baseUrl}"
wire_api = "responses"
supports_websockets = true
requires_openai_auth = true

[features]
responses_websockets_v2 = true`
    : `model_provider = "OpenAI"
model = "gpt-5.4"
review_model = "gpt-5.4"
model_reasoning_effort = "xhigh"
disable_response_storage = true
network_access = "enabled"
windows_wsl_setup_acknowledged = true
model_context_window = 1000000
model_auto_compact_token_limit = 900000

[model_providers.OpenAI]
name = "OpenAI"
base_url = "${baseUrl}"
wire_api = "responses"
requires_openai_auth = true`

  return [
    {
      fileName: 'config.toml',
      suggestedPath: `${configDir}/config.toml`,
      content: configContent,
      hint: '建议覆盖 Codex CLI 的 config.toml'
    },
    {
      fileName: 'auth.json',
      suggestedPath: `${configDir}/auth.json`,
      content: `{
  "OPENAI_API_KEY": "${apiKey}"
}`,
      hint: '建议覆盖 Codex CLI 的 auth.json'
    }
  ]
}

function buildClaudeSettingsFile(baseUrl: string, apiKey: string): KeyExchangeConfigFile[] {
  return [
    {
      fileName: 'settings.json',
      suggestedPath: '~/.claude/settings.json',
      content: `{
  "env": {
    "ANTHROPIC_BASE_URL": "${baseUrl}",
    "ANTHROPIC_AUTH_TOKEN": "${apiKey}",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "CLAUDE_CODE_ATTRIBUTION_HEADER": "0"
  }
}`,
      hint: '建议覆盖 Claude Code 的 settings.json'
    }
  ]
}

function buildOpenCodeFile(baseUrl: string, apiKey: string, providerId: string): KeyExchangeConfigFile[] {
  const providerMap: Record<string, unknown> = {
    [providerId]: {
      options: {
        baseURL: baseUrl,
        apiKey
      }
    }
  }

  return [
    {
      fileName: 'opencode.json',
      suggestedPath: '~/.config/opencode/opencode.json',
      content: JSON.stringify(
        {
          provider: providerMap,
          $schema: 'https://opencode.ai/config.json'
        },
        null,
        2
      ),
      hint: '建议覆盖 OpenCode 的 opencode.json'
    }
  ]
}

export function buildKeyExchangeConfigPresets(input: BuildKeyExchangeConfigInput): KeyExchangeConfigPreset[] {
  const baseRoot = input.baseUrl.replace(/\/v1\/?$/, '').replace(/\/+$/, '')
  const apiBase = ensureV1(baseRoot)
  const antigravityBase = ensureV1(`${baseRoot}/antigravity`)
  const geminiBase = ensureV1beta(baseRoot)

  switch (input.platform) {
    case 'openai':
      return [
        {
          id: 'codex',
          label: 'Codex CLI',
          description: '生成 Codex CLI 的 config.toml 与 auth.json',
          files: buildCodexFiles(apiBase, input.apiKey, false)
        },
        {
          id: 'codex-ws',
          label: 'Codex CLI WS',
          description: '生成启用 WebSocket v2 的 Codex CLI 配置',
          files: buildCodexFiles(apiBase, input.apiKey, true)
        },
        {
          id: 'opencode',
          label: 'OpenCode',
          description: '生成 OpenCode 的 opencode.json',
          files: buildOpenCodeFile(apiBase, input.apiKey, 'openai')
        }
      ]
    case 'anthropic':
      return [
        {
          id: 'claude-settings',
          label: 'Claude Code',
          description: '生成 Claude Code 的 settings.json',
          files: buildClaudeSettingsFile(apiBase, input.apiKey)
        },
        {
          id: 'opencode',
          label: 'OpenCode',
          description: '生成 OpenCode 的配置文件',
          files: buildOpenCodeFile(apiBase, input.apiKey, 'anthropic')
        }
      ]
    case 'gemini':
      return [
        {
          id: 'opencode',
          label: 'OpenCode',
          description: '生成 OpenCode 的 Gemini 配置文件',
          files: buildOpenCodeFile(geminiBase, input.apiKey, 'gemini')
        }
      ]
    case 'antigravity':
      return [
        {
          id: 'claude-settings',
          label: 'Claude Code',
          description: '生成 Claude Code 的 Antigravity 配置',
          files: buildClaudeSettingsFile(antigravityBase, input.apiKey)
        },
        {
          id: 'opencode',
          label: 'OpenCode',
          description: '生成 OpenCode 的 Antigravity 配置文件',
          files: buildOpenCodeFile(antigravityBase, input.apiKey, 'antigravity-claude')
        }
      ]
    default:
      return []
  }
}
