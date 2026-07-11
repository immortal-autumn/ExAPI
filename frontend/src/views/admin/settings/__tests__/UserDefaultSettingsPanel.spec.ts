import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/components/common/Select.vue', () => ({
  default: { template: '<select><slot name="selected" :option="null" /><slot name="option" :option="{}" :selected="false" /></select>' },
}))
vi.mock('@/components/common/GroupBadge.vue', () => ({ default: { template: '<span>group-badge</span>' } }))
vi.mock('@/components/common/GroupOptionItem.vue', () => ({ default: { template: '<span>group-option</span>' } }))

import UserDefaultSettingsPanel from '../users/UserDefaultSettingsPanel.vue'

describe('UserDefaultSettingsPanel', () => {
  it('renders user defaults and delegates subscription actions', async () => {
    const addDefaultSubscription = vi.fn()
    const removeDefaultSubscription = vi.fn()
    const form = {
      default_balance: 10,
      default_concurrency: 2,
      default_user_rpm_limit: 60,
      default_subscriptions: [{ group_id: 1, validity_days: 30 }],
      default_platform_quotas: {
        anthropic: { daily: 1, weekly: 2, monthly: 3 },
        openai: { daily: 0, weekly: 0, monthly: 0 },
        gemini: { daily: 0, weekly: 0, monthly: 0 },
        antigravity: { daily: 0, weekly: 0, monthly: 0 },
        grok: { daily: 0, weekly: 0, monthly: 0 },
      },
    }

    const wrapper = mount(UserDefaultSettingsPanel, {
      props: {
        form,
        subscriptionGroups: [{ id: 1 }],
        defaultSubscriptionGroupOptions: [{ value: 1, label: 'default', platform: 'openai' }],
        addDefaultSubscription,
        removeDefaultSubscription,
      },
    })

    expect(wrapper.text()).toContain('admin.settings.defaults.title')
    expect(wrapper.text()).toContain('admin.settings.defaults.defaultBalance')
    expect(wrapper.text()).toContain('admin.settings.defaults.defaultSubscriptions')
    expect(wrapper.text()).toContain('admin.settings.defaults.defaultPlatformQuotas')

    await wrapper.findAll('button').find((button) => button.text().includes('admin.settings.defaults.addDefaultSubscription'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('common.delete'))!.trigger('click')

    expect(addDefaultSubscription).toHaveBeenCalledTimes(1)
    expect(removeDefaultSubscription).toHaveBeenCalledWith(0)
  })
})
