import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/components/common/Toggle.vue', () => ({
  default: { template: '<button type="button">toggle</button>' },
}))

import SubscriptionExpiryNotificationPanel from '../email/SubscriptionExpiryNotificationPanel.vue'

describe('SubscriptionExpiryNotificationPanel', () => {
  it('renders subscription expiry notification setting', () => {
    const wrapper = mount(SubscriptionExpiryNotificationPanel, {
      props: {
        form: { subscription_expiry_notify_enabled: true },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.subscriptionExpiryNotify.title')
    expect(wrapper.text()).toContain('admin.settings.subscriptionExpiryNotify.enabled')
    expect(wrapper.text()).toContain('toggle')
  })
})
