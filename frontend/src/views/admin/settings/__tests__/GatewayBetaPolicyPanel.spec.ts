import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import GatewayBetaPolicyPanel from '../gateway/GatewayBetaPolicyPanel.vue'

describe('GatewayBetaPolicyPanel', () => {
  it('renders beta rules, applies helpers, and delegates save', async () => {
    const saveBetaPolicySettings = vi.fn()
    const getBetaDisplayName = vi.fn((token: string) => token === 'context-1m-2025-08-07' ? 'Context 1M' : token)
    const applyBetaPreset = vi.fn((rule, preset) => {
      rule.action = preset.action
      rule.model_whitelist = [...preset.model_whitelist]
      rule.fallback_action = preset.fallback_action
    })
    const addQuickPattern = vi.fn((rule, pattern) => {
      if (!rule.model_whitelist) rule.model_whitelist = []
      rule.model_whitelist.push(pattern)
    })
    const betaPolicyForm = {
      rules: [
        {
          beta_token: 'context-1m-2025-08-07',
          action: 'block',
          scope: 'all',
          error_message: 'blocked',
          model_whitelist: [],
          fallback_action: 'pass',
          fallback_error_message: '',
        },
      ],
    }

    const wrapper = mount(GatewayBetaPolicyPanel, {
      props: {
        betaPolicyLoading: false,
        betaPolicySaving: false,
        betaPolicyForm,
        betaPolicyActionOptions: [{ value: 'pass', label: 'pass' }, { value: 'block', label: 'block' }],
        betaPolicyScopeOptions: [{ value: 'all', label: 'all' }],
        betaPresets: {
          'context-1m-2025-08-07': [
            {
              label: 'Opus only',
              description: 'Only Opus',
              action: 'pass',
              model_whitelist: ['claude-opus-*'],
              fallback_action: 'filter',
            },
          ],
        },
        commonModelPatterns: ['claude-sonnet-*'],
        getBetaDisplayName,
        applyBetaPreset,
        addQuickPattern,
        saveBetaPolicySettings,
      },
      global: {
        stubs: {
          Select: { template: '<select />' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.betaPolicy.title')
    expect(wrapper.text()).toContain('Context 1M')
    expect(wrapper.text()).toContain('Opus only')

    await wrapper.find('[data-test="beta-preset-0-0"]').trigger('click')
    expect(applyBetaPreset).toHaveBeenCalledTimes(1)
    expect(betaPolicyForm.rules[0].model_whitelist).toEqual(['claude-opus-*'])

    await wrapper.find('[data-test="beta-quick-pattern-0-0"]').trigger('click')
    expect(addQuickPattern).toHaveBeenCalledWith(betaPolicyForm.rules[0], 'claude-sonnet-*')

    await wrapper.find('[data-test="save-beta-policy"]').trigger('click')
    expect(saveBetaPolicySettings).toHaveBeenCalledTimes(1)
  })
})
