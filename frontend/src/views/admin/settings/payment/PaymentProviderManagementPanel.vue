<template>
  <PaymentProviderList
    v-if="form.payment_enabled"
    :providers="providers"
    :loading="providersLoading"
    :can-create="hasAnyPaymentTypeEnabled"
    :enabled-payment-types="form.payment_enabled_types"
    :all-payment-types="allPaymentTypes"
    :redirect-label="t('admin.settings.payment.easypayRedirect')"
    @refresh="loadProviders"
    @create="openCreateProvider"
    @edit="openEditProvider"
    @delete="confirmDeleteProvider"
    @toggle-field="handleToggleField"
    @toggle-type="handleToggleType"
    @reorder="handleReorderProviders"
  />
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import PaymentProviderList from '@/components/payment/PaymentProviderList.vue'

const { t } = useI18n()

type AnyHandler = (...args: any[]) => void

defineProps<{
  form: any
  providers: any[]
  providersLoading: boolean
  hasAnyPaymentTypeEnabled: boolean
  allPaymentTypes: Array<{ value: string; label: string }>
  loadProviders: AnyHandler
  openCreateProvider: AnyHandler
  openEditProvider: AnyHandler
  confirmDeleteProvider: AnyHandler
  handleToggleField: AnyHandler
  handleToggleType: AnyHandler
  handleReorderProviders: AnyHandler
}>()
</script>
