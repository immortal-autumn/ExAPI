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
  </div>
</template>

<script setup lang="ts">
import GatewayCooldownPanel from '@/views/admin/settings/gateway/GatewayCooldownPanel.vue'
import GatewayStreamTimeoutPanel from '@/views/admin/settings/gateway/GatewayStreamTimeoutPanel.vue'
import GatewayRectifierPanel from '@/views/admin/settings/gateway/GatewayRectifierPanel.vue'
import GatewayBetaPolicyPanel from '@/views/admin/settings/gateway/GatewayBetaPolicyPanel.vue'
import GatewayOpenAIFastPolicyPanel from '@/views/admin/settings/gateway/GatewayOpenAIFastPolicyPanel.vue'

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
}>()
</script>
