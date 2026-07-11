<template>
<!-- Web Search Emulation -->
<div class="card">
  <div
    class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
  >
    <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
      {{ t("admin.settings.webSearchEmulation.title") }}
    </h2>
    <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
      {{ t("admin.settings.webSearchEmulation.description") }}
    </p>
  </div>
  <div class="space-y-5 p-6">
    <!-- Global Toggle -->
    <div class="flex items-center justify-between">
      <div>
        <label
          class="text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          {{ t("admin.settings.webSearchEmulation.enabled") }}
        </label>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.webSearchEmulation.enabledHint") }}
        </p>
      </div>
      <Toggle v-model="webSearchConfig.enabled" />
    </div>

    <!-- Providers -->
    <div v-if="webSearchConfig.enabled" class="space-y-4">
      <div class="flex items-center justify-between">
        <label
          class="text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          {{ t("admin.settings.webSearchEmulation.providers") }}
        </label>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          @click="addWebSearchProvider"
        >
          {{ t("admin.settings.webSearchEmulation.addProvider") }}
        </button>
      </div>

      <div
        v-if="webSearchConfig.providers.length === 0"
        class="rounded-lg border border-dashed border-gray-300 p-4 text-center text-sm text-gray-400 dark:border-dark-600"
      >
        {{ t("admin.settings.webSearchEmulation.noProviders") }}
      </div>

      <div
        v-for="(provider, pIdx) in webSearchConfig.providers"
        :key="pIdx"
        class="rounded-lg border border-gray-200 dark:border-dark-600"
      >
        <!-- Collapsible header -->
        <div
          class="flex cursor-pointer items-center justify-between px-4 py-3"
          @click="toggleProviderExpand(pIdx)"
        >
          <div class="flex items-center gap-3">
            <svg
              class="h-4 w-4 text-gray-400 transition-transform"
              :class="{ 'rotate-90': expandedProviders[pIdx] }"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M9 5l7 7-7 7"
              />
            </svg>
            <Select
              v-model="provider.type"
              :options="[
                { value: 'brave', label: 'Brave Search' },
                { value: 'tavily', label: 'Tavily' },
              ]"
              class="w-36"
              @click.stop
            />
            <!-- Quota summary (always visible) -->
            <span class="text-xs text-gray-400">
              {{ provider.quota_used ?? 0 }} /
              {{
                provider.quota_limit != null &&
                provider.quota_limit > 0
                  ? provider.quota_limit
                  : "∞"
              }}
            </span>
            <span
              v-if="
                !expandedProviders[pIdx] &&
                provider.api_key_configured
              "
              class="text-xs text-green-500"
            >
              {{
                t(
                  "admin.settings.webSearchEmulation.apiKeyConfigured",
                )
              }}
            </span>
          </div>
          <button
            type="button"
            class="text-red-500 hover:text-red-700 text-xs"
            @click.stop="removeWebSearchProvider(pIdx)"
          >
            {{
              t("admin.settings.webSearchEmulation.removeProvider")
            }}
          </button>
        </div>

        <!-- Expanded content -->
        <div
          v-if="expandedProviders[pIdx]"
          class="space-y-3 border-t border-gray-100 px-4 pb-4 pt-3 dark:border-dark-700"
        >
          <!-- API Key with inline show/copy -->
          <div>
            <label class="text-xs text-gray-500">{{
              t("admin.settings.webSearchEmulation.apiKey")
            }}</label>
            <div class="relative">
              <input
                v-model="provider.api_key"
                :type="apiKeyVisible[pIdx] ? 'text' : 'password'"
                class="input w-full text-sm"
                :class="
                  provider.api_key || provider.api_key_configured
                    ? 'pr-16'
                    : ''
                "
                :placeholder="
                  provider.api_key_configured
                    ? '••••••••'
                    : t(
                        'admin.settings.webSearchEmulation.apiKeyPlaceholder',
                      )
                "
              />
              <div
                v-if="provider.api_key || provider.api_key_configured"
                class="absolute inset-y-0 right-0 flex items-center pr-1.5"
              >
                <button
                  type="button"
                  class="rounded p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                  :title="
                    apiKeyVisible[pIdx]
                      ? t(
                          'admin.settings.webSearchEmulation.hideApiKey',
                        )
                      : t(
                          'admin.settings.webSearchEmulation.showApiKey',
                        )
                  "
                  @click="apiKeyVisible[pIdx] = !apiKeyVisible[pIdx]"
                >
                  <svg
                    v-if="!apiKeyVisible[pIdx]"
                    class="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                    />
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                    />
                  </svg>
                  <svg
                    v-else
                    class="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.878 9.878L3 3m6.878 6.878L21 21"
                    />
                  </svg>
                </button>
                <button
                  type="button"
                  class="rounded p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                  :class="{
                    'opacity-30 cursor-not-allowed':
                      !provider.api_key,
                  }"
                  :title="
                    t('admin.settings.webSearchEmulation.copyApiKey')
                  "
                  :disabled="!provider.api_key"
                  @click="copyApiKey(pIdx)"
                >
                  <svg
                    class="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                    />
                  </svg>
                </button>
              </div>
            </div>
          </div>

          <!-- Quota + Subscription in compact row -->
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="text-xs text-gray-500">{{
                t("admin.settings.webSearchEmulation.quotaLimit")
              }}</label>
              <input
                v-model="provider.quota_limit"
                type="number"
                min="1"
                class="input text-sm"
                :placeholder="'∞'"
              />
              <p class="mt-0.5 text-xs text-gray-400">
                {{
                  t(
                    "admin.settings.webSearchEmulation.quotaLimitHint",
                  )
                }}
              </p>
            </div>
            <div>
              <label class="text-xs text-gray-500">{{
                t("admin.settings.webSearchEmulation.subscribedAt")
              }}</label>
              <input
                :value="formatSubscribedAt(provider.subscribed_at)"
                type="date"
                class="input text-sm"
                @input="
                  provider.subscribed_at = parseSubscribedAt(
                    ($event.target as HTMLInputElement).value,
                  )
                "
              />
              <p class="mt-0.5 text-xs text-gray-400">
                {{
                  t(
                    "admin.settings.webSearchEmulation.subscribedAtHint",
                  )
                }}
              </p>
            </div>
          </div>

          <!-- Usage display -->
          <div class="flex items-center gap-2">
            <span class="text-xs text-gray-500"
              >{{
                t("admin.settings.webSearchEmulation.quotaUsage")
              }}:</span
            >
            <div
              v-if="
                provider.quota_limit != null &&
                provider.quota_limit > 0
              "
              class="flex-1 rounded-full bg-gray-200 dark:bg-dark-600"
              style="height: 6px"
            >
              <div
                class="h-full rounded-full transition-all"
                :class="
                  quotaPercentage(provider) > 90
                    ? 'bg-red-500'
                    : quotaPercentage(provider) > 70
                      ? 'bg-yellow-500'
                      : 'bg-green-500'
                "
                :style="{
                  width:
                    Math.min(quotaPercentage(provider), 100) + '%',
                }"
              />
            </div>
            <div v-else class="flex-1" />
            <span class="text-xs text-gray-500"
              >{{ provider.quota_used ?? 0 }} /
              {{
                provider.quota_limit != null &&
                provider.quota_limit > 0
                  ? provider.quota_limit
                  : "∞"
              }}</span
            >
            <button
              v-if="(provider.quota_used ?? 0) > 0"
              type="button"
              class="text-xs text-primary-600 hover:text-primary-700"
              @click="resetWebSearchUsage(pIdx)"
            >
              {{ t("admin.settings.webSearchEmulation.resetUsage") }}
            </button>
          </div>

          <!-- Proxy + Test on same row -->
          <div class="flex items-end gap-3">
            <div class="flex-1">
              <label class="text-xs text-gray-500">{{
                t("admin.settings.webSearchEmulation.proxy")
              }}</label>
              <ProxySelector
                v-model="provider.proxy_id"
                :proxies="webSearchProxies"
              />
            </div>
            <button
              type="button"
              class="btn btn-secondary btn-sm whitespace-nowrap"
              @click="openTestDialog()"
            >
              {{ t("admin.settings.webSearchEmulation.test") }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import Select from '@/components/common/Select.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'

const { t } = useI18n()

type AnyHandler = (...args: any[]) => void

defineProps<{
  webSearchConfig: { enabled: boolean; providers: any[] }
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
