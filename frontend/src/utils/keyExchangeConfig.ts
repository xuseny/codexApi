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

const opencodePermission = {
  read: 'allow',
  list: 'allow',
  glob: 'allow',
  grep: 'allow',
  lsp: 'allow',
  edit: 'ask',
  bash: 'ask',
  webfetch: 'ask',
  websearch: 'ask'
}

function compactVariants(value: Record<string, unknown>) {
  return Object.fromEntries(Object.entries(value).filter(([, v]) => v !== undefined))
}

function openCodeModel(name: string, context: number, output: number) {
  return {
    name,
    limit: {
      context,
      output
    },
    modalities: {
      input: ['text', 'image', 'pdf'],
      output: ['text']
    },
    attachment: true,
    reasoning: true,
    tool_call: true,
    interleaved: true,
    options: {
      store: false
    },
    variants: compactVariants({
      low: {},
      medium: {},
      high: {},
      xhigh: {}
    })
  }
}

function buildOpenCodeModels(platform: GroupPlatform) {
  if (platform === 'anthropic' || platform === 'antigravity') {
    return {
      'claude-opus-4-7': openCodeModel('Claude Opus 4.7', 200000, 128000),
      'claude-opus-4-7-low': openCodeModel('Claude Opus 4.7 Low', 200000, 128000),
      'claude-opus-4-7-medium': openCodeModel('Claude Opus 4.7 Medium', 200000, 128000),
      'claude-opus-4-7-high': openCodeModel('Claude Opus 4.7 High', 200000, 128000),
      'claude-opus-4-7-xhigh': openCodeModel('Claude Opus 4.7 XHigh', 200000, 128000),
      'claude-opus-4-6': openCodeModel('Claude Opus 4.6', 200000, 128000),
      'claude-sonnet-4-6': openCodeModel('Claude Sonnet 4.6', 200000, 64000),
      'claude-haiku-4-5': openCodeModel('Claude Haiku 4.5', 200000, 64000)
    }
  }
  if (platform === 'gemini') {
    return {
      'gemini-2.5-pro': openCodeModel('Gemini 2.5 Pro', 1000000, 65536),
      'gemini-2.5-flash': openCodeModel('Gemini 2.5 Flash', 1000000, 65536)
    }
  }
  return {
    'gpt-5.5': openCodeModel('GPT-5.5', 1050000, 128000),
    'gpt-5.4': openCodeModel('GPT-5.4', 1050000, 128000),
    'gpt-5.4-mini': openCodeModel('GPT-5.4 Mini', 400000, 128000),
    'gpt-5.3-codex': openCodeModel('GPT-5.3 Codex', 400000, 128000),
    'gpt-5.3-codex-spark': openCodeModel('GPT-5.3 Codex Spark', 128000, 32000),
    'gpt-5.2': openCodeModel('GPT-5.2', 400000, 128000),
    'codex-mini-latest': openCodeModel('Codex Mini', 200000, 100000)
  }
}

function buildOpenCodeFile(baseUrl: string, apiKey: string, providerId: string, platform: GroupPlatform): KeyExchangeConfigFile[] {
  const providerMap: Record<string, unknown> = {
    [providerId]: {
      options: {
        baseURL: baseUrl,
        apiKey
      },
      ...(platform === 'anthropic' || platform === 'antigravity' ? { npm: '@ai-sdk/anthropic' } : {}),
      ...(platform === 'gemini' ? { npm: '@ai-sdk/google' } : {}),
      models: buildOpenCodeModels(platform)
    }
  }

  return [
    {
      fileName: 'opencode.json',
      suggestedPath: '~/.config/opencode/opencode.json',
      content: JSON.stringify(
        {
          provider: providerMap,
          agent: {
            build: {
              options: {
                store: false
              }
            },
            plan: {
              options: {
                store: false
              }
            }
          },
          permission: opencodePermission,
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
          files: buildOpenCodeFile(apiBase, input.apiKey, 'openai', 'openai')
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
          files: buildOpenCodeFile(apiBase, input.apiKey, 'anthropic', 'anthropic')
        }
      ]
    case 'gemini':
      return [
        {
          id: 'opencode',
          label: 'OpenCode',
          description: '生成 OpenCode 的 Gemini 配置文件',
          files: buildOpenCodeFile(geminiBase, input.apiKey, 'gemini', 'gemini')
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
          files: buildOpenCodeFile(antigravityBase, input.apiKey, 'antigravity-claude', 'antigravity')
        }
      ]
    default:
      return []
  }
}
