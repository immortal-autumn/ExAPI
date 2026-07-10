import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => params?.index ? `${key}:${params.index}` : key,
  }),
}))

import GatewaySettingsTab from '../tabs/GatewaySettingsTab.vue'

describe('GatewaySettingsTab', () => {
  it('renders the extracted gateway panel stack', () => {
    const fn = vi.fn()
    const wrapper = mount(GatewaySettingsTab, {
      props: {
        overloadCooldownLoading: false,
        overloadCooldownSaving: false,
        overloadCooldownForm: { enabled: true, cooldown_minutes: 15 },
        saveOverloadCooldownSettings: fn,
        rateLimit429CooldownLoading: false,
        rateLimit429CooldownSaving: false,
        rateLimit429CooldownForm: { enabled: true, cooldown_seconds: 60 },
        saveRateLimit429CooldownSettings: fn,
        streamTimeoutLoading: false,
        streamTimeoutSaving: false,
        streamTimeoutForm: { enabled: true, action: 'temp_unsched', temp_unsched_minutes: 10, threshold_count: 3, threshold_window_minutes: 5 },
        saveStreamTimeoutSettings: fn,
        rectifierLoading: false,
        rectifierSaving: false,
        rectifierForm: { enabled: true, thinking_signature_enabled: true, thinking_budget_enabled: true, apikey_signature_enabled: false, apikey_signature_patterns: [] },
        saveRectifierSettings: fn,
        betaPolicyLoading: false,
        betaPolicySaving: false,
        betaPolicyForm: { rules: [] },
        betaPolicyActionOptions: [],
        betaPolicyScopeOptions: [],
        betaPresets: {},
        commonModelPatterns: [],
        getBetaDisplayName: (token: string) => token,
        applyBetaPreset: fn,
        addQuickPattern: fn,
        saveBetaPolicySettings: fn,
        openaiFastPolicyForm: { rules: [] },
        openaiFastPolicyTierOptions: [],
        openaiFastPolicyActionOptions: [],
        openaiFastPolicyScopeOptions: [],
        addOpenAIFastPolicyRule: fn,
        removeOpenAIFastPolicyRule: fn,
        addOpenAIFastPolicyModelPattern: fn,
        removeOpenAIFastPolicyModelPattern: fn,
      },
      global: {
        stubs: {
          GatewayCooldownPanel: { template: '<div>cooldown-panel</div>' },
          GatewayStreamTimeoutPanel: { template: '<div>stream-timeout-panel</div>' },
          GatewayRectifierPanel: { template: '<div>rectifier-panel</div>' },
          GatewayBetaPolicyPanel: { template: '<div>beta-policy-panel</div>' },
          GatewayOpenAIFastPolicyPanel: { template: '<div>openai-fast-policy-panel</div>' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.overloadCooldown.title')
    expect(wrapper.text()).toContain('admin.settings.rateLimit429Cooldown.title')
    expect(wrapper.text()).toContain('admin.settings.streamTimeout.title')
    expect(wrapper.text()).toContain('admin.settings.rectifier.title')
    expect(wrapper.text()).toContain('admin.settings.betaPolicy.title')
    expect(wrapper.text()).toContain('admin.settings.openaiFastPolicy.title')
  })
})
