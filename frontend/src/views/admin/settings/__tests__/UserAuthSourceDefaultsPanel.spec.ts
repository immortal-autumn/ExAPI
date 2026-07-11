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
vi.mock('@/components/common/Select.vue', () => ({
  default: { template: '<select><slot name="selected" :option="null" /><slot name="option" :option="{}" :selected="false" /></select>' },
}))
vi.mock('@/components/common/GroupBadge.vue', () => ({ default: { template: '<span>group-badge</span>' } }))
vi.mock('@/components/common/GroupOptionItem.vue', () => ({ default: { template: '<span>group-option</span>' } }))

import UserAuthSourceDefaultsPanel from '../users/UserAuthSourceDefaultsPanel.vue'

describe('UserAuthSourceDefaultsPanel', () => {
  it('renders auth source defaults and delegates subscription actions', async () => {
    const addAuthSourceDefaultSubscription = vi.fn()
    const removeAuthSourceDefaultSubscription = vi.fn()
    const form = { force_email_on_third_party_signup: true }
    const authSourceDefaults = {
      email: {
        grant_on_signup: true,
        balance: 10,
        concurrency: 2,
        grant_on_first_bind: true,
        subscriptions: [{ group_id: 1, validity_days: 30 }],
        platform_quotas: {
          anthropic: { daily: 1, weekly: 2, monthly: 3 },
          openai: { daily: 0, weekly: 0, monthly: 0 },
          gemini: { daily: 0, weekly: 0, monthly: 0 },
          antigravity: { daily: 0, weekly: 0, monthly: 0 },
          grok: { daily: 0, weekly: 0, monthly: 0 },
        },
      },
    }

    const wrapper = mount(UserAuthSourceDefaultsPanel, {
      props: {
        form,
        authSourceDefaults,
        authSourceDefaultsMeta: [
          { source: 'email', title: 'Email', description: 'Email source' },
        ],
        subscriptionGroups: [{ id: 1 }],
        defaultSubscriptionGroupOptions: [{ value: 1, label: 'default', platform: 'openai' }],
        addAuthSourceDefaultSubscription,
        removeAuthSourceDefaultSubscription,
      },
    })

    expect(wrapper.text()).toContain('admin.settings.authSourceDefaults.title')
    expect(wrapper.text()).toContain('admin.settings.authSourceDefaults.requireEmailLabel')
    expect(wrapper.text()).toContain('Email')
    expect(wrapper.text()).toContain('admin.settings.authSourceDefaults.defaultSubscriptionsLabel')
    expect(wrapper.text()).toContain('admin.settings.authSourceDefaults.platformQuotasOverride')

    await wrapper.findAll('button').find((button) => button.text().includes('admin.settings.defaults.addDefaultSubscription'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('common.delete'))!.trigger('click')

    expect(addAuthSourceDefaultSubscription).toHaveBeenCalledWith('email')
    expect(removeAuthSourceDefaultSubscription).toHaveBeenCalledWith('email', 0)
  })
})
