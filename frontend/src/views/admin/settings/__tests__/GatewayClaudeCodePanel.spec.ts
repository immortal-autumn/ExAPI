import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import GatewayClaudeCodePanel from '../gateway/GatewayClaudeCodePanel.vue'

describe('GatewayClaudeCodePanel', () => {
  it('renders Claude Code min and max version settings', () => {
    const form = {
      min_claude_code_version: '1.0.0',
      max_claude_code_version: '2.0.0',
    }

    const wrapper = mount(GatewayClaudeCodePanel, {
      props: { form },
    })

    expect(wrapper.text()).toContain('admin.settings.claudeCode.title')
    expect(wrapper.text()).toContain('admin.settings.claudeCode.minVersion')
    expect(wrapper.text()).toContain('admin.settings.claudeCode.maxVersion')
    expect(wrapper.find('input[placeholder="admin.settings.claudeCode.minVersionPlaceholder"]').exists()).toBe(true)
    expect(wrapper.find('input[placeholder="admin.settings.claudeCode.maxVersionPlaceholder"]').exists()).toBe(true)
  })
})
