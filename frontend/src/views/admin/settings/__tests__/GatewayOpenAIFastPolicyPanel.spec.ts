import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => params?.index ? `${key}:${params.index}` : key,
  }),
}))

import GatewayOpenAIFastPolicyPanel from '../gateway/GatewayOpenAIFastPolicyPanel.vue'

describe('GatewayOpenAIFastPolicyPanel', () => {
  it('renders OpenAI fast policy rules and delegates rule/model-pattern editing', async () => {
    const addOpenAIFastPolicyRule = vi.fn()
    const removeOpenAIFastPolicyRule = vi.fn()
    const addOpenAIFastPolicyModelPattern = vi.fn((rule) => {
      if (!rule.model_whitelist) rule.model_whitelist = []
      rule.model_whitelist.push('')
    })
    const removeOpenAIFastPolicyModelPattern = vi.fn((rule, index: number) => {
      rule.model_whitelist?.splice(index, 1)
    })
    const rule = {
      service_tier: 'priority',
      action: 'block',
      scope: 'all',
      error_message: 'blocked',
      model_whitelist: ['gpt-*'],
      fallback_action: 'pass',
      fallback_error_message: '',
    }
    const openaiFastPolicyForm = { rules: [rule] }

    const wrapper = mount(GatewayOpenAIFastPolicyPanel, {
      props: {
        openaiFastPolicyForm,
        openaiFastPolicyTierOptions: [{ value: 'priority', label: 'priority' }],
        openaiFastPolicyActionOptions: [{ value: 'pass', label: 'pass' }, { value: 'block', label: 'block' }],
        openaiFastPolicyScopeOptions: [{ value: 'all', label: 'all' }],
        addOpenAIFastPolicyRule,
        removeOpenAIFastPolicyRule,
        addOpenAIFastPolicyModelPattern,
        removeOpenAIFastPolicyModelPattern,
      },
      global: {
        stubs: {
          Select: { template: '<select />' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.openaiFastPolicy.title')
    expect(wrapper.text()).toContain('admin.settings.openaiFastPolicy.ruleHeader:1')

    await wrapper.find('[data-test="add-openai-fast-policy-pattern-0"]').trigger('click')
    expect(addOpenAIFastPolicyModelPattern).toHaveBeenCalledWith(rule)

    await wrapper.find('[data-test="remove-openai-fast-policy-pattern-0-0"]').trigger('click')
    expect(removeOpenAIFastPolicyModelPattern).toHaveBeenCalledWith(rule, 0)

    await wrapper.find('[data-test="remove-openai-fast-policy-rule-0"]').trigger('click')
    expect(removeOpenAIFastPolicyRule).toHaveBeenCalledWith(0)

    await wrapper.find('[data-test="add-openai-fast-policy-rule"]').trigger('click')
    expect(addOpenAIFastPolicyRule).toHaveBeenCalledTimes(1)
  })
})
