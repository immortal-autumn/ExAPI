<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t("admin.settings.scheduling.title") }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t("admin.settings.scheduling.description") }}
      </p>
    </div>
    <div class="space-y-5 p-6">
      <div class="flex items-center justify-between">
        <div>
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t("admin.settings.scheduling.allowUngroupedKey") }}
          </label>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t("admin.settings.scheduling.allowUngroupedKeyHint") }}
          </p>
        </div>
        <Toggle v-model="form.allow_ungrouped_key_scheduling" />
      </div>

      <div class="flex items-center justify-between">
        <div>
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t("admin.settings.openaiExperimentalScheduler.title") }}
          </label>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t("admin.settings.openaiExperimentalScheduler.description") }}
          </p>
        </div>
        <Toggle v-model="form.openai_advanced_scheduler_enabled" />
      </div>

      <div
        v-if="form.openai_advanced_scheduler_enabled"
        class="flex items-center justify-between border-t border-gray-100 pt-5 dark:border-dark-700"
      >
        <div>
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t("admin.settings.openaiExperimentalScheduler.stickyWeightedTitle") }}
          </label>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t("admin.settings.openaiExperimentalScheduler.stickyWeightedDescription") }}
          </p>
        </div>
        <Toggle v-model="form.openai_advanced_scheduler_sticky_weighted_enabled" />
      </div>

      <div
        v-if="form.openai_advanced_scheduler_enabled"
        class="flex items-center justify-between border-t border-gray-100 pt-5 dark:border-dark-700"
      >
        <div>
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t("admin.settings.openaiExperimentalScheduler.subscriptionPriorityTitle") }}
          </label>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t("admin.settings.openaiExperimentalScheduler.subscriptionPriorityDescription") }}
          </p>
        </div>
        <Toggle v-model="form.openai_advanced_scheduler_subscription_priority_enabled" />
      </div>

      <div
        v-if="form.openai_advanced_scheduler_enabled"
        class="border-t border-gray-100 pt-5 dark:border-dark-700"
      >
        <div>
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t("admin.settings.openaiExperimentalScheduler.weightsTitle") }}
          </label>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t("admin.settings.openaiExperimentalScheduler.weightsDescription") }}
          </p>
        </div>

        <div class="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-5">
          <label v-for="field in openAIAdvancedSchedulerWeightFields" :key="field.key" class="block">
            <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ field.label }}
            </span>
            <input
              v-model="form[field.key]"
              class="input mt-1"
              inputmode="decimal"
              :placeholder="field.placeholder"
              type="text"
            />
          </label>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'

const { t } = useI18n()

defineProps<{
  form: Record<string, any> & {
    allow_ungrouped_key_scheduling: boolean
    openai_advanced_scheduler_enabled: boolean
    openai_advanced_scheduler_sticky_weighted_enabled: boolean
    openai_advanced_scheduler_subscription_priority_enabled: boolean
  }
  openAIAdvancedSchedulerWeightFields: Array<{
    key: string
    label: string
    placeholder: string
  }>
}>()
</script>
