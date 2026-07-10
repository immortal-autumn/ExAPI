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

    <RegistrationSettingsPanel
      v-model:registration-email-suffix-whitelist-draft="registrationEmailSuffixWhitelistDraft"
      :form="form"
      :registration-email-suffix-whitelist-tags="registrationEmailSuffixWhitelistTags"
      :remove-registration-email-suffix-whitelist-tag="removeRegistrationEmailSuffixWhitelistTag"
      :handle-registration-email-suffix-whitelist-draft-input="handleRegistrationEmailSuffixWhitelistDraftInput"
      :handle-registration-email-suffix-whitelist-draft-keydown="handleRegistrationEmailSuffixWhitelistDraftKeydown"
      :commit-registration-email-suffix-whitelist-draft="commitRegistrationEmailSuffixWhitelistDraft"
      :handle-registration-email-suffix-whitelist-paste="handleRegistrationEmailSuffixWhitelistPaste"
    />

    <SecurityAccessControlsPanel :form="form" />

    <LinuxDoOAuthPanel
      :form="form"
      :linuxdo-redirect-url-suggestion="linuxdoRedirectUrlSuggestion"
      :set-and-copy-linuxdo-redirect-url="setAndCopyLinuxdoRedirectUrl"
    />

    <EmailOAuthPanel
      :form="form"
      :is-zh-locale="isZhLocale"
      :github-o-auth-redirect-url-suggestion="githubOAuthRedirectUrlSuggestion"
      :google-o-auth-redirect-url-suggestion="googleOAuthRedirectUrlSuggestion"
      :set-and-copy-email-o-auth-redirect-url="setAndCopyEmailOAuthRedirectUrl"
      :local-text="localText"
    />

    <WeChatConnectPanel
      :form="form"
      :wechat-redirect-url-suggestion="wechatRedirectUrlSuggestion"
      :set-and-copy-we-chat-redirect-url="setAndCopyWeChatRedirectUrl"
      :handle-we-chat-open-enabled-change="handleWeChatOpenEnabledChange"
      :handle-we-chat-m-p-enabled-change="handleWeChatMPEnabledChange"
      :handle-we-chat-mobile-enabled-change="handleWeChatMobileEnabledChange"
      :local-text="localText"
    />

    <DingTalkConnectPanel
      :form="form"
      :local-text="localText"
    />

    <OidcConnectPanel
      :form="form"
      :oidc-redirect-url-suggestion="oidcRedirectUrlSuggestion"
      :set-and-copy-o-i-d-c-redirect-url="setAndCopyOIDCRedirectUrl"
    />
  </div>
</template>

<script setup lang="ts">
import SecurityAdminApiKeyPanel from '@/views/admin/settings/panels/SecurityAdminApiKeyPanel.vue'
import RegistrationSettingsPanel from '@/views/admin/settings/panels/RegistrationSettingsPanel.vue'
import SecurityAccessControlsPanel from '@/views/admin/settings/panels/SecurityAccessControlsPanel.vue'
import LinuxDoOAuthPanel from '@/views/admin/settings/panels/LinuxDoOAuthPanel.vue'
import EmailOAuthPanel from '@/views/admin/settings/panels/EmailOAuthPanel.vue'
import WeChatConnectPanel from '@/views/admin/settings/panels/WeChatConnectPanel.vue'
import DingTalkConnectPanel from '@/views/admin/settings/panels/DingTalkConnectPanel.vue'
import OidcConnectPanel from '@/views/admin/settings/panels/OidcConnectPanel.vue'

type MutableRecord = Record<string, any>
type AnyHandler = (...args: any[]) => void
type LocalText = (zh: string, en: string) => string

const registrationEmailSuffixWhitelistDraft = defineModel<string>(
  'registrationEmailSuffixWhitelistDraft',
  { required: true },
)

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
