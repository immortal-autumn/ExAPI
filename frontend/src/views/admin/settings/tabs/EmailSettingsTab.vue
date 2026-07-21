<template>
  <div class="space-y-6">
    <!-- Email disabled hint - show when email_verify_enabled is off -->
    <div v-if="!form.email_verify_enabled" class="card">
      <div class="p-6">
        <div class="flex items-start gap-3">
          <Icon
            name="mail"
            size="md"
            class="mt-0.5 flex-shrink-0 text-gray-400 dark:text-gray-500"
          />
          <div>
            <h3 class="font-medium text-gray-900 dark:text-white">
              {{ t("admin.settings.emailTabDisabledTitle") }}
            </h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.emailTabDisabledHint") }}
            </p>
          </div>
        </div>
      </div>
    </div>

    <SmtpSettingsPanel
      :form="form"
      :testing-smtp="testingSmtp"
      :load-failed="loadFailed"
      :test-smtp-connection="testSmtpConnection"
      :mark-smtp-password-manually-edited="markSmtpPasswordManuallyEdited"
    />

    <TestEmailPanel
      :form="form"
      :test-email-address="testEmailAddress"
      :sending-test-email="sendingTestEmail"
      :load-failed="loadFailed"
      :send-test-email="sendTestEmail"
      :update-test-email-address="updateTestEmailAddress"
    />

    <AccountQuotaNotificationPanel
      :form="form"
      :add-quota-notify-email="addQuotaNotifyEmail"
      :remove-quota-notify-email="removeQuotaNotifyEmail"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import SmtpSettingsPanel from '@/views/admin/settings/email/SmtpSettingsPanel.vue'
import TestEmailPanel from '@/views/admin/settings/email/TestEmailPanel.vue'
import AccountQuotaNotificationPanel from '@/views/admin/settings/email/AccountQuotaNotificationPanel.vue'

const { t } = useI18n()

type AnyHandler = (...args: any[]) => void

defineProps<{
  form: any
  testingSmtp: boolean
  loadFailed: boolean
  testSmtpConnection: AnyHandler
  markSmtpPasswordManuallyEdited: AnyHandler
  testEmailAddress: string
  sendingTestEmail: boolean
  sendTestEmail: AnyHandler
  updateTestEmailAddress: (value: string) => void
  currentOrigin: string
  addQuotaNotifyEmail: AnyHandler
  removeQuotaNotifyEmail: (index: number) => void
}>()
</script>
