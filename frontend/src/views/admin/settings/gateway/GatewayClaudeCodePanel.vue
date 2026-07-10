<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t("admin.settings.claudeCode.title") }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t("admin.settings.claudeCode.description") }}
      </p>
    </div>
    <div class="p-6">
      <div>
        <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t("admin.settings.claudeCode.minVersion") }}
        </label>
        <input
          v-model="form.min_claude_code_version"
          type="text"
          class="input max-w-xs font-mono text-sm"
          :placeholder="t('admin.settings.claudeCode.minVersionPlaceholder')"
        />
        <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.claudeCode.minVersionHint") }}
        </p>
      </div>
      <div class="mt-4">
        <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t("admin.settings.claudeCode.maxVersion") }}
        </label>
        <input
          v-model="form.max_claude_code_version"
          type="text"
          class="input max-w-xs font-mono text-sm"
          :placeholder="t('admin.settings.claudeCode.maxVersionPlaceholder')"
        />
        <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.claudeCode.maxVersionHint") }}
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps<{
  form: {
    min_claude_code_version: string
    max_claude_code_version: string
  }
}>()
</script>
