<template>
          <!-- Admin API Key Settings -->
          <SettingsSectionCard>
            <template #title>
              {{ t("admin.settings.adminApiKey.title") }}
            </template>
            <template #description>
              {{ t("admin.settings.adminApiKey.description") }}
            </template>
            <!-- Security Warning -->
            <div
              class="rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-900/20"
            >
                <div class="flex items-start">
                  <Icon
                    name="exclamationTriangle"
                    size="md"
                    class="mt-0.5 flex-shrink-0 text-amber-500"
                  />
                  <p class="ml-3 text-sm text-amber-700 dark:text-amber-300">
                    {{ t("admin.settings.adminApiKey.securityWarning") }}
                  </p>
                </div>
              </div>

              <!-- Loading State -->
              <div
                v-if="adminApiKeyLoading"
                class="flex items-center gap-2 text-gray-500"
              >
                <div
                  class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"
                ></div>
                {{ t("common.loading") }}
              </div>

              <!-- No Key Configured -->
              <div
                v-else-if="!adminApiKeyExists"
                class="flex items-center justify-between"
              >
                <span class="text-gray-500 dark:text-gray-400">
                  {{ t("admin.settings.adminApiKey.notConfigured") }}
                </span>
                <button
                  type="button"
                  @click="createAdminApiKey"
                  :disabled="adminApiKeyOperating"
                  class="btn btn-primary btn-sm"
                >
                  <svg
                    v-if="adminApiKeyOperating"
                    class="mr-1 h-4 w-4 animate-spin"
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
                    adminApiKeyOperating
                      ? t("admin.settings.adminApiKey.creating")
                      : t("admin.settings.adminApiKey.create")
                  }}
                </button>
              </div>

              <!-- Key Exists -->
              <div v-else class="space-y-4">
                <div class="flex items-center justify-between">
                  <div>
                    <label
                      class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                    >
                      {{ t("admin.settings.adminApiKey.currentKey") }}
                    </label>
                    <code
                      class="rounded bg-gray-100 px-2 py-1 font-mono text-sm text-gray-900 dark:bg-dark-700 dark:text-gray-100"
                    >
                      {{ adminApiKeyMasked }}
                    </code>
                  </div>
                  <div class="flex gap-2">
                    <button
                      type="button"
                      data-test="regenerate-admin-api-key"
                      @click="openConfirm('regenerate')"
                      :disabled="adminApiKeyOperating"
                      class="btn btn-secondary btn-sm"
                    >
                      {{
                        adminApiKeyOperating
                          ? t("admin.settings.adminApiKey.regenerating")
                          : t("admin.settings.adminApiKey.regenerate")
                      }}
                    </button>
                    <button
                      type="button"
                      data-test="delete-admin-api-key"
                      @click="openConfirm('delete')"
                      :disabled="adminApiKeyOperating"
                      class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                    >
                      {{ t("admin.settings.adminApiKey.delete") }}
                    </button>
                  </div>
                </div>

                <!-- Newly Generated Key Display -->
                <div
                  v-if="newAdminApiKey"
                  class="space-y-3 rounded-lg border border-green-200 bg-green-50 p-4 dark:border-green-800 dark:bg-green-900/20"
                >
                  <p
                    class="text-sm font-medium text-green-700 dark:text-green-300"
                  >
                    {{ t("admin.settings.adminApiKey.keyWarning") }}
                  </p>
                  <div class="flex items-center gap-2">
                    <code
                      class="flex-1 select-all break-all rounded border border-green-300 bg-white px-3 py-2 font-mono text-sm dark:border-green-700 dark:bg-dark-800"
                    >
                      {{ newAdminApiKey }}
                    </code>
                    <button
                      type="button"
                      data-test="copy-new-admin-api-key"
                      @click="copyNewKey"
                      class="btn btn-primary btn-sm flex-shrink-0"
                    >
                      {{ t("admin.settings.adminApiKey.copyKey") }}
                    </button>
                  </div>
                  <p class="text-xs text-green-600 dark:text-green-400">
                    {{ t("admin.settings.adminApiKey.usage") }}
                  </p>
                </div>
              </div>

              <ConfirmDialog
                :show="pendingAction !== null"
                :title="pendingAction === 'delete'
                  ? t('admin.settings.adminApiKey.delete')
                  : t('admin.settings.adminApiKey.regenerate')"
                :message="pendingAction === 'delete'
                  ? t('admin.settings.adminApiKey.deleteConfirm')
                  : t('admin.settings.adminApiKey.regenerateConfirm')"
                :confirm-text="pendingAction === 'delete'
                  ? t('admin.settings.adminApiKey.delete')
                  : t('admin.settings.adminApiKey.regenerate')"
                :cancel-text="t('common.cancel')"
                :danger="pendingAction === 'delete'"
                :loading="confirmLoading"
                @confirm="confirmPendingAction"
                @cancel="clearPendingAction"
              />
          </SettingsSectionCard>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import Icon from '@/components/icons/Icon.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import SettingsSectionCard from '../SettingsSectionCard.vue'

type AnyHandler = (...args: any[]) => void

const { t } = useI18n()

type PendingAction = 'regenerate' | 'delete' | null

const props = defineProps<{
  adminApiKeyLoading: boolean
  adminApiKeyExists: boolean
  adminApiKeyOperating: boolean
  adminApiKeyMasked: string
  newAdminApiKey: string
  createAdminApiKey: AnyHandler
  regenerateAdminApiKey: AnyHandler
  deleteAdminApiKey: AnyHandler
  copyNewKey: AnyHandler
}>()

const pendingAction = ref<PendingAction>(null)
const confirmLoading = ref(false)

const openConfirm = (action: Exclude<PendingAction, null>) => {
  if (props.adminApiKeyOperating) return
  pendingAction.value = action
}

const clearPendingAction = () => {
  if (confirmLoading.value) return
  pendingAction.value = null
}

const confirmPendingAction = async () => {
  if (!pendingAction.value || confirmLoading.value) return
  confirmLoading.value = true
  try {
    if (pendingAction.value === 'regenerate') {
      await Promise.resolve(props.regenerateAdminApiKey())
    } else if (pendingAction.value === 'delete') {
      await Promise.resolve(props.deleteAdminApiKey())
    }
    pendingAction.value = null
  } finally {
    confirmLoading.value = false
  }
}
</script>
