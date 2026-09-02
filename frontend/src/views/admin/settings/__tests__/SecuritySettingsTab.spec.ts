import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import SecuritySettingsTab from '../tabs/SecuritySettingsTab.vue'

describe('SecuritySettingsTab', () => {
  it('renders the localized private-boundary contract', () => {
    const wrapper = mount(SecuritySettingsTab, {
      global: {
        stubs: {
          Icon: { template: '<span />' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.security.privateBoundaryTitle')
    expect(wrapper.text()).toContain('admin.settings.security.privateBoundaryDescription')
    expect(wrapper.text()).toContain('admin.settings.security.requestProtectionsTitle')
    expect(wrapper.text()).not.toContain('Private operator boundary')
    expect(wrapper.text()).not.toContain('Request protections')
  })
})
