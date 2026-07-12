<template>
  <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
    <section class="card p-4 xl:col-span-2">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <div class="flex items-center gap-2">
            <Icon name="fire" size="md" class="text-orange-500" />
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">Single-user cockpit</h2>
          </div>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            Private gateway control plane for quota monitoring, wakeup readiness, and upstream account switching.
          </p>
        </div>
        <div class="flex gap-2">
          <button class="btn btn-secondary btn-sm" :disabled="loading" @click="loadAccounts">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span>Refresh</span>
          </button>
          <button class="btn btn-primary btn-sm" @click="router.push('/admin/accounts')">
            <Icon name="server" size="sm" />
            <span>Accounts</span>
          </button>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-700 dark:bg-dark-800/60">
          <p class="text-xs text-gray-500 dark:text-gray-400">Accounts</p>
          <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ summary.totalAccounts }}</p>
          <p class="text-xs text-green-600 dark:text-green-400">{{ summary.activeAccounts }} active</p>
        </div>
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-700 dark:bg-dark-800/60">
          <p class="text-xs text-gray-500 dark:text-gray-400">Wakeup ready</p>
          <p class="mt-1 text-2xl font-bold text-violet-600 dark:text-violet-400">{{ summary.wakeupReadyAccounts }}</p>
          <p class="text-xs text-gray-500 dark:text-gray-400">active + schedulable</p>
        </div>
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-700 dark:bg-dark-800/60">
          <p class="text-xs text-gray-500 dark:text-gray-400">Quota watch</p>
          <p class="mt-1 text-2xl font-bold text-amber-600 dark:text-amber-400">{{ summary.quotaWatchAccounts.length }}</p>
          <p class="text-xs text-gray-500 dark:text-gray-400">≥70% configured limit</p>
        </div>
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-700 dark:bg-dark-800/60">
          <p class="text-xs text-gray-500 dark:text-gray-400">Errors</p>
          <p class="mt-1 text-2xl font-bold" :class="summary.errorAccounts ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'">
            {{ summary.errorAccounts }}
          </p>
          <p class="text-xs text-gray-500 dark:text-gray-400">needs attention</p>
        </div>
      </div>

      <div class="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div class="rounded-lg border border-gray-200 p-3 dark:border-gray-700">
          <div class="mb-3 flex items-center justify-between">
            <h3 class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">Quota monitoring</h3>
            <button class="text-xs text-primary-600 hover:text-primary-500 dark:text-primary-400" @click="router.push('/admin/accounts')">
              Manage limits
            </button>
          </div>
          <div v-if="summary.quotaWatchAccounts.length" class="space-y-3">
            <div v-for="acct in summary.quotaWatchAccounts" :key="acct.id" class="space-y-1">
              <div class="flex items-center justify-between gap-3 text-sm">
                <span class="truncate font-medium text-gray-900 dark:text-white">{{ acct.name }}</span>
                <span :class="quotaTextClass(acct.quotaState.severity)">{{ acct.quotaState.percent }}%</span>
              </div>
              <div class="h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700">
                <div class="h-full rounded-full" :class="quotaBarClass(acct.quotaState.severity)" :style="{ width: `${Math.min(acct.quotaState.percent, 100)}%` }" />
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ acct.platform }} · {{ acct.quotaState.scope }} · ${{ formatCost(acct.quotaState.used) }} / ${{ formatCost(acct.quotaState.limit) }}
              </p>
            </div>
          </div>
          <p v-else class="text-sm text-gray-500 dark:text-gray-400">
            No configured quota window is above 70%. Add per-account limits on the Accounts page to make this card more useful.
          </p>
        </div>

        <div class="rounded-lg border border-gray-200 p-3 dark:border-gray-700">
          <div class="mb-3 flex items-center justify-between">
            <h3 class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">Upstream account routing</h3>
            <button class="text-xs text-primary-600 hover:text-primary-500 dark:text-primary-400" @click="router.push('/admin/accounts')">
              Open table
            </button>
          </div>
          <div v-if="summary.platforms.length" class="space-y-2">
            <div v-for="platform in summary.platforms" :key="platform.platform" class="flex items-center justify-between rounded-md bg-gray-50 px-3 py-2 text-sm dark:bg-dark-800/60">
              <div>
                <p class="font-medium capitalize text-gray-900 dark:text-white">{{ platform.platform }}</p>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ platform.schedulable }} wakeup-ready · {{ platform.error }} errors</p>
              </div>
              <div class="text-right">
                <p class="font-semibold text-gray-900 dark:text-white">{{ platform.active }}/{{ platform.total }}</p>
                <p class="text-xs text-gray-500 dark:text-gray-400">active</p>
              </div>
            </div>
          </div>
          <p v-else class="text-sm text-gray-500 dark:text-gray-400">
            No upstream accounts configured yet.
          </p>
        </div>
      </div>
    </section>

    <section class="card p-4">
      <div class="mb-4 flex items-center gap-2">
        <Icon name="terminal" size="md" class="text-primary-500" />
        <div>
          <h2 class="text-sm font-semibold text-gray-900 dark:text-white">Local integration</h2>
          <p class="text-xs text-gray-500 dark:text-gray-400">Copy endpoints for IDEs, local tools, and WireGuard admin.</p>
        </div>
      </div>

      <div class="space-y-3">
        <IntegrationCopyRow label="Public AI base URL" :value="links.publicGatewayBaseURL" @copy="copyValue" />
        <IntegrationCopyRow label="WireGuard control panel" :value="links.privateControlPanelURL" @copy="copyValue" />
        <IntegrationCopyRow label="Local control panel" :value="links.localControlPanelURL" @copy="copyValue" />
      </div>

      <div class="mt-4 rounded-lg border border-violet-200 bg-violet-50 p-3 text-sm dark:border-violet-800/60 dark:bg-violet-900/20">
        <div class="flex items-start gap-2">
          <Icon name="bolt" size="sm" class="mt-0.5 text-violet-600 dark:text-violet-300" />
          <div>
            <p class="font-medium text-violet-900 dark:text-violet-100">Wakeup task heuristic</p>
            <p class="mt-1 text-xs text-violet-700 dark:text-violet-200">
              Use scheduled account tests as lightweight wakeup tasks. Keep them low-frequency and token-capped; this panel counts accounts that are active and schedulable.
            </p>
            <button class="mt-2 text-xs font-medium text-violet-700 hover:text-violet-600 dark:text-violet-200" @click="router.push('/admin/accounts')">
              Configure per account →
            </button>
          </div>
        </div>
      </div>

      <p v-if="copiedLabel" class="mt-3 text-xs text-green-600 dark:text-green-400">Copied {{ copiedLabel }}</p>
      <p v-if="error" class="mt-3 text-xs text-red-600 dark:text-red-400">{{ error }}</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { adminAPI } from '@/api/admin'
import Icon from '@/components/icons/Icon.vue'
import type { Account } from '@/types'
import {
  buildSingleUserCockpitSummary,
  getLocalIntegrationLinks,
  type QuotaSeverity,
} from '@/utils/singleUserCockpit'

const router = useRouter()
const accounts = ref<Account[]>([])
const loading = ref(false)
const error = ref('')
const copiedLabel = ref('')
let copyTimer: ReturnType<typeof setTimeout> | null = null

const links = computed(() => getLocalIntegrationLinks({
  origin: typeof window !== 'undefined' ? window.location.origin : undefined,
  publicHost: 'sub2api.research.for-immortal.cn',
  wireGuardURL: 'http://100.97.17.1:8027',
}))

const summary = computed(() => buildSingleUserCockpitSummary(accounts.value))

async function loadAccounts() {
  loading.value = true
  error.value = ''
  try {
    const response = await adminAPI.accounts.list(1, 200, {
      include_scheduler_score: 'true',
      sort_by: 'name',
      sort_order: 'asc',
    })
    accounts.value = response.items || []
  } catch (err) {
    console.error('Failed to load single-user cockpit accounts:', err)
    error.value = 'Failed to load account summary.'
  } finally {
    loading.value = false
  }
}

async function copyValue(value: string, label: string) {
  try {
    await navigator.clipboard.writeText(value)
    copiedLabel.value = label
    if (copyTimer) clearTimeout(copyTimer)
    copyTimer = setTimeout(() => {
      copiedLabel.value = ''
    }, 2000)
  } catch (err) {
    console.error('Failed to copy integration value:', err)
    error.value = 'Clipboard copy failed.'
  }
}

function formatCost(value: number): string {
  if (value >= 1) return value.toFixed(2)
  if (value > 0) return value.toFixed(4)
  return '0.00'
}

function quotaTextClass(severity: QuotaSeverity): string {
  switch (severity) {
    case 'critical':
      return 'font-semibold text-red-600 dark:text-red-400'
    case 'warning':
      return 'font-semibold text-amber-600 dark:text-amber-400'
    case 'healthy':
      return 'font-semibold text-green-600 dark:text-green-400'
    default:
      return 'font-semibold text-gray-500 dark:text-gray-400'
  }
}

function quotaBarClass(severity: QuotaSeverity): string {
  switch (severity) {
    case 'critical':
      return 'bg-red-500'
    case 'warning':
      return 'bg-amber-500'
    case 'healthy':
      return 'bg-green-500'
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
          onClick: () => emit('copy', props.value, props.label),
        }, 'Copy'),
      ]),
    ])
  },
})

onMounted(() => {
  void loadAccounts()
})
</script>
