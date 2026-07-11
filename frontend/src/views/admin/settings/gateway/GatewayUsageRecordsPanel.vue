<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('admin.settings.usageRecords.title') }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.settings.usageRecords.description') }}
      </p>
    </div>
    <div class="space-y-4 p-6">
      <div class="flex items-center justify-between">
        <div>
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.user_error_view.label') }}
          </label>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.user_error_view.description') }}
          </p>
        </div>
        <label class="toggle">
          <input v-model="form.allow_user_view_error_requests" type="checkbox" />
          <span class="toggle-slider"></span>
        </label>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps<{
  form: {
    allow_user_view_error_requests: boolean
  }
}>()
</script>
