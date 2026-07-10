import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import SecurityAdminApiKeyPanel from '../panels/SecurityAdminApiKeyPanel.vue'

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
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.adminApiKey.title')
    expect(wrapper.text()).toContain('exapi_****')

    await wrapper.find('[data-test="regenerate-admin-api-key"]').trigger('click')
    expect(regenerateAdminApiKey).toHaveBeenCalledTimes(1)

    await wrapper.find('[data-test="copy-new-admin-api-key"]').trigger('click')
    expect(copyNewKey).toHaveBeenCalledTimes(1)
  })
})
