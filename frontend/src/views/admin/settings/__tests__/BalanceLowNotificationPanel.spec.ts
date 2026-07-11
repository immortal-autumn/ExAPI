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

import BalanceLowNotificationPanel from '../email/BalanceLowNotificationPanel.vue'

describe('BalanceLowNotificationPanel', () => {
  it('renders balance notification fields when enabled', () => {
    const form = {
      balance_low_notify_enabled: true,
      balance_low_notify_threshold: 5,
      balance_low_notify_recharge_url: 'https://example.test/recharge',
    }
    const wrapper = mount(BalanceLowNotificationPanel, {
      props: {
        form,
        currentOrigin: 'https://example.test',
      },
    })

    expect(wrapper.text()).toContain('admin.settings.balanceNotify.title')
    expect(wrapper.text()).toContain('admin.settings.balanceNotify.threshold')
    expect((wrapper.find('input[type="url"]').element as HTMLInputElement).value).toBe('https://example.test/recharge')
  })
})
