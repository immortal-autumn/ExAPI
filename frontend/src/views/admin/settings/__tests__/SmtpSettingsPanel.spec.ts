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

import SmtpSettingsPanel from '../email/SmtpSettingsPanel.vue'

describe('SmtpSettingsPanel', () => {
  it('renders SMTP settings when email verification is enabled and forwards test/password edit events', async () => {
    const testSmtpConnection = vi.fn()
    const markSmtpPasswordManuallyEdited = vi.fn()
    const form = {
      email_verify_enabled: true,
      smtp_host: 'smtp.example.test',
      smtp_port: 587,
      smtp_username: 'mailer',
      smtp_password: '',
      smtp_password_configured: true,
      smtp_from_email: 'noreply@example.test',
      smtp_from_name: 'ExAPI',
      smtp_use_tls: true,
    }

    const wrapper = mount(SmtpSettingsPanel, {
      props: {
        form,
        testingSmtp: false,
        loadFailed: false,
        testSmtpConnection,
        markSmtpPasswordManuallyEdited,
      },
    })

    expect(wrapper.text()).toContain('admin.settings.smtp.title')
    expect(wrapper.find('input[type="password"]').attributes('autocomplete')).toBe('new-password')

    await wrapper.find('button.btn').trigger('click')
    await wrapper.find('input[type="password"]').trigger('keydown')
    await wrapper.find('input[type="password"]').trigger('paste')

    expect(testSmtpConnection).toHaveBeenCalledOnce()
    expect(markSmtpPasswordManuallyEdited).toHaveBeenCalledTimes(2)
  })
})
