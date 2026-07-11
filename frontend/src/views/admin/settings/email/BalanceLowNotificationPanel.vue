<template>
<!-- Balance Low Notification -->
<div class="card">
  <div
    class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
  >
    <h3 class="text-base font-medium text-gray-900 dark:text-white">
      {{ t("admin.settings.balanceNotify.title") }}
    </h3>
    <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
      {{ t("admin.settings.balanceNotify.description") }}
    </p>
  </div>
  <div class="px-6 py-6 space-y-4">
    <div class="flex items-center justify-between">
      <label
        class="mb-0 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >{{ t("admin.settings.balanceNotify.enabled") }}</label
      >
      <Toggle v-model="form.balance_low_notify_enabled" />
    </div>
    <div v-if="form.balance_low_notify_enabled">
      <label
        class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >{{ t("admin.settings.balanceNotify.threshold") }}</label
      >
      <div class="relative">
        <span
          class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
          >$</span
        >
        <input
          v-model.number="form.balance_low_notify_threshold"
          type="number"
          min="0"
          step="0.01"
          class="input pl-7"
        />
      </div>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t("admin.settings.balanceNotify.thresholdHint") }}
      </p>
    </div>
    <div>
      <label
        class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >{{ t("admin.settings.balanceNotify.rechargeUrl") }}</label
      >
      <input
        v-model="form.balance_low_notify_recharge_url"
        type="url"
        class="input"
        :placeholder="currentOrigin"
      />
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t("admin.settings.balanceNotify.rechargeUrlHint") }}
      </p>
    </div>
  </div>
</div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'

const { t } = useI18n()

defineProps<{
  form: any
  currentOrigin: string
}>()
</script>
