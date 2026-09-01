import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import SecurityAdminApiKeyPanel from '../panels/SecurityAdminApiKeyPanel.vue'

const ConfirmDialogStub = defineComponent({
  props: {
    show: Boolean,
    title: String,
    message: String,
    confirmText: String,
    cancelText: String,
    danger: Boolean,
    loading: Boolean,
  },
  emits: ['confirm', 'cancel'],
  template: `
    <div v-if="show" data-testid="confirm-dialog">
      <div class="confirm-title">{{ title }}</div>
      <div class="confirm-message">{{ message }}</div>
      <button type="button" data-testid="confirm-action" @click="$emit('confirm')">{{ confirmText }}</button>
      <button type="button" data-testid="cancel-action" @click="$emit('cancel')">{{ cancelText }}</button>
    </div>
  `,
})

describe('SecurityAdminApiKeyPanel', () => {
  it('renders key state and delegates key actions', async () => {
    const createAdminApiKey = vi.fn()
    const regenerateAdminApiKey = vi.fn()
    const deleteAdminApiKey = vi.fn()
    const copyNewKey = vi.fn()

    const wrapper = mount(SecurityAdminApiKeyPanel, {
      props: {
        adminApiKeyLoading: false,
        adminApiKeyExists: true,
        adminApiKeyOperating: false,
        adminApiKeyMasked: 'exapi_****',
        newAdminApiKey: 'exapi_new_secret',
        createAdminApiKey,
        regenerateAdminApiKey,
        deleteAdminApiKey,
        copyNewKey,
      },
      global: {
        stubs: {
          Icon: { template: '<span />' },
          ConfirmDialog: ConfirmDialogStub,
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.adminApiKey.title')
    expect(wrapper.text()).toContain('exapi_****')

    await wrapper.find('[data-test="regenerate-admin-api-key"]').trigger('click')
    expect(wrapper.text()).toContain('admin.settings.adminApiKey.regenerateConfirm')
    await wrapper.find('[data-testid="confirm-action"]').trigger('click')
    expect(regenerateAdminApiKey).toHaveBeenCalledTimes(1)

    await wrapper.find('[data-test="delete-admin-api-key"]').trigger('click')
    expect(wrapper.text()).toContain('admin.settings.adminApiKey.deleteConfirm')
    await wrapper.find('[data-testid="confirm-action"]').trigger('click')
    expect(deleteAdminApiKey).toHaveBeenCalledTimes(1)

    await wrapper.find('[data-test="copy-new-admin-api-key"]').trigger('click')
    expect(copyNewKey).toHaveBeenCalledTimes(1)
  })
})
