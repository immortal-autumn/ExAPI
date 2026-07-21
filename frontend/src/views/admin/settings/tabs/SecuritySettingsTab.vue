<template>
  <div class="space-y-6">
    <SecurityAdminApiKeyPanel
      :admin-api-key-loading="adminApiKeyLoading"
      :admin-api-key-exists="adminApiKeyExists"
      :admin-api-key-operating="adminApiKeyOperating"
      :admin-api-key-masked="adminApiKeyMasked"
      :new-admin-api-key="newAdminApiKey"
      :create-admin-api-key="createAdminApiKey"
      :regenerate-admin-api-key="regenerateAdminApiKey"
      :delete-admin-api-key="deleteAdminApiKey"
      :copy-new-key="copyNewKey"
    />

    <SecurityAccessControlsPanel :form="form" />
  </div>
</template>

<script setup lang="ts">
import SecurityAdminApiKeyPanel from '@/views/admin/settings/panels/SecurityAdminApiKeyPanel.vue'
import SecurityAccessControlsPanel from '@/views/admin/settings/panels/SecurityAccessControlsPanel.vue'

type MutableRecord = Record<string, any>
type AnyHandler = (...args: any[]) => void
type LocalText = (zh: string, en: string) => string

defineModel<string>('registrationEmailSuffixWhitelistDraft', { required: true })

defineProps<{
  form: MutableRecord
  adminApiKeyLoading: boolean
  adminApiKeyExists: boolean
  adminApiKeyOperating: boolean
  adminApiKeyMasked: string
  newAdminApiKey: string
  createAdminApiKey: AnyHandler
  regenerateAdminApiKey: AnyHandler
  deleteAdminApiKey: AnyHandler
  copyNewKey: AnyHandler
  registrationEmailSuffixWhitelistTags: string[]
  removeRegistrationEmailSuffixWhitelistTag: (suffix: string) => void
  handleRegistrationEmailSuffixWhitelistDraftInput: AnyHandler
  handleRegistrationEmailSuffixWhitelistDraftKeydown: AnyHandler
  commitRegistrationEmailSuffixWhitelistDraft: AnyHandler
  handleRegistrationEmailSuffixWhitelistPaste: AnyHandler
  linuxdoRedirectUrlSuggestion: string
  setAndCopyLinuxdoRedirectUrl: AnyHandler
  isZhLocale: boolean
  githubOAuthRedirectUrlSuggestion: string
  googleOAuthRedirectUrlSuggestion: string
  setAndCopyEmailOAuthRedirectUrl: (provider: 'github' | 'google') => void
  wechatRedirectUrlSuggestion: string
  setAndCopyWeChatRedirectUrl: AnyHandler
  handleWeChatOpenEnabledChange: AnyHandler
  handleWeChatMPEnabledChange: AnyHandler
  handleWeChatMobileEnabledChange: AnyHandler
  oidcRedirectUrlSuggestion: string
  setAndCopyOIDCRedirectUrl: AnyHandler
  localText: LocalText
}>()
</script>
