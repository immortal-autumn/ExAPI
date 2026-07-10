import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import LinuxDoOAuthPanel from '../panels/LinuxDoOAuthPanel.vue'

describe('LinuxDoOAuthPanel', () => {
  it('renders LinuxDo OAuth fields and delegates quick copy', async () => {
    const setAndCopyLinuxdoRedirectUrl = vi.fn()

    const wrapper = mount(LinuxDoOAuthPanel, {
      props: {
        form: {
          linuxdo_connect_enabled: true,
          linuxdo_connect_client_id: 'client-id',
          linuxdo_connect_client_secret: '',
          linuxdo_connect_client_secret_configured: true,
          linuxdo_connect_redirect_url: '',
        },
        linuxdoRedirectUrlSuggestion: 'https://example.com/api/v1/auth/oauth/linuxdo/callback',
        setAndCopyLinuxdoRedirectUrl,
      },
      global: {
        stubs: {
          Toggle: { template: '<input type="checkbox" />' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.linuxdo.title')
    expect(wrapper.text()).toContain('https://example.com/api/v1/auth/oauth/linuxdo/callback')

    await wrapper.find('[data-test="copy-linuxdo-redirect-url"]').trigger('click')
    expect(setAndCopyLinuxdoRedirectUrl).toHaveBeenCalledTimes(1)
  })
})
