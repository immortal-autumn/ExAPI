<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <div class="card p-4">
        <div class="flex flex-wrap gap-2" role="tablist" :aria-label="t('admin.settings.title')">
          <button
            v-for="tab in tabs"
            :key="tab"
            type="button"
            role="tab"
            :id="`admin-settings-tab-${tab}`"
            class="btn btn-sm"
            :class="activeTab === tab ? 'btn-primary' : 'btn-secondary'"
            :aria-selected="activeTab === tab"
            :aria-controls="`admin-settings-panel-${tab}`"
            @click="activeTab = tab"
          >
            {{ t(`admin.settings.tabs.${tab}`) }}
          </button>
        </div>
      </div>

      <div v-if="loading" class="flex justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>

      <form
        v-else
        :id="`admin-settings-panel-${activeTab}`"
        role="tabpanel"
        :aria-labelledby="`admin-settings-tab-${activeTab}`"
        class="space-y-6"
        novalidate
        @submit.prevent="saveSettings"
      >
        <GeneralSettingsTab
          v-if="activeTab === 'general'"
          v-model:table-page-size-options-input="tablePageSizeOptionsInput"
          :form="form"
          :add-endpoint="addEndpoint"
          :remove-endpoint="removeEndpoint"
        />

        <div v-if="activeTab === 'gateway'" class="space-y-6">
          <GatewayCooldownPanel
            :overload-cooldown-loading="gatewayLoading"
            :overload-cooldown-saving="overloadSaving"
            :overload-cooldown-form="overloadForm"
            :save-overload-cooldown-settings="saveOverload"
            :rate-limit429-cooldown-loading="gatewayLoading"
            :rate-limit429-cooldown-saving="rateLimitSaving"
            :rate-limit429-cooldown-form="rateLimitForm"
            :save-rate-limit429-cooldown-settings="saveRateLimit"
          />
          <GatewayStreamTimeoutPanel
            :stream-timeout-loading="gatewayLoading"
            :stream-timeout-saving="streamSaving"
            :stream-timeout-form="streamForm"
            :save-stream-timeout-settings="saveStreamTimeout"
          />
          <GatewayRectifierPanel
            :rectifier-loading="gatewayLoading"
            :rectifier-saving="rectifierSaving"
            :rectifier-form="rectifierForm"
            :save-rectifier-settings="saveRectifier"
          />

          <section class="card">
            <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.settings.privateControlPlane.runtimeTitle') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.privateControlPlane.runtimeDescription') }}
              </p>
            </div>
            <div class="grid gap-5 p-6 md:grid-cols-2">
              <OperatorToggle v-model="form.ops_monitoring_enabled" :label="t('admin.settings.privateControlPlane.monitoringEnabled')" />
              <OperatorToggle v-model="form.ops_realtime_monitoring_enabled" :label="t('admin.settings.privateControlPlane.realtimeMonitoringEnabled')" />
              <OperatorToggle v-model="form.channel_monitor_enabled" :label="t('admin.settings.privateControlPlane.channelMonitorEnabled')" />
              <OperatorToggle v-model="form.risk_control_enabled" :label="t('admin.settings.privateControlPlane.riskControlEnabled')" />
              <OperatorToggle v-model="form.allow_ungrouped_key_scheduling" :label="t('admin.settings.privateControlPlane.allowUngroupedKeyScheduling')" />
              <OperatorToggle v-model="form.api_key_acl_trust_forwarded_ip" :label="t('admin.settings.privateControlPlane.trustForwardedIp')" />
              <label class="space-y-1 text-sm text-gray-700 dark:text-gray-300">
                <span>{{ t('admin.settings.privateControlPlane.monitorInterval') }}</span>
                <input v-model.number="form.channel_monitor_default_interval_seconds" type="number" min="10" class="input" />
              </label>
              <label class="space-y-1 text-sm text-gray-700 dark:text-gray-300">
                <span>{{ t('admin.settings.privateControlPlane.metricsInterval') }}</span>
                <input v-model.number="form.ops_metrics_interval_seconds" type="number" min="10" class="input" />
              </label>
              <label class="space-y-1 text-sm text-gray-700 dark:text-gray-300 md:col-span-2">
                <span>{{ t('admin.settings.privateControlPlane.forwardedHeaders') }}</span>
                <textarea v-model="forwardedHeadersInput" rows="3" class="input font-mono text-sm"></textarea>
              </label>
            </div>
          </section>

          <section class="card">
            <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.settings.privateControlPlane.routingTitle') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.privateControlPlane.routingDescription') }}
              </p>
            </div>
            <div class="grid gap-5 p-6 md:grid-cols-2">
              <OperatorToggle v-model="form.enable_model_fallback" :label="t('admin.settings.privateControlPlane.modelFallback')" />
              <OperatorToggle v-model="form.enable_identity_patch" :label="t('admin.settings.privateControlPlane.identityPatch')" />
              <OperatorToggle v-model="form.enable_fingerprint_unification" :label="t('admin.settings.privateControlPlane.fingerprintUnification')" />
              <OperatorToggle v-model="form.enable_client_dateline_normalization" :label="t('admin.settings.privateControlPlane.clientDatelineNormalization')" />
              <OperatorToggle v-model="form.enable_metadata_passthrough" :label="t('admin.settings.privateControlPlane.metadataPassthrough')" />
              <OperatorToggle v-model="form.openai_advanced_scheduler_enabled" :label="t('admin.settings.privateControlPlane.openaiAdvancedScheduler')" />
              <label v-for="field in fallbackFields" :key="field.key" class="space-y-1 text-sm text-gray-700 dark:text-gray-300">
                <span>{{ field.label }}</span>
                <input v-model="form[field.key]" type="text" class="input font-mono text-sm" />
              </label>
              <label class="space-y-1 text-sm text-gray-700 dark:text-gray-300 md:col-span-2">
                <span>{{ t('admin.settings.privateControlPlane.identityPatchPrompt') }}</span>
                <textarea v-model="form.identity_patch_prompt" rows="4" class="input font-mono text-sm"></textarea>
              </label>
            </div>
          </section>

          <section class="card">
            <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.settings.privateControlPlane.compatibilityTitle') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.privateControlPlane.compatibilityDescription') }}
              </p>
            </div>
            <div class="grid gap-5 p-6 md:grid-cols-2">
              <label v-for="field in versionFields" :key="field.key" class="space-y-1 text-sm text-gray-700 dark:text-gray-300">
                <span>{{ field.label }}</span>
                <input v-model="form[field.key]" type="text" class="input font-mono text-sm" />
              </label>
              <OperatorToggle
                v-model="form.codex_cli_only_allow_app_server_clients"
                :label="t('admin.settings.privateControlPlane.allowCodexAppServerClients')"
              />
            </div>
          </section>
        </div>

        <EmailSettingsTab
          v-if="activeTab === 'email'"
          :form="form"
          :testing-smtp="testingSmtp"
          :load-failed="loadFailed"
          :test-smtp-connection="testSmtp"
          :mark-smtp-password-manually-edited="markSmtpPasswordEdited"
          :test-email-address="testEmailAddress"
          :sending-test-email="sendingTestEmail"
          :send-test-email="sendTestEmail"
          :update-test-email-address="updateTestEmailAddress"
          :current-origin="currentOrigin"
          :add-quota-notify-email="addQuotaNotifyEmail"
          :remove-quota-notify-email="removeQuotaNotifyEmail"
        />

        <SecuritySettingsTab v-if="activeTab === 'security'" />

        <BackupSettingsTab v-if="activeTab === 'backup'" />

        <div v-if="activeTab !== 'backup' && activeTab !== 'security'" class="flex justify-end">
          <button type="submit" class="btn btn-primary" :disabled="saving || loadFailed">
            {{ saving ? t('admin.settings.saving') : t('admin.settings.saveSettings') }}
          </button>
        </div>
      </form>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Toggle from '@/components/common/Toggle.vue'
import { operatorAPI } from '@/api/operator'
import { useAppStore } from '@/stores/app'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { extractApiErrorMessage } from '@/utils/apiError'
import { buildPrivateOperatorSettings } from './settings/privateSettingsPayload'
import GeneralSettingsTab from './settings/tabs/GeneralSettingsTab.vue'
import EmailSettingsTab from './settings/tabs/EmailSettingsTab.vue'
import SecuritySettingsTab from './settings/tabs/SecuritySettingsTab.vue'
import BackupSettingsTab from './settings/tabs/BackupSettingsTab.vue'
import GatewayCooldownPanel from './settings/gateway/GatewayCooldownPanel.vue'
import GatewayStreamTimeoutPanel from './settings/gateway/GatewayStreamTimeoutPanel.vue'
import GatewayRectifierPanel from './settings/gateway/GatewayRectifierPanel.vue'

type Tab = 'general' | 'gateway' | 'email' | 'security' | 'backup'

const OperatorToggle = defineComponent({
  props: { modelValue: Boolean, label: { type: String, required: true } },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () => h('label', { class: 'flex items-center justify-between gap-4 text-sm text-gray-700 dark:text-gray-300' }, [
      h('span', props.label),
      h(Toggle, {
        modelValue: props.modelValue,
        'onUpdate:modelValue': (value: boolean) => emit('update:modelValue', value),
      }),
    ])
  },
})

const { t } = useI18n()
const appStore = useAppStore()
const adminSettingsStore = useAdminSettingsStore()
const tabs: Tab[] = ['general', 'gateway', 'email', 'security', 'backup']
const activeTab = ref<Tab>('general')
const loading = ref(true)
const loadFailed = ref(false)
const saving = ref(false)
const gatewayLoading = ref(true)
const testingSmtp = ref(false)
const sendingTestEmail = ref(false)
const smtpPasswordEdited = ref(false)
const testEmailAddress = ref('')
const tablePageSizeOptionsInput = ref('10, 20, 50, 100')
const currentOrigin = typeof window === 'undefined' ? '' : window.location.origin

const form = reactive<any>({
  audit_log_retention_days: 180,
  site_name: 'ExAPI',
  site_logo: '',
  site_subtitle: 'Private AI gateway control plane',
  api_base_url: '',
  contact_info: '',
  doc_url: '',
  home_content: '',
  backend_mode_enabled: true,
  hide_ccs_import_button: false,
  table_default_page_size: 20,
  table_page_size_options: [10, 20, 50, 100],
  custom_endpoints: [],
  frontend_url: '',
  smtp_host: '',
  smtp_port: 587,
  smtp_username: '',
  smtp_password: '',
  smtp_password_configured: false,
  smtp_from_email: '',
  smtp_from_name: '',
  smtp_use_tls: true,
  account_quota_notify_enabled: false,
  account_quota_notify_emails: [],
  api_key_acl_trust_forwarded_ip: false,
  forwarded_client_ip_headers: [],
  risk_control_enabled: false,
  cyber_session_block_enabled: false,
  cyber_session_block_ttl_seconds: 3600,
  enable_model_fallback: false,
  fallback_model_anthropic: '',
  fallback_model_openai: '',
  fallback_model_gemini: '',
  fallback_model_antigravity: '',
  enable_identity_patch: true,
  identity_patch_prompt: '',
  ops_monitoring_enabled: true,
  ops_realtime_monitoring_enabled: true,
  ops_query_mode_default: 'auto',
  ops_metrics_interval_seconds: 60,
  min_claude_code_version: '',
  max_claude_code_version: '',
  allow_ungrouped_key_scheduling: false,
  openai_advanced_scheduler_enabled: false,
  openai_advanced_scheduler_sticky_weighted_enabled: false,
  openai_advanced_scheduler_subscription_priority_enabled: false,
  enable_fingerprint_unification: true,
  enable_metadata_passthrough: false,
  enable_client_dateline_normalization: true,
  min_codex_version: '',
  max_codex_version: '',
  codex_cli_only_allow_app_server_clients: false,
  channel_monitor_enabled: true,
  channel_monitor_default_interval_seconds: 60,
  allow_user_view_error_requests: false,
})

const fallbackFields = computed(() => [
  { key: 'fallback_model_anthropic', label: t('admin.settings.privateControlPlane.fallbackAnthropic') },
  { key: 'fallback_model_openai', label: t('admin.settings.privateControlPlane.fallbackOpenai') },
  { key: 'fallback_model_gemini', label: t('admin.settings.privateControlPlane.fallbackGemini') },
  { key: 'fallback_model_antigravity', label: t('admin.settings.privateControlPlane.fallbackAntigravity') },
])
const versionFields = computed(() => [
  { key: 'min_claude_code_version', label: t('admin.settings.privateControlPlane.minClaudeCodeVersion') },
  { key: 'max_claude_code_version', label: t('admin.settings.privateControlPlane.maxClaudeCodeVersion') },
  { key: 'min_codex_version', label: t('admin.settings.privateControlPlane.minCodexVersion') },
  { key: 'max_codex_version', label: t('admin.settings.privateControlPlane.maxCodexVersion') },
])

const forwardedHeadersInput = computed({
  get: () => (form.forwarded_client_ip_headers || []).join('\n'),
  set: (value: string) => {
    form.forwarded_client_ip_headers = value.split(/\r?\n/).map((item) => item.trim()).filter(Boolean)
  },
})

const overloadForm = reactive({ enabled: true, cooldown_minutes: 10 })
const rateLimitForm = reactive({ enabled: true, cooldown_seconds: 5 })
const streamForm = reactive({
  enabled: true,
  action: 'temp_unsched' as 'temp_unsched' | 'error' | 'none',
  temp_unsched_minutes: 5,
  threshold_count: 3,
  threshold_window_minutes: 10,
})
const rectifierForm = reactive({
  enabled: true,
  thinking_signature_enabled: true,
  thinking_budget_enabled: true,
  apikey_signature_enabled: false,
  apikey_signature_patterns: [] as string[],
})
const overloadSaving = ref(false)
const rateLimitSaving = ref(false)
const streamSaving = ref(false)
const rectifierSaving = ref(false)

function addEndpoint() {
  form.custom_endpoints.push({ name: '', endpoint: '', description: '' })
}

function removeEndpoint(index: number) {
  form.custom_endpoints.splice(index, 1)
}

function parsePageSizes(): number[] | null {
  const values = tablePageSizeOptionsInput.value.split(',').map((value) => Number(value.trim()))
  if (!values.length || values.some((value) => !Number.isInteger(value) || value < 5 || value > 1000)) return null
  return [...new Set(values)].sort((a, b) => a - b)
}

async function loadSettings() {
  try {
    const settings = await operatorAPI.settings.getSettings()
    for (const key of Object.keys(form)) {
      if (settings[key as keyof typeof settings] !== undefined && settings[key as keyof typeof settings] !== null) {
        form[key] = settings[key as keyof typeof settings]
      }
    }
    tablePageSizeOptionsInput.value = (form.table_page_size_options || [10, 20, 50, 100]).join(', ')
  } catch (error) {
    loadFailed.value = true
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.failedToLoad')))
  } finally {
    loading.value = false
  }
}

async function loadGatewaySettings() {
  const results = await Promise.allSettled([
    operatorAPI.settings.getOverloadCooldownSettings(),
    operatorAPI.settings.getRateLimit429CooldownSettings(),
    operatorAPI.settings.getStreamTimeoutSettings(),
    operatorAPI.settings.getRectifierSettings(),
  ])
  if (results[0].status === 'fulfilled') Object.assign(overloadForm, results[0].value)
  if (results[1].status === 'fulfilled') Object.assign(rateLimitForm, results[1].value)
  if (results[2].status === 'fulfilled') Object.assign(streamForm, results[2].value)
  if (results[3].status === 'fulfilled') Object.assign(rectifierForm, results[3].value)
  gatewayLoading.value = false
}

async function saveSettings() {
  const pageSizes = parsePageSizes()
  if (!pageSizes) {
    appStore.showError(t('admin.settings.site.tablePageSizeOptionsFormatError', { min: 5, max: 1000 }))
    return
  }
  const defaultPageSize = Math.floor(Number(form.table_default_page_size))
  if (defaultPageSize < 5 || defaultPageSize > 1000) {
    appStore.showError(t('admin.settings.site.tableDefaultPageSizeRangeError', { min: 5, max: 1000 }))
    return
  }
  form.table_default_page_size = defaultPageSize
  form.table_page_size_options = pageSizes
  saving.value = true
  try {
    const source = { ...form }
    if (!smtpPasswordEdited.value || !form.smtp_password) source.smtp_password = undefined
    const payload = buildPrivateOperatorSettings(source)
    const updated = await operatorAPI.settings.updateSettings(payload)
    for (const key of Object.keys(form)) {
      if (updated[key as keyof typeof updated] !== undefined && updated[key as keyof typeof updated] !== null) {
        form[key] = updated[key as keyof typeof updated]
      }
    }
    form.smtp_password = ''
    smtpPasswordEdited.value = false
    await adminSettingsStore.fetch(true)
    await appStore.fetchPublicSettings(true)
    appStore.showSuccess(t('admin.settings.saved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.failedToSave')))
  } finally {
    saving.value = false
  }
}

function markSmtpPasswordEdited() {
  smtpPasswordEdited.value = true
}

function updateTestEmailAddress(value: string) {
  testEmailAddress.value = value
}

function smtpConfig() {
  return {
    smtp_host: form.smtp_host,
    smtp_port: Number(form.smtp_port),
    smtp_username: form.smtp_username,
    smtp_password: form.smtp_password,
    smtp_use_tls: form.smtp_use_tls,
  }
}

async function testSmtp() {
  testingSmtp.value = true
  try {
    const result = await operatorAPI.settings.testSmtpConnection(smtpConfig())
    appStore.showSuccess(result.message)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.smtp.testFailed')))
  } finally {
    testingSmtp.value = false
  }
}

async function sendTestEmail() {
  sendingTestEmail.value = true
  try {
    const result = await operatorAPI.settings.sendTestEmail({
      email: testEmailAddress.value,
      ...smtpConfig(),
      smtp_from_email: form.smtp_from_email,
      smtp_from_name: form.smtp_from_name,
    })
    appStore.showSuccess(result.message)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.testEmail.sendFailed')))
  } finally {
    sendingTestEmail.value = false
  }
}

function addQuotaNotifyEmail() {
  form.account_quota_notify_emails.push({ email: '', disabled: false, verified: false })
}

function removeQuotaNotifyEmail(index: number) {
  form.account_quota_notify_emails.splice(index, 1)
}

async function runGatewaySave(flag: { value: boolean }, action: () => Promise<unknown>, successKey: string) {
  flag.value = true
  try {
    await action()
    appStore.showSuccess(t(successKey))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.failedToSave')))
  } finally {
    flag.value = false
  }
}

const saveOverload = () => runGatewaySave(overloadSaving, () => operatorAPI.settings.updateOverloadCooldownSettings({ ...overloadForm }), 'admin.settings.overloadCooldown.saved')
const saveRateLimit = () => runGatewaySave(rateLimitSaving, () => operatorAPI.settings.updateRateLimit429CooldownSettings({ ...rateLimitForm }), 'admin.settings.rateLimit429Cooldown.saved')
const saveStreamTimeout = () => runGatewaySave(streamSaving, () => operatorAPI.settings.updateStreamTimeoutSettings({ ...streamForm }), 'admin.settings.streamTimeout.saved')
const saveRectifier = () => runGatewaySave(rectifierSaving, () => operatorAPI.settings.updateRectifierSettings({
  ...rectifierForm,
  apikey_signature_patterns: rectifierForm.apikey_signature_patterns.map((item) => item.trim()).filter(Boolean),
}), 'admin.settings.rectifier.saved')

onMounted(() => {
  void loadSettings()
  void loadGatewaySettings()
})
</script>
