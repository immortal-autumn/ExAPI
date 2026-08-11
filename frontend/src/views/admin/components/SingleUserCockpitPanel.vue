<template>
  <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
    <section class="card p-4 xl:col-span-2">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <div class="flex items-center gap-2">
            <Icon name="fire" size="md" class="text-orange-500" />
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">单用户控制台</h2>
          </div>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            用于监控额度、检查账号可调度状态和切换上游账号的私有网关控制台。
          </p>
        </div>
        <div class="flex gap-2">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" aria-label="刷新控制台摘要" @click="loadSummary">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span>刷新</span>
          </button>
          <button class="btn btn-primary btn-sm" @click="router.push('/admin/accounts')">
            <Icon name="server" size="sm" />
            <span>账号管理</span>
          </button>
        </div>
      </div>

      <div
        v-if="loading && !summary"
        class="flex min-h-52 items-center justify-center rounded-lg border border-gray-200 text-sm text-gray-500 dark:border-gray-700 dark:text-gray-400"
        role="status"
        aria-live="polite"
      >
        正在加载账号摘要…
      </div>
      <div
        v-else-if="error || !summary"
        class="flex min-h-52 flex-col items-center justify-center gap-3 rounded-lg border border-red-200 bg-red-50 p-6 text-center dark:border-red-900/60 dark:bg-red-950/20"
        role="alert"
      >
        <Icon name="exclamationTriangle" size="lg" class="text-red-500" />
        <div>
          <p class="font-medium text-red-800 dark:text-red-200">账号摘要不可用</p>
          <p class="mt-1 text-sm text-red-700 dark:text-red-300">{{ error || '无法读取控制台摘要。' }}</p>
        </div>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="loadSummary">
          重试
        </button>
      </div>

      <template v-else>
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-700 dark:bg-dark-800/60">
          <p class="text-xs text-gray-500 dark:text-gray-400">账号</p>
          <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ summary.accounts.total }}</p>
          <p class="text-xs text-green-600 dark:text-green-400">{{ summary.accounts.active }} 个正常</p>
        </div>
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-700 dark:bg-dark-800/60">
          <p class="text-xs text-gray-500 dark:text-gray-400">可调度</p>
          <p class="mt-1 text-2xl font-bold text-violet-600 dark:text-violet-400">{{ summary.accounts.dispatch_eligible }}</p>
          <p class="text-xs text-gray-500 dark:text-gray-400">当前符合调度条件</p>
        </div>
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-700 dark:bg-dark-800/60">
          <p class="text-xs text-gray-500 dark:text-gray-400">额度预警</p>
          <p class="mt-1 text-2xl font-bold text-amber-600 dark:text-amber-400">{{ summary.accounts.quota_warning_total }}</p>
          <p class="text-xs text-gray-500 dark:text-gray-400">已使用配置限额的 70% 以上</p>
        </div>
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-700 dark:bg-dark-800/60">
          <p class="text-xs text-gray-500 dark:text-gray-400">异常账号</p>
          <p class="mt-1 text-2xl font-bold" :class="summary.accounts.error ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'">
            {{ summary.accounts.error }}
          </p>
          <p class="text-xs text-gray-500 dark:text-gray-400">需要处理</p>
        </div>
      </div>

      <div class="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div class="rounded-lg border border-gray-200 p-3 dark:border-gray-700">
          <div class="mb-3 flex items-center justify-between">
            <h3 class="text-xs font-semibold tracking-wide text-gray-500 dark:text-gray-400">额度监控</h3>
            <button class="text-xs text-primary-600 hover:text-primary-500 dark:text-primary-400" @click="router.push('/admin/accounts')">
              管理限额
            </button>
          </div>
          <div v-if="quotaWarnings.length" class="space-y-3">
            <button
              v-for="warning in quotaWarnings"
              :key="warning.account_id"
              type="button"
              class="block w-full space-y-1 rounded-md text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
              :aria-label="`查看账号 ${warning.name}，额度使用 ${warning.percent}%`"
              @click="goToAccount(warning.account_id)"
            >
              <div class="flex items-center justify-between gap-3 text-sm">
                <span class="truncate font-medium text-gray-900 dark:text-white">{{ warning.name }}</span>
                <span :class="quotaTextClass(warning.severity)">{{ warning.percent }}%</span>
              </div>
              <div class="h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700">
                <div class="h-full rounded-full" :class="quotaBarClass(warning.severity)" :style="{ width: `${Math.min(warning.percent, 100)}%` }" />
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ warning.platform }} · {{ warning.scope }} · ${{ formatCost(warning.used) }} / ${{ formatCost(warning.limit) }}
              </p>
            </button>
          </div>
          <p v-else class="text-sm text-gray-500 dark:text-gray-400">
            暂无用量超过 70% 的额度周期。请前往账号管理为各账号设置限额。
          </p>
        </div>

        <div class="rounded-lg border border-gray-200 p-3 dark:border-gray-700">
          <div class="mb-3 flex items-center justify-between">
            <h3 class="text-xs font-semibold tracking-wide text-gray-500 dark:text-gray-400">上游账号调度</h3>
            <button class="text-xs text-primary-600 hover:text-primary-500 dark:text-primary-400" @click="router.push('/admin/accounts')">
              查看账号列表
            </button>
          </div>
          <div v-if="summary.platforms.length" class="space-y-2">
            <div v-for="platform in summary.platforms" :key="platform.platform" class="flex items-center justify-between rounded-md bg-gray-50 px-3 py-2 text-sm dark:bg-dark-800/60">
              <div>
                <p class="font-medium capitalize text-gray-900 dark:text-white">{{ platform.platform }}</p>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ platform.dispatch_eligible }} 个可调度 · {{ platform.error }} 个异常</p>
              </div>
              <div class="text-right">
                <p class="font-semibold text-gray-900 dark:text-white">{{ platform.active }}/{{ platform.total }}</p>
                <p class="text-xs text-gray-500 dark:text-gray-400">正常</p>
              </div>
            </div>
          </div>
          <p v-else class="text-sm text-gray-500 dark:text-gray-400">
            尚未配置上游账号。
          </p>
        </div>
      </div>
      <p class="sr-only" role="status" aria-live="polite">{{ loadAnnouncement }}</p>
      </template>
    </section>

    <section class="card p-4">
      <div class="mb-4 flex items-center gap-2">
        <Icon name="terminal" size="md" class="text-primary-500" />
        <div>
          <h2 class="text-sm font-semibold text-gray-900 dark:text-white">本地集成</h2>
          <p class="text-xs text-gray-500 dark:text-gray-400">复制供 IDE、本地工具及 WireGuard 管理使用的端点。</p>
        </div>
      </div>

      <div class="space-y-3">
        <IntegrationCopyRow label="公网 AI 基础地址" :value="links.publicGatewayBaseURL" @copy="copyValue" />
        <IntegrationCopyRow label="WireGuard 控制台" :value="links.privateControlPanelURL" @copy="copyValue" />
        <IntegrationCopyRow label="本地控制台" :value="links.localControlPanelURL" @copy="copyValue" />
      </div>

      <div class="mt-4 rounded-lg border border-violet-200 bg-violet-50 p-3 text-sm dark:border-violet-800/60 dark:bg-violet-900/20">
        <div class="flex items-start gap-2">
          <Icon name="bolt" size="sm" class="mt-0.5 text-violet-600 dark:text-violet-300" />
          <div>
            <p class="font-medium text-violet-900 dark:text-violet-100">定时检测建议</p>
            <p class="mt-1 text-xs text-violet-700 dark:text-violet-200">
              可将定时账号测试用于低频健康检查。建议限制执行频率与 Token 用量；可调度统计不包含瞬时并发饱和状态。
            </p>
            <button class="mt-2 text-xs font-medium text-violet-700 hover:text-violet-600 dark:text-violet-200" @click="router.push('/admin/accounts')">
              按账号配置 →
            </button>
          </div>
        </div>
      </div>

      <p class="mt-3 min-h-4 text-xs text-green-600 dark:text-green-400" role="status" aria-live="polite">
        {{ copiedLabel ? `已复制：${copiedLabel}` : '' }}
      </p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { operatorAPI as adminAPI } from '@/api/operator'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import type { CockpitQuotaSeverity, CockpitSummaryResponse } from '@/api/operator'
import {
  getLocalIntegrationLinks,
} from '@/utils/singleUserCockpit'

const router = useRouter()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()
const summary = ref<CockpitSummaryResponse | null>(null)
const loading = ref(false)
const error = ref('')
const copiedLabel = ref('')
const loadAnnouncement = ref('')
let copyTimer: ReturnType<typeof setTimeout> | null = null

const links = computed(() => getLocalIntegrationLinks({
  origin: typeof window !== 'undefined' ? window.location.origin : undefined,
  apiBaseUrl: appStore.apiBaseUrl,
}))

const quotaWarnings = computed(() => summary.value?.quota_warnings.slice(0, 6) ?? [])

async function loadSummary() {
  loading.value = true
  error.value = ''
  try {
    summary.value = await adminAPI.cockpit.getSummary()
    loadAnnouncement.value = `账号摘要已更新，共 ${summary.value.accounts.total} 个账号。`
  } catch (err) {
    console.error('Failed to load cockpit summary:', err)
    summary.value = null
    error.value = '加载账号摘要失败，请检查服务状态后重试。'
    loadAnnouncement.value = error.value
  } finally {
    loading.value = false
  }
}

async function copyValue(value: string, label: string) {
  const copied = await copyToClipboard(value)
  if (copied) {
    copiedLabel.value = label
    if (copyTimer) clearTimeout(copyTimer)
    copyTimer = setTimeout(() => {
      copiedLabel.value = ''
    }, 2000)
  }
}

function goToAccount(accountID: number) {
  void router.push({ path: '/admin/accounts', query: { account_id: String(accountID) } })
}

function formatCost(value: number): string {
  if (value >= 1) return value.toFixed(2)
  if (value > 0) return value.toFixed(4)
  return '0.00'
}

function quotaTextClass(severity: CockpitQuotaSeverity): string {
  switch (severity) {
    case 'critical':
      return 'font-semibold text-red-600 dark:text-red-400'
    case 'warning':
      return 'font-semibold text-amber-600 dark:text-amber-400'
    default:
      return 'font-semibold text-gray-500 dark:text-gray-400'
  }
}

function quotaBarClass(severity: CockpitQuotaSeverity): string {
  switch (severity) {
    case 'critical':
      return 'bg-red-500'
    case 'warning':
      return 'bg-amber-500'
    default:
      return 'bg-gray-400'
  }
}

const IntegrationCopyRow = defineComponent({
  name: 'IntegrationCopyRow',
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
  },
  emits: ['copy'],
  setup(props, { emit }) {
    return () => h('div', { class: 'rounded-lg border border-gray-200 p-3 dark:border-gray-700' }, [
      h('p', { class: 'text-xs font-medium text-gray-500 dark:text-gray-400' }, props.label),
      h('div', { class: 'mt-2 flex items-center gap-2' }, [
        h('code', { class: 'min-w-0 flex-1 truncate rounded bg-gray-100 px-2 py-1 text-xs text-gray-800 dark:bg-dark-800 dark:text-gray-100', title: props.value }, props.value),
        h('button', {
          class: 'btn btn-secondary btn-sm px-2',
          type: 'button',
          'aria-label': `复制${props.label}`,
          onClick: () => emit('copy', props.value, props.label),
        }, '复制'),
      ]),
    ])
  },
})

onMounted(() => {
  void loadSummary()
})

onUnmounted(() => {
  if (copyTimer) clearTimeout(copyTimer)
})
</script>
