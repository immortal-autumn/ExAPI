<template>
          <!-- Registration Settings -->
          <div class="card">
            <div
              class="border-b border-gray-100 px-6 py-4 dark:border-dark-700"
            >
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t("admin.settings.registration.title") }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t("admin.settings.registration.description") }}
              </p>
            </div>
            <div class="space-y-5 p-6">
              <!-- Enable Registration -->
              <div class="flex items-center justify-between">
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">{{
                    t("admin.settings.registration.enableRegistration")
                  }}</label>
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {{
                      t("admin.settings.registration.enableRegistrationHint")
                    }}
                  </p>
                </div>
                <Toggle v-model="form.registration_enabled" />
              </div>

              <!-- Email Verification -->
              <div
                class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
              >
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">{{
                    t("admin.settings.registration.emailVerification")
                  }}</label>
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {{ t("admin.settings.registration.emailVerificationHint") }}
                  </p>
                </div>
                <Toggle v-model="form.email_verify_enabled" />
              </div>

              <!-- Email Suffix Whitelist -->
              <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t("admin.settings.registration.emailSuffixWhitelist")
                }}</label>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{
                    t("admin.settings.registration.emailSuffixWhitelistHint")
                  }}
                </p>
                <div
                  class="mt-3 rounded-lg border border-gray-300 bg-white p-2 dark:border-dark-500 dark:bg-dark-700"
                >
                  <div class="flex flex-wrap items-center gap-2">
                    <span
                      v-for="suffix in registrationEmailSuffixWhitelistTags"
                      :key="suffix"
                      class="inline-flex items-center gap-1 rounded bg-gray-100 px-2 py-1 text-xs font-mono text-gray-700 dark:bg-dark-600 dark:text-gray-200"
                    >
                      <span>{{ suffix }}</span>
                      <button
                        type="button"
                        class="rounded-full text-gray-500 hover:bg-gray-200 hover:text-gray-700 dark:text-gray-300 dark:hover:bg-dark-500 dark:hover:text-white"
                        :data-test="`remove-registration-email-suffix-${suffix}`"
                        @click="removeRegistrationEmailSuffixWhitelistTag(suffix)"
                      >
                        <Icon
                          name="x"
                          size="xs"
                          class="h-3.5 w-3.5"
                          :stroke-width="2"
                        />
                      </button>
                    </span>

                    <div
                      class="flex min-w-[220px] flex-1 items-center gap-1 rounded border border-transparent px-2 py-1 focus-within:border-primary-300 dark:focus-within:border-primary-700"
                    >
                      <input
                        v-model="registrationEmailSuffixWhitelistDraft"
                        type="text"
                        class="w-full bg-transparent text-sm font-mono text-gray-900 outline-none placeholder:text-gray-400 dark:text-white dark:placeholder:text-gray-500"
                        :placeholder="
                          t(
                            'admin.settings.registration.emailSuffixWhitelistPlaceholder',
                          )
                        "
                        @input="
                          handleRegistrationEmailSuffixWhitelistDraftInput
                        "
                        @keydown="
                          handleRegistrationEmailSuffixWhitelistDraftKeydown
                        "
                        @blur="commitRegistrationEmailSuffixWhitelistDraft"
                        @paste="handleRegistrationEmailSuffixWhitelistPaste"
                      />
                    </div>
                  </div>
                </div>
                <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                  {{
                    t(
                      "admin.settings.registration.emailSuffixWhitelistInputHint",
                    )
                  }}
                </p>
              </div>

              <!-- Promo Code -->
              <div
                class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
              >
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">{{
                    t("admin.settings.registration.promoCode")
                  }}</label>
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {{ t("admin.settings.registration.promoCodeHint") }}
                  </p>
                </div>
                <Toggle v-model="form.promo_code_enabled" />
              </div>

              <!-- Invitation Code -->
              <div
                class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
              >
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">{{
                    t("admin.settings.registration.invitationCode")
                  }}</label>
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {{ t("admin.settings.registration.invitationCodeHint") }}
                  </p>
                </div>
                <Toggle v-model="form.invitation_code_enabled" />
              </div>
              <!-- Password Reset - Only show when email verification is enabled -->
              <div
                v-if="form.email_verify_enabled"
                class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
              >
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">{{
                    t("admin.settings.registration.passwordReset")
                  }}</label>
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {{ t("admin.settings.registration.passwordResetHint") }}
                  </p>
                </div>
                <Toggle v-model="form.password_reset_enabled" />
              </div>
              <!-- Frontend URL - Only show when password reset is enabled -->
              <div
                v-if="form.email_verify_enabled && form.password_reset_enabled"
                class="border-t border-gray-100 pt-4 dark:border-dark-700"
              >
                <label
                  class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >
                  {{ t("admin.settings.registration.frontendUrl") }}
                </label>
                <input
                  v-model="form.frontend_url"
                  type="url"
                  class="input"
                  :placeholder="
                    t('admin.settings.registration.frontendUrlPlaceholder')
                  "
                />
                <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t("admin.settings.registration.frontendUrlHint") }}
                </p>
              </div>

              <!-- TOTP 2FA -->
              <div
                class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
              >
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">{{
                    t("admin.settings.registration.totp")
                  }}</label>
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {{ t("admin.settings.registration.totpHint") }}
                  </p>
                  <!-- Warning when encryption key not configured -->
                  <p
                    v-if="!form.totp_encryption_key_configured"
                    class="mt-2 text-sm text-amber-600 dark:text-amber-400"
                  >
                    {{ t("admin.settings.registration.totpKeyNotConfigured") }}
                  </p>
                </div>
                <Toggle
                  v-model="form.totp_enabled"
                  :disabled="!form.totp_encryption_key_configured"
                />
              </div>
            </div>
          </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'

type MutableRecord = Record<string, any>
type AnyHandler = (...args: any[]) => void

defineProps<{
  form: MutableRecord
  registrationEmailSuffixWhitelistTags: string[]
  removeRegistrationEmailSuffixWhitelistTag: (suffix: string) => void
  handleRegistrationEmailSuffixWhitelistDraftInput: AnyHandler
  handleRegistrationEmailSuffixWhitelistDraftKeydown: AnyHandler
  commitRegistrationEmailSuffixWhitelistDraft: AnyHandler
  handleRegistrationEmailSuffixWhitelistPaste: AnyHandler
}>()

const registrationEmailSuffixWhitelistDraft = defineModel<string>(
  'registrationEmailSuffixWhitelistDraft',
  { required: true },
)

const { t } = useI18n()
</script>
