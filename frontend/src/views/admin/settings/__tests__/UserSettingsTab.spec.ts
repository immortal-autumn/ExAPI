import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/views/admin/settings/users/UserDefaultSettingsPanel.vue', () => ({
  default: { template: '<div>user-default-settings-panel</div>' },
}))

vi.mock('@/views/admin/settings/users/UserAuthSourceDefaultsPanel.vue', () => ({
  default: { template: '<div>user-auth-source-defaults-panel</div>' },
}))

import UserSettingsTab from '../tabs/UserSettingsTab.vue'

describe('UserSettingsTab', () => {
  it('renders user settings panels', () => {
    const fn = vi.fn()
    const wrapper = mount(UserSettingsTab, {
      props: {
        form: { force_email_on_third_party_signup: true },
        subscriptionGroups: [],
        defaultSubscriptionGroupOptions: [],
        addDefaultSubscription: fn,
        removeDefaultSubscription: fn,
        authSourceDefaults: {},
        authSourceDefaultsMeta: [],
        addAuthSourceDefaultSubscription: fn,
        removeAuthSourceDefaultSubscription: fn,
      },
      global: {
        stubs: {
          UserDefaultSettingsPanel: { template: '<div>user-default-settings-panel</div>' },
          UserAuthSourceDefaultsPanel: { template: '<div>user-auth-source-defaults-panel</div>' },
        },
      },
    })

    expect(wrapper.text()).toContain('user-default-settings-panel')
    expect(wrapper.text()).toContain('user-auth-source-defaults-panel')
  })
})
