import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))
vi.mock('@/components/icons/Icon.vue', () => ({ default: { template: '<span>mail-icon</span>' } }))
vi.mock('@/views/admin/settings/email/SmtpSettingsPanel.vue', () => ({ default: { template: '<div>smtp-settings-panel</div>' } }))
vi.mock('@/views/admin/settings/email/TestEmailPanel.vue', () => ({ default: { template: '<div>test-email-panel</div>' } }))
vi.mock('@/views/admin/settings/email/SubscriptionExpiryNotificationPanel.vue', () => ({ default: { template: '<div>subscription-expiry-panel</div>' } }))
vi.mock('@/views/admin/settings/EmailTemplateEditor.vue', () => ({ default: { template: '<div>email-template-editor</div>' } }))
vi.mock('@/views/admin/settings/email/BalanceLowNotificationPanel.vue', () => ({ default: { template: '<div>balance-low-panel</div>' } }))
vi.mock('@/views/admin/settings/email/AccountQuotaNotificationPanel.vue', () => ({ default: { template: '<div>account-quota-panel</div>' } }))

import EmailSettingsTab from '../tabs/EmailSettingsTab.vue'

describe('EmailSettingsTab', () => {
  it('does not render customer email-verification state', () => {
    const wrapper = mount(EmailSettingsTab, {
      props: {
        form: { email_verify_enabled: false },
        testingSmtp: false,
        loadFailed: false,
        testSmtpConnection: vi.fn(),
        markSmtpPasswordManuallyEdited: vi.fn(),
        testEmailAddress: '',
        sendingTestEmail: false,
        sendTestEmail: vi.fn(),
        updateTestEmailAddress: vi.fn(),
        currentOrigin: 'https://example.test',
        addQuotaNotifyEmail: vi.fn(),
        removeQuotaNotifyEmail: vi.fn(),
      },
    })

    expect(wrapper.text()).not.toContain('admin.settings.emailTabDisabledTitle')
    expect(wrapper.text()).toContain('smtp-settings-panel')
  })

  it('renders only operator email panels when email is enabled', () => {
    const wrapper = mount(EmailSettingsTab, {
      props: {
        form: { email_verify_enabled: true },
        testingSmtp: false,
        loadFailed: false,
        testSmtpConnection: vi.fn(),
        markSmtpPasswordManuallyEdited: vi.fn(),
        testEmailAddress: 'admin@example.test',
        sendingTestEmail: false,
        sendTestEmail: vi.fn(),
        updateTestEmailAddress: vi.fn(),
        currentOrigin: 'https://example.test',
        addQuotaNotifyEmail: vi.fn(),
        removeQuotaNotifyEmail: vi.fn(),
      },
    })

    expect(wrapper.text()).toContain('smtp-settings-panel')
    expect(wrapper.text()).toContain('test-email-panel')
    expect(wrapper.text()).toContain('account-quota-panel')
    expect(wrapper.text()).not.toContain('subscription-expiry-panel')
    expect(wrapper.text()).not.toContain('email-template-editor')
    expect(wrapper.text()).not.toContain('balance-low-panel')
  })
})
