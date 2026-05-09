import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn().mockResolvedValue(true)
  })
}))

import UseKeyModal from '../UseKeyModal.vue'

describe('UseKeyModal', () => {
  const mountModal = (
    platform: 'openai' | 'anthropic' | 'windsurf',
    extraProps: Record<string, unknown> = {}
  ) =>
    mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform,
        ...extraProps
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

  const openOpenCodeTab = async (wrapper: ReturnType<typeof mount>) => {
    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )

    expect(opencodeTab).toBeDefined()
    await opencodeTab!.trigger('click')
    await nextTick()
  }

  it('renders GPT-5.4 mini entry in OpenCode config', async () => {
    const wrapper = mountModal('openai')

    await openOpenCodeTab(wrapper)

    const codeBlock = wrapper.find('pre code')
    expect(codeBlock.exists()).toBe(true)
    expect(codeBlock.text()).toContain('"name": "GPT-5.4 Mini"')
    expect(codeBlock.text()).not.toContain('"name": "GPT-5.4 Nano"')
  })

  it('enables OpenCode reasoning and tool metadata for OpenAI models', async () => {
    const wrapper = mountModal('openai')

    await openOpenCodeTab(wrapper)

    const config = JSON.parse(wrapper.find('pre code').text())
    expect(config.provider.openai.models['gpt-5.5'].reasoning).toBe(true)
    expect(config.provider.openai.models['gpt-5.5'].tool_call).toBe(true)
    expect(config.provider.openai.models['gpt-5.5'].variants.high.reasoningEffort).toBe('high')
    expect(config.permission.read).toBe('allow')
    expect(config.permission.grep).toBe('allow')
  })

  it('renders Anthropic provider in OpenAI OpenCode config when Claude dispatch is enabled', async () => {
    const wrapper = mountModal('openai', { allowMessagesDispatch: true })

    await openOpenCodeTab(wrapper)

    const config = JSON.parse(wrapper.find('pre code').text())
    expect(config.provider.anthropic.npm).toBe('@ai-sdk/anthropic')
    expect(config.provider.anthropic.models['claude-opus-4-7'].tool_call).toBe(true)
    expect(config.provider.anthropic.models['claude-opus-4-7'].reasoning).toBe(true)
    expect(config.provider.anthropic.models['claude-opus-4-7-xhigh']).toBeUndefined()
  })

  it('does not render Anthropic provider in normal OpenAI OpenCode config', async () => {
    const wrapper = mountModal('openai')

    await openOpenCodeTab(wrapper)

    const config = JSON.parse(wrapper.find('pre code').text())
    expect(config.provider.anthropic).toBeUndefined()
  })

  it('renders combined OpenAI and Anthropic OpenCode config for Windsurf groups', async () => {
    const wrapper = mountModal('windsurf')

    await openOpenCodeTab(wrapper)

    const config = JSON.parse(wrapper.find('pre code').text())
    expect(config.provider.openai.models['gpt-5.5'].tool_call).toBe(true)
    expect(config.provider.anthropic.models['claude-sonnet-4-6'].tool_call).toBe(true)
    expect(config.provider.anthropic.models['claude-opus-4-7-xhigh'].tool_call).toBe(true)
    expect(config.provider.anthropic.options.baseURL).toBe('https://example.com/v1')
  })

  it('renders Claude model metadata in Anthropic OpenCode config', async () => {
    const wrapper = mountModal('anthropic')

    await openOpenCodeTab(wrapper)

    const config = JSON.parse(wrapper.find('pre code').text())
    expect(config.provider.anthropic.models['claude-opus-4-7'].reasoning).toBe(true)
    expect(config.provider.anthropic.models['claude-opus-4-7'].tool_call).toBe(true)
    expect(config.provider.anthropic.models['claude-opus-4-7'].variants.high.thinking.budgetTokens).toBe(24576)
    expect(config.provider.anthropic.models['claude-opus-4-7-xhigh']).toBeUndefined()
    expect(config.provider.anthropic.models['claude-opus-4-5-20251101'].tool_call).toBe(true)
  })
})
