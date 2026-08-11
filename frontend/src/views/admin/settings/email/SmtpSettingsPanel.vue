<template>
<!-- SMTP remains available for private operational notifications. -->
<div class="card">
  <div
    class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700"
  >
    <div>
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t("admin.settings.smtp.title") }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t("admin.settings.smtp.description") }}
      </p>
    </div>
    <button
      type="button"
      @click="testSmtpConnection"
      :disabled="testingSmtp || loadFailed"
      class="btn btn-secondary btn-sm"
    >
      <svg
        v-if="testingSmtp"
        class="h-4 w-4 animate-spin"
        fill="none"
        viewBox="0 0 24 24"
      >
        <circle
          class="opacity-25"
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          stroke-width="4"
        ></circle>
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        ></path>
      </svg>
      {{
        testingSmtp
          ? t("admin.settings.smtp.testing")
          : t("admin.settings.smtp.testConnection")
      }}
    </button>
  </div>
  <div class="space-y-6 p-6">
    <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
      <div>
        <label
          class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          {{ t("admin.settings.smtp.host") }}
        </label>
        <input
          v-model="form.smtp_host"
          type="text"
          class="input"
          :placeholder="t('admin.settings.smtp.hostPlaceholder')"
        />
      </div>
      <div>
        <label
          class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          {{ t("admin.settings.smtp.port") }}
        </label>
        <input
          v-model.number="form.smtp_port"
          type="number"
          min="1"
          max="65535"
          class="input"
          :placeholder="t('admin.settings.smtp.portPlaceholder')"
        />
      </div>
      <div>
        <label
          class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          {{ t("admin.settings.smtp.username") }}
        </label>
        <input
          v-model="form.smtp_username"
          type="text"
          class="input"
          :placeholder="t('admin.settings.smtp.usernamePlaceholder')"
        />
      </div>
      <div>
        <label
          class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          {{ t("admin.settings.smtp.password") }}
        </label>
        <input
          v-model="form.smtp_password"
          type="password"
          class="input"
          autocomplete="new-password"
          autocapitalize="off"
          spellcheck="false"
          @keydown="markSmtpPasswordManuallyEdited"
          @paste="markSmtpPasswordManuallyEdited"
          :placeholder="
            form.smtp_password_configured
              ? t('admin.settings.smtp.passwordConfiguredPlaceholder')
              : t('admin.settings.smtp.passwordPlaceholder')
          "
        />
        <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{
            form.smtp_password_configured
              ? t("admin.settings.smtp.passwordConfiguredHint")
              : t("admin.settings.smtp.passwordHint")
          }}
        </p>
      </div>
      <div>
        <label
          class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          {{ t("admin.settings.smtp.fromEmail") }}
        </label>
        <input
          v-model="form.smtp_from_email"
          type="email"
          class="input"
          :placeholder="t('admin.settings.smtp.fromEmailPlaceholder')"
        />
      </div>
      <div>
        <label
          class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          {{ t("admin.settings.smtp.fromName") }}
        </label>
        <input
          v-model="form.smtp_from_name"
          type="text"
          class="input"
          :placeholder="t('admin.settings.smtp.fromNamePlaceholder')"
        />
      </div>
    </div>

    <!-- Use TLS Toggle -->
    <div
      class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
    >
      <div>
        <label class="font-medium text-gray-900 dark:text-white">{{
          t("admin.settings.smtp.useTls")
        }}</label>
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.smtp.useTlsHint") }}
        </p>
      </div>
      <Toggle v-model="form.smtp_use_tls" />
    </div>
  </div>
</div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'

const { t } = useI18n()

type AnyHandler = (...args: any[]) => void

defineProps<{
  form: any
  testingSmtp: boolean
  loadFailed: boolean
  testSmtpConnection: AnyHandler
  markSmtpPasswordManuallyEdited: AnyHandler
}>()
</script>
