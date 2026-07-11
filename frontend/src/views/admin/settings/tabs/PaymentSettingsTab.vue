<template>
  <div class="space-y-6">
    <PaymentSystemSettingsPanel
      :form="form"
      :load-balance-options="loadBalanceOptions"
      :cancel-rate-limit-mode-options="cancelRateLimitModeOptions"
      :cancel-rate-limit-unit-options="cancelRateLimitUnitOptions"
      :all-payment-types="allPaymentTypes"
      :payment-guide-href="paymentGuideHref"
      :payment-methods-href="paymentMethodsHref"
      :toggle-payment-type="togglePaymentType"
      :is-payment-type-enabled="isPaymentTypeEnabled"
    />

    <PaymentProviderManagementPanel
      :form="form"
      :providers="providers"
      :providers-loading="providersLoading"
      :has-any-payment-type-enabled="hasAnyPaymentTypeEnabled"
      :all-payment-types="allPaymentTypes"
      :load-providers="loadProviders"
      :open-create-provider="openCreateProvider"
      :open-edit-provider="openEditProvider"
      :confirm-delete-provider="confirmDeleteProvider"
      :handle-toggle-field="handleToggleField"
      :handle-toggle-type="handleToggleType"
      :handle-reorder-providers="handleReorderProviders"
    />
  </div>
</template>

<script setup lang="ts">
import PaymentSystemSettingsPanel from '@/views/admin/settings/payment/PaymentSystemSettingsPanel.vue'
import PaymentProviderManagementPanel from '@/views/admin/settings/payment/PaymentProviderManagementPanel.vue'

type AnyHandler = (...args: any[]) => void

defineProps<{
  form: any
  loadBalanceOptions: any[]
  cancelRateLimitModeOptions: any[]
  cancelRateLimitUnitOptions: any[]
  allPaymentTypes: Array<{ value: string; label: string }>
  paymentGuideHref: string
  paymentMethodsHref: string
  togglePaymentType: AnyHandler
  isPaymentTypeEnabled: (value: string) => boolean
  providers: any[]
  providersLoading: boolean
  hasAnyPaymentTypeEnabled: boolean
  loadProviders: AnyHandler
  openCreateProvider: AnyHandler
  openEditProvider: AnyHandler
  confirmDeleteProvider: AnyHandler
  handleToggleField: AnyHandler
  handleToggleType: AnyHandler
  handleReorderProviders: AnyHandler
}>()
</script>
