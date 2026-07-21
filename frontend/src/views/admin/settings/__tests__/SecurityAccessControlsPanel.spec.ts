import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import SecurityAccessControlsPanel from '../panels/SecurityAccessControlsPanel.vue'

describe('SecurityAccessControlsPanel', () => {
  it('renders only the operator API ACL control', () => {
    const wrapper = mount(SecurityAccessControlsPanel, {
      props: {
        form: {
          api_key_acl_trust_forwarded_ip: false,
          turnstile_enabled: true,
          turnstile_site_key: 'site-key',
          turnstile_secret_key: '',
          turnstile_secret_key_configured: true,
        },
      },
      global: {
        stubs: {
          Toggle: { template: '<input type="checkbox" />' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.apiKeyAcl.title')
    expect(wrapper.text()).not.toContain('admin.settings.turnstile.title')
    expect(wrapper.find('input[placeholder="0x4AAAAAAA..."]').exists()).toBe(false)
  })
})
