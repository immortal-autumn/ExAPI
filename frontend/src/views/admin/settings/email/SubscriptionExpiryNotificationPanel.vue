<template>
<!-- 订阅到期提醒 -->
<div class="card">
  <div
    class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
  >
    <h3 class="text-base font-medium text-gray-900 dark:text-white">
      {{ t("admin.settings.subscriptionExpiryNotify.title") }}
    </h3>
    <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
      {{ t("admin.settings.subscriptionExpiryNotify.description") }}
    </p>
  </div>
  <div class="px-6 py-6">
    <div class="flex items-center justify-between gap-4">
      <div>
        <label
          class="mb-0 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          {{ t("admin.settings.subscriptionExpiryNotify.enabled") }}
        </label>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.subscriptionExpiryNotify.enabledHint") }}
        </p>
      </div>
      <Toggle v-model="form.subscription_expiry_notify_enabled" />
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
}>()
</script>
