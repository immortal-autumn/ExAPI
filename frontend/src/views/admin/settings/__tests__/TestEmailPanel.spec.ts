import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import TestEmailPanel from '../email/TestEmailPanel.vue'

describe('TestEmailPanel', () => {
  it('renders recipient input and forwards send action', async () => {
    const sendTestEmail = vi.fn()
    const updateTestEmailAddress = vi.fn()
    const wrapper = mount(TestEmailPanel, {
      props: {
        form: { email_verify_enabled: true },
        testEmailAddress: 'admin@example.test',
        sendingTestEmail: false,
        loadFailed: false,
        sendTestEmail,
        updateTestEmailAddress,
      },
    })

    expect(wrapper.text()).toContain('admin.settings.testEmail.title')
    expect((wrapper.find('input[type="email"]').element as HTMLInputElement).value).toBe('admin@example.test')

    await wrapper.find('button.btn').trigger('click')
    expect(sendTestEmail).toHaveBeenCalledOnce()
  })
})
