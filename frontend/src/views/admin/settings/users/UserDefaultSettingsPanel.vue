<template>
<!-- Default Settings -->
<div class="card">
  <div
    class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
  >
    <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
      {{ t("admin.settings.defaults.title") }}
    </h2>
    <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
      {{ t("admin.settings.defaults.description") }}
    </p>
  </div>
  <div class="space-y-6 p-6">
    <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
      <div>
        <label
          class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          {{ t("admin.settings.defaults.defaultBalance") }}
        </label>
        <input
          v-model.number="form.default_balance"
          type="number"
          step="0.01"
          min="0"
          class="input"
          placeholder="0.00"
        />
        <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.defaults.defaultBalanceHint") }}
        </p>
      </div>
      <div>
        <label
          class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          {{ t("admin.settings.defaults.defaultConcurrency") }}
        </label>
        <input
          v-model.number="form.default_concurrency"
          type="number"
          min="1"
          class="input"
          placeholder="1"
        />
        <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.defaults.defaultConcurrencyHint") }}
        </p>
      </div>
      <div>
        <label
          class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          {{ t("admin.settings.defaults.defaultUserRpmLimit") }}
        </label>
        <input
          v-model.number="form.default_user_rpm_limit"
          type="number"
          min="0"
          step="1"
          class="input"
          placeholder="0"
        />
        <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.defaults.defaultUserRpmLimitHint") }}
        </p>
      </div>
    </div>

    <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
      <div class="mb-3 flex items-center justify-between">
        <div>
          <label class="font-medium text-gray-900 dark:text-white">
            {{ t("admin.settings.defaults.defaultSubscriptions") }}
          </label>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {{
              t("admin.settings.defaults.defaultSubscriptionsHint")
            }}
          </p>
        </div>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          @click="addDefaultSubscription"
          :disabled="subscriptionGroups.length === 0"
        >
          {{ t("admin.settings.defaults.addDefaultSubscription") }}
        </button>
      </div>

      <div
        v-if="form.default_subscriptions.length === 0"
        class="rounded border border-dashed border-gray-300 px-4 py-3 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
      >
        {{ t("admin.settings.defaults.defaultSubscriptionsEmpty") }}
      </div>

      <div v-else class="space-y-3">
        <div
          v-for="(item, index) in form.default_subscriptions"
          :key="`default-sub-${index}`"
          class="grid grid-cols-1 gap-3 rounded border border-gray-200 p-3 md:grid-cols-[1fr_160px_auto] dark:border-dark-600"
        >
          <div>
            <label
              class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400"
            >
              {{ t("admin.settings.defaults.subscriptionGroup") }}
            </label>
            <Select
              v-model="item.group_id"
              class="default-sub-group-select"
              :options="defaultSubscriptionGroupOptions"
              :placeholder="
                t('admin.settings.defaults.subscriptionGroup')
              "
            >
              <template #selected="{ option }">
                <GroupBadge
                  v-if="option"
                  :name="
                    (
                      option as unknown as any
                    ).label
                  "
                  :platform="
                    (
                      option as unknown as any
                    ).platform
                  "
                  :subscription-type="
                    (
                      option as unknown as any
                    ).subscriptionType
                  "
                  :rate-multiplier="
                    (
                      option as unknown as any
                    ).rate
                  "
                />
                <span v-else class="text-gray-400">
                  {{ t("admin.settings.defaults.subscriptionGroup") }}
                </span>
              </template>
              <template #option="{ option, selected }">
                <GroupOptionItem
                  :name="
                    (
                      option as unknown as any
                    ).label
                  "
                  :platform="
                    (
                      option as unknown as any
                    ).platform
                  "
                  :subscription-type="
                    (
                      option as unknown as any
                    ).subscriptionType
                  "
                  :rate-multiplier="
                    (
                      option as unknown as any
                    ).rate
                  "
                  :description="
                    (
                      option as unknown as any
                    ).description
                  "
                  :selected="selected"
                />
              </template>
            </Select>
          </div>
          <div>
            <label
              class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400"
            >
              {{
                t("admin.settings.defaults.subscriptionValidityDays")
              }}
            </label>
            <input
              v-model.number="item.validity_days"
              type="number"
              min="1"
              max="36500"
              class="input h-[42px]"
            />
          </div>
          <div class="flex items-end">
            <button
              type="button"
              class="btn btn-secondary default-sub-delete-btn w-full text-red-600 hover:text-red-700 dark:text-red-400"
              @click="removeDefaultSubscription(index)"
            >
              {{ t("common.delete") }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- ★ 新增：系统全局默认平台限额矩阵 -->
    <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
      <div class="mb-3">
        <label class="font-medium text-gray-900 dark:text-white">
          {{ t("admin.settings.defaults.defaultPlatformQuotas") }}
        </label>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.defaults.defaultPlatformQuotasHint") }}
        </p>
        <p class="mt-0.5 text-xs text-amber-600 dark:text-amber-400">
          {{ t("admin.settings.defaults.platformQuotaNotice") }}
        </p>
      </div>
      <div class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead>
            <tr class="text-left text-xs text-gray-500 dark:text-gray-400">
              <th class="pb-2 pr-4 font-medium">{{ t("admin.settings.platformQuota.platform") }}</th>
              <th class="pb-2 pr-4 font-medium">{{ t("admin.settings.platformQuota.daily") }}</th>
              <th class="pb-2 pr-4 font-medium">{{ t("admin.settings.platformQuota.weekly") }}</th>
              <th class="pb-2 font-medium">{{ t("admin.settings.platformQuota.monthly") }}</th>
            </tr>
          </thead>
          <tbody class="space-y-2">
            <tr v-for="p in (['anthropic', 'openai', 'gemini', 'antigravity', 'grok'] as const)" :key="p" class="align-top">
              <td class="pr-4 py-1">
                <span class="font-mono text-xs text-gray-700 dark:text-gray-300">{{ p }}</span>
              </td>
              <td class="pr-4 py-1">
                <input
                  v-model.number="form.default_platform_quotas[p]!.daily"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input h-8 w-28 text-sm"
                  :placeholder="t('admin.settings.platformQuota.placeholder')"
                />
              </td>
              <td class="pr-4 py-1">
                <input
                  v-model.number="form.default_platform_quotas[p]!.weekly"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input h-8 w-28 text-sm"
                  :placeholder="t('admin.settings.platformQuota.placeholder')"
                />
              </td>
              <td class="py-1">
                <input
                  v-model.number="form.default_platform_quotas[p]!.monthly"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input h-8 w-28 text-sm"
                  :placeholder="t('admin.settings.platformQuota.placeholder')"
                />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <!-- /全局平台限额矩阵 -->
  </div>
</div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'

const { t } = useI18n()

type AnyHandler = (...args: any[]) => void

defineProps<{
  form: any
  subscriptionGroups: any[]
  defaultSubscriptionGroupOptions: any[]
  addDefaultSubscription: AnyHandler
  removeDefaultSubscription: AnyHandler
}>()
</script>
