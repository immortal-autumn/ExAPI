import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => params?.index ? `${key}:${params.index}` : key,
  }),
}))

vi.mock('@/components/common/ProxySelector.vue', () => ({
  default: { template: '<div>proxy-selector</div>' },
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
        form: {
          min_claude_code_version: '1.0.0',
          max_claude_code_version: '2.0.0',
          min_codex_version: '0.1.0',
          max_codex_version: '1.0.0',
          codex_cli_only_allow_app_server_clients: true,
          allow_ungrouped_key_scheduling: true,
          openai_advanced_scheduler_enabled: false,
          openai_advanced_scheduler_sticky_weighted_enabled: false,
          openai_advanced_scheduler_subscription_priority_enabled: false,
          enable_fingerprint_unification: true,
          enable_metadata_passthrough: true,
          enable_cch_signing: true,
          enable_claude_oauth_system_prompt_injection: true,
          enable_anthropic_cache_ttl_1h_injection: false,
          rewrite_message_cache_control: false,
          enable_client_dateline_normalization: true,
          antigravity_user_agent_version: '1.2.3',
          openai_codex_user_agent: 'codex-test',
          allow_user_view_error_requests: true,
        },
        codexFingerprintRows: [],
        codexFingerprintNoRequired: false,
        codexBlacklistRows: [],
        codexWhitelistRows: [],
        addCodexFingerprintRow: fn,
        removeCodexFingerprintRow: fn,
        addCodexBlacklistRow: fn,
        removeCodexBlacklistRow: fn,
        addCodexWhitelistRow: fn,
        removeCodexWhitelistRow: fn,
        openAIAdvancedSchedulerWeightFields: [],
        claudeOAuthSystemPromptBlocks: [],
        claudeOAuthSystemPromptPresetOptions: [],
        claudeOAuthSystemPromptBlockTypeOptions: [],
        claudeOAuthSystemPromptCacheTTLOptions: [],
        getClaudeOAuthPresetLabel: (preset: string) => preset,
        addClaudeOAuthSystemPromptBlock: fn,
        resetClaudeOAuthSystemPromptBlocks: fn,
        toggleClaudeOAuthSystemPromptBlock: fn,
        moveClaudeOAuthSystemPromptBlock: fn,
        removeClaudeOAuthSystemPromptBlock: fn,
        applyClaudeOAuthSystemPromptPreset: fn,
        markClaudeOAuthSystemPromptBlockCustom: fn,
        webSearchConfig: { enabled: false, providers: [] },
        expandedProviders: {},
        apiKeyVisible: {},
        webSearchProxies: [],
        addWebSearchProvider: fn,
        removeWebSearchProvider: fn,
        toggleProviderExpand: fn,
        copyApiKey: fn,
        formatSubscribedAt: () => '',
        parseSubscribedAt: () => null,
        quotaPercentage: () => 0,
        resetWebSearchUsage: fn,
        openTestDialog: fn,
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
    expect(wrapper.text()).toContain('admin.settings.claudeCode.title')
    expect(wrapper.text()).toContain('admin.settings.gatewayForwarding.codexHardeningTitle')
    expect(wrapper.text()).toContain('admin.settings.scheduling.title')
    expect(wrapper.text()).toContain('admin.settings.gatewayForwarding.title')
    expect(wrapper.text()).toContain('admin.settings.webSearchEmulation.title')
    expect(wrapper.text()).toContain('admin.settings.usageRecords.title')
  })
})
