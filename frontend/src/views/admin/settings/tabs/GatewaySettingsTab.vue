<template>
  <div class="space-y-6">
    <GatewayCooldownPanel
      :overload-cooldown-loading="overloadCooldownLoading"
      :overload-cooldown-saving="overloadCooldownSaving"
      :overload-cooldown-form="overloadCooldownForm"
      :save-overload-cooldown-settings="saveOverloadCooldownSettings"
      :rate-limit429-cooldown-loading="rateLimit429CooldownLoading"
      :rate-limit429-cooldown-saving="rateLimit429CooldownSaving"
      :rate-limit429-cooldown-form="rateLimit429CooldownForm"
      :save-rate-limit429-cooldown-settings="saveRateLimit429CooldownSettings"
    />

    <GatewayStreamTimeoutPanel
      :stream-timeout-loading="streamTimeoutLoading"
      :stream-timeout-saving="streamTimeoutSaving"
      :stream-timeout-form="streamTimeoutForm"
      :save-stream-timeout-settings="saveStreamTimeoutSettings"
    />

    <GatewayRectifierPanel
      :rectifier-loading="rectifierLoading"
      :rectifier-saving="rectifierSaving"
      :rectifier-form="rectifierForm"
      :save-rectifier-settings="saveRectifierSettings"
    />

    <GatewayBetaPolicyPanel
      :beta-policy-loading="betaPolicyLoading"
      :beta-policy-saving="betaPolicySaving"
      :beta-policy-form="betaPolicyForm"
      :beta-policy-action-options="betaPolicyActionOptions"
      :beta-policy-scope-options="betaPolicyScopeOptions"
      :beta-presets="betaPresets"
      :common-model-patterns="commonModelPatterns"
      :get-beta-display-name="getBetaDisplayName"
      :apply-beta-preset="applyBetaPreset"
      :add-quick-pattern="addQuickPattern"
      :save-beta-policy-settings="saveBetaPolicySettings"
    />

    <GatewayOpenAIFastPolicyPanel
      :openai-fast-policy-form="openaiFastPolicyForm"
      :openai-fast-policy-tier-options="openaiFastPolicyTierOptions"
      :openai-fast-policy-action-options="openaiFastPolicyActionOptions"
      :openai-fast-policy-scope-options="openaiFastPolicyScopeOptions"
      :add-open-a-i-fast-policy-rule="addOpenAIFastPolicyRule"
      :remove-open-a-i-fast-policy-rule="removeOpenAIFastPolicyRule"
      :add-open-a-i-fast-policy-model-pattern="addOpenAIFastPolicyModelPattern"
      :remove-open-a-i-fast-policy-model-pattern="removeOpenAIFastPolicyModelPattern"
    />

    <GatewayClaudeCodePanel :form="form" />

    <GatewayCodexHardeningPanel
      :form="form"
      :codex-fingerprint-rows="codexFingerprintRows"
      :codex-fingerprint-no-required="codexFingerprintNoRequired"
      :codex-blacklist-rows="codexBlacklistRows"
      :codex-whitelist-rows="codexWhitelistRows"
      :add-codex-fingerprint-row="addCodexFingerprintRow"
      :remove-codex-fingerprint-row="removeCodexFingerprintRow"
      :add-codex-blacklist-row="addCodexBlacklistRow"
      :remove-codex-blacklist-row="removeCodexBlacklistRow"
      :add-codex-whitelist-row="addCodexWhitelistRow"
      :remove-codex-whitelist-row="removeCodexWhitelistRow"
    />

    <GatewaySchedulingPanel
      :form="form"
      :open-a-i-advanced-scheduler-weight-fields="openAIAdvancedSchedulerWeightFields"
    />

    <GatewayForwardingBehaviorPanel
      :form="form"
      :claude-o-auth-system-prompt-blocks="claudeOAuthSystemPromptBlocks"
      :claude-o-auth-system-prompt-preset-options="claudeOAuthSystemPromptPresetOptions"
      :claude-o-auth-system-prompt-block-type-options="claudeOAuthSystemPromptBlockTypeOptions"
      :claude-o-auth-system-prompt-cache-t-t-l-options="claudeOAuthSystemPromptCacheTTLOptions"
      :get-claude-o-auth-preset-label="getClaudeOAuthPresetLabel"
      :add-claude-o-auth-system-prompt-block="addClaudeOAuthSystemPromptBlock"
      :reset-claude-o-auth-system-prompt-blocks="resetClaudeOAuthSystemPromptBlocks"
      :toggle-claude-o-auth-system-prompt-block="toggleClaudeOAuthSystemPromptBlock"
      :move-claude-o-auth-system-prompt-block="moveClaudeOAuthSystemPromptBlock"
      :remove-claude-o-auth-system-prompt-block="removeClaudeOAuthSystemPromptBlock"
      :apply-claude-o-auth-system-prompt-preset="applyClaudeOAuthSystemPromptPreset"
      :mark-claude-o-auth-system-prompt-block-custom="markClaudeOAuthSystemPromptBlockCustom"
    />

    <GatewayWebSearchEmulationPanel
      :web-search-config="webSearchConfig"
      :expanded-providers="expandedProviders"
      :api-key-visible="apiKeyVisible"
      :web-search-proxies="webSearchProxies"
      :add-web-search-provider="addWebSearchProvider"
      :remove-web-search-provider="removeWebSearchProvider"
      :toggle-provider-expand="toggleProviderExpand"
      :copy-api-key="copyApiKey"
      :format-subscribed-at="formatSubscribedAt"
      :parse-subscribed-at="parseSubscribedAt"
      :quota-percentage="quotaPercentage"
      :reset-web-search-usage="resetWebSearchUsage"
      :open-test-dialog="openTestDialog"
    />

    <GatewayUsageRecordsPanel :form="form" />
  </div>
</template>

<script setup lang="ts">
import GatewayCooldownPanel from '@/views/admin/settings/gateway/GatewayCooldownPanel.vue'
import GatewayStreamTimeoutPanel from '@/views/admin/settings/gateway/GatewayStreamTimeoutPanel.vue'
import GatewayRectifierPanel from '@/views/admin/settings/gateway/GatewayRectifierPanel.vue'
import GatewayBetaPolicyPanel from '@/views/admin/settings/gateway/GatewayBetaPolicyPanel.vue'
import GatewayOpenAIFastPolicyPanel from '@/views/admin/settings/gateway/GatewayOpenAIFastPolicyPanel.vue'
import GatewayClaudeCodePanel from '@/views/admin/settings/gateway/GatewayClaudeCodePanel.vue'
import GatewayCodexHardeningPanel from '@/views/admin/settings/gateway/GatewayCodexHardeningPanel.vue'
import GatewaySchedulingPanel from '@/views/admin/settings/gateway/GatewaySchedulingPanel.vue'
import GatewayForwardingBehaviorPanel from '@/views/admin/settings/gateway/GatewayForwardingBehaviorPanel.vue'
import GatewayWebSearchEmulationPanel from '@/views/admin/settings/gateway/GatewayWebSearchEmulationPanel.vue'
import GatewayUsageRecordsPanel from '@/views/admin/settings/gateway/GatewayUsageRecordsPanel.vue'

type AnyHandler = (...args: any[]) => void

defineProps<{
  overloadCooldownLoading: boolean
  overloadCooldownSaving: boolean
  overloadCooldownForm: any
  saveOverloadCooldownSettings: AnyHandler
  rateLimit429CooldownLoading: boolean
  rateLimit429CooldownSaving: boolean
  rateLimit429CooldownForm: any
  saveRateLimit429CooldownSettings: AnyHandler
  streamTimeoutLoading: boolean
  streamTimeoutSaving: boolean
  streamTimeoutForm: any
  saveStreamTimeoutSettings: AnyHandler
  rectifierLoading: boolean
  rectifierSaving: boolean
  rectifierForm: any
  saveRectifierSettings: AnyHandler
  betaPolicyLoading: boolean
  betaPolicySaving: boolean
  betaPolicyForm: any
  betaPolicyActionOptions: any[]
  betaPolicyScopeOptions: any[]
  betaPresets: any
  commonModelPatterns: string[]
  getBetaDisplayName: (token: string) => string
  applyBetaPreset: AnyHandler
  addQuickPattern: AnyHandler
  saveBetaPolicySettings: AnyHandler
  openaiFastPolicyForm: any
  openaiFastPolicyTierOptions: any[]
  openaiFastPolicyActionOptions: any[]
  openaiFastPolicyScopeOptions: any[]
  addOpenAIFastPolicyRule: AnyHandler
  removeOpenAIFastPolicyRule: AnyHandler
  addOpenAIFastPolicyModelPattern: AnyHandler
  removeOpenAIFastPolicyModelPattern: AnyHandler
  form: any
  codexFingerprintRows: any[]
  codexFingerprintNoRequired: boolean
  codexBlacklistRows: any[]
  codexWhitelistRows: any[]
  addCodexFingerprintRow: AnyHandler
  removeCodexFingerprintRow: AnyHandler
  addCodexBlacklistRow: AnyHandler
  removeCodexBlacklistRow: AnyHandler
  addCodexWhitelistRow: AnyHandler
  removeCodexWhitelistRow: AnyHandler
  openAIAdvancedSchedulerWeightFields: any[]
  claudeOAuthSystemPromptBlocks: any[]
  claudeOAuthSystemPromptPresetOptions: any[]
  claudeOAuthSystemPromptBlockTypeOptions: any[]
  claudeOAuthSystemPromptCacheTTLOptions: any[]
  getClaudeOAuthPresetLabel: (preset: any) => string
  addClaudeOAuthSystemPromptBlock: AnyHandler
  resetClaudeOAuthSystemPromptBlocks: AnyHandler
  toggleClaudeOAuthSystemPromptBlock: AnyHandler
  moveClaudeOAuthSystemPromptBlock: AnyHandler
  removeClaudeOAuthSystemPromptBlock: AnyHandler
  applyClaudeOAuthSystemPromptPreset: AnyHandler
  markClaudeOAuthSystemPromptBlockCustom: AnyHandler
  webSearchConfig: any
  expandedProviders: Record<number, boolean>
  apiKeyVisible: Record<number, boolean>
  webSearchProxies: any[]
  addWebSearchProvider: AnyHandler
  removeWebSearchProvider: AnyHandler
  toggleProviderExpand: AnyHandler
  copyApiKey: AnyHandler
  formatSubscribedAt: (ts: number | null) => string
  parseSubscribedAt: (dateStr: string) => number | null
  quotaPercentage: (provider: any) => number
  resetWebSearchUsage: AnyHandler
  openTestDialog: AnyHandler
}>()
</script>
