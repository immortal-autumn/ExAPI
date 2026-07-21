<template>
  <div class="space-y-6">
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
import SmtpSettingsPanel from '@/views/admin/settings/email/SmtpSettingsPanel.vue'
import TestEmailPanel from '@/views/admin/settings/email/TestEmailPanel.vue'
import AccountQuotaNotificationPanel from '@/views/admin/settings/email/AccountQuotaNotificationPanel.vue'

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
