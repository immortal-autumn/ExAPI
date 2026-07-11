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
vi.mock('@/components/icons/Icon.vue', () => ({
  default: { template: '<span>x-icon</span>' },
}))

import AccountQuotaNotificationPanel from '../email/AccountQuotaNotificationPanel.vue'

describe('AccountQuotaNotificationPanel', () => {
  it('renders quota notification emails and delegates add/remove actions', async () => {
    const addQuotaNotifyEmail = vi.fn()
    const removeQuotaNotifyEmail = vi.fn()
    const form = {
      account_quota_notify_enabled: true,
      account_quota_notify_emails: [{ email: 'ops@example.test', disabled: false }],
    }
    const wrapper = mount(AccountQuotaNotificationPanel, {
      props: {
        form,
        addQuotaNotifyEmail,
        removeQuotaNotifyEmail,
      },
    })

    expect(wrapper.text()).toContain('admin.settings.quotaNotify.title')
    expect((wrapper.find('input[type="email"]').element as HTMLInputElement).value).toBe('ops@example.test')

    await wrapper.findAll('button').find((button) => button.text().includes('admin.settings.quotaNotify.addEmail'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('x-icon'))!.trigger('click')

    expect(addQuotaNotifyEmail).toHaveBeenCalledOnce()
    expect(removeQuotaNotifyEmail).toHaveBeenCalledWith(0)
  })
})
