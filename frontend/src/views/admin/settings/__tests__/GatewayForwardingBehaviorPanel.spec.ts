import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => params?.index ? `${key}:${params.index}` : key,
  }),
}))

import GatewayForwardingBehaviorPanel from '../gateway/GatewayForwardingBehaviorPanel.vue'

describe('GatewayForwardingBehaviorPanel', () => {
  it('renders gateway forwarding controls and delegates prompt block actions', async () => {
    const addBlock = vi.fn()
    const resetBlocks = vi.fn()
    const toggleBlock = vi.fn()
    const moveBlock = vi.fn()
    const removeBlock = vi.fn()
    const applyPreset = vi.fn()
    const markCustom = vi.fn()

    const wrapper = mount(GatewayForwardingBehaviorPanel, {
      props: {
        form: {
          enable_fingerprint_unification: true,
          enable_metadata_passthrough: true,
          enable_cch_signing: true,
          enable_claude_oauth_system_prompt_injection: true,
          enable_anthropic_cache_ttl_1h_injection: false,
          rewrite_message_cache_control: false,
          enable_client_dateline_normalization: true,
          antigravity_user_agent_version: '1.2.3',
          openai_codex_user_agent: 'codex-test',
        },
        claudeOAuthSystemPromptBlocks: [
          {
            id: 'block-1',
            enabled: true,
            expanded: true,
            preset: 'custom',
            type: 'text',
            text: 'hello',
            cacheControlEnabled: true,
            cacheControlTTL: '5m',
          },
        ],
        claudeOAuthSystemPromptPresetOptions: [{ value: 'custom', label: 'Custom' }],
        claudeOAuthSystemPromptBlockTypeOptions: [{ value: 'text', label: 'Text' }],
        claudeOAuthSystemPromptCacheTTLOptions: [{ value: '5m', label: '5m' }],
        getClaudeOAuthPresetLabel: (preset: string) => `preset:${preset}`,
        addClaudeOAuthSystemPromptBlock: addBlock,
        resetClaudeOAuthSystemPromptBlocks: resetBlocks,
        toggleClaudeOAuthSystemPromptBlock: toggleBlock,
        moveClaudeOAuthSystemPromptBlock: moveBlock,
        removeClaudeOAuthSystemPromptBlock: removeBlock,
        applyClaudeOAuthSystemPromptPreset: applyPreset,
        markClaudeOAuthSystemPromptBlockCustom: markCustom,
      },
      global: {
        stubs: {
          Toggle: { template: '<button type="button">toggle</button>' },
          Icon: { props: ['name'], template: '<span>{{ name }}</span>' },
          Select: { template: '<select><slot /></select>' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.gatewayForwarding.title')
    expect(wrapper.text()).toContain('admin.settings.gatewayForwarding.fingerprintUnification')
    expect(wrapper.text()).toContain('admin.settings.gatewayForwarding.systemBlockTitle:1')
    expect(wrapper.text()).toContain('preset:custom')
    expect(wrapper.find('input[placeholder="admin.settings.gatewayForwarding.antigravityUserAgentVersionPlaceholder"]').exists()).toBe(true)
    expect(wrapper.find('input[placeholder="admin.settings.gatewayForwarding.openaiCodexUserAgentPlaceholder"]').exists()).toBe(true)

    await wrapper.findAll('button').find((button) => button.text().includes('admin.settings.gatewayForwarding.addSystemBlock'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.settings.gatewayForwarding.resetSystemBlocks'))!.trigger('click')

    expect(addBlock).toHaveBeenCalledTimes(1)
    expect(resetBlocks).toHaveBeenCalledTimes(1)
  })
})
