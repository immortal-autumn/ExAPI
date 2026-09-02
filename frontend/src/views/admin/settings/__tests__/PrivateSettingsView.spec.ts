import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const {
  getSettings,
  updateSettings,
  testSmtpConnection,
  sendTestEmail,
  getOverloadCooldownSettings,
  getRateLimit429CooldownSettings,
  getStreamTimeoutSettings,
  getRectifierSettings,
  adminSettingsFetch,
  publicSettingsFetch,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  testSmtpConnection: vi.fn(),
  sendTestEmail: vi.fn(),
  getOverloadCooldownSettings: vi.fn(),
  getRateLimit429CooldownSettings: vi.fn(),
  getStreamTimeoutSettings: vi.fn(),
  getRectifierSettings: vi.fn(),
  adminSettingsFetch: vi.fn(),
  publicSettingsFetch: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/operator', () => ({
  operatorAPI: {
    settings: {
      getSettings,
      updateSettings,
      testSmtpConnection,
      sendTestEmail,
      getOverloadCooldownSettings,
      getRateLimit429CooldownSettings,
      getStreamTimeoutSettings,
      getRectifierSettings,
      updateOverloadCooldownSettings: vi.fn(),
      updateRateLimit429CooldownSettings: vi.fn(),
      updateStreamTimeoutSettings: vi.fn(),
      updateRectifierSettings: vi.fn(),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    fetchPublicSettings: publicSettingsFetch,
    showError,
    showSuccess,
  }),
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({ fetch: adminSettingsFetch }),
}))

vi.mock('vue-i18n', async () => ({
  ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')),
  useI18n: () => ({ t: (key: string) => key }),
}))

import PrivateSettingsView from '../../PrivateSettingsView.vue'

const EmailSettingsTabStub = {
  props: [
    'form',
    'testSmtpConnection',
    'markSmtpPasswordManuallyEdited',
    'sendTestEmail',
    'updateTestEmailAddress',
  ],
  template: `
    <div>
      <button type="button" data-test="test-smtp" @click="testSmtpConnection">test smtp</button>
      <button
        type="button"
        data-test="edit-smtp-password"
        @click="form.smtp_password = 'new-smtp-secret'; markSmtpPasswordManuallyEdited()"
      >edit password</button>
      <button
        type="button"
        data-test="send-test-email"
        @click="updateTestEmailAddress('operator@example.com'); sendTestEmail()"
      >send test email</button>
    </div>
  `,
}

const mountView = async () => {
  const wrapper = mount(PrivateSettingsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        GeneralSettingsTab: true,
        EmailSettingsTab: EmailSettingsTabStub,
        SecuritySettingsTab: { template: '<section data-testid="private-security-tab">security tab</section>' },
        BackupSettingsTab: true,
        GatewayCooldownPanel: true,
        GatewayStreamTimeoutPanel: true,
        GatewayRectifierPanel: true,
        Toggle: true,
      },
    },
  })
  await flushPromises()
  return wrapper
}

describe('PrivateSettingsView controller', () => {
  beforeEach(() => {
    for (const mock of [
      getSettings,
      updateSettings,
      testSmtpConnection,
      sendTestEmail,
      getOverloadCooldownSettings,
      getRateLimit429CooldownSettings,
      getStreamTimeoutSettings,
      getRectifierSettings,
      adminSettingsFetch,
      publicSettingsFetch,
      showError,
      showSuccess,
    ]) {
      mock.mockReset()
    }

    getSettings.mockResolvedValue({
      site_name: 'Private ExAPI',
      table_default_page_size: 20,
      table_page_size_options: [10, 20, 50, 100],
      smtp_password_configured: true,
      smtp_host: 'smtp.example.com',
      smtp_port: 465,
      smtp_username: 'operator',
      smtp_use_tls: true,
      smtp_from_email: 'exapi@example.com',
      smtp_from_name: 'ExAPI',
      openai_advanced_scheduler_enabled: true,
      enable_metadata_passthrough: true,
      payment_enabled: true,
      registration_enabled: true,
    })
    getOverloadCooldownSettings.mockResolvedValue({ enabled: true, cooldown_minutes: 10 })
    getRateLimit429CooldownSettings.mockResolvedValue({ enabled: true, cooldown_seconds: 5 })
    getStreamTimeoutSettings.mockResolvedValue({
      enabled: true,
      action: 'temp_unsched',
      temp_unsched_minutes: 5,
      threshold_count: 3,
      threshold_window_minutes: 10,
    })
    getRectifierSettings.mockResolvedValue({
      enabled: true,
      thinking_signature_enabled: true,
      thinking_budget_enabled: true,
      apikey_signature_enabled: false,
      apikey_signature_patterns: [],
    })
    updateSettings.mockResolvedValue({ site_name: 'Private ExAPI' })
    adminSettingsFetch.mockResolvedValue(undefined)
    publicSettingsFetch.mockResolvedValue(null)
  })

  it('saves allowlisted gateway settings, omits untouched SMTP secrets, and refreshes stores', async () => {
    const wrapper = await mountView()

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(updateSettings).toHaveBeenCalledTimes(1)
    const payload = updateSettings.mock.calls[0]?.[0]
    expect(payload).toEqual(expect.objectContaining({
      site_name: 'Private ExAPI',
      openai_advanced_scheduler_enabled: true,
      enable_metadata_passthrough: true,
    }))
    expect(payload).not.toHaveProperty('smtp_password')
    expect(payload).not.toHaveProperty('payment_enabled')
    expect(payload).not.toHaveProperty('registration_enabled')
    expect(adminSettingsFetch).toHaveBeenCalledWith(true)
    expect(publicSettingsFetch).toHaveBeenCalledWith(true)
    expect(showSuccess).toHaveBeenCalledWith('admin.settings.saved')
  })

  it('keeps the private security tab available without exposing SaaS settings', async () => {
    const wrapper = await mountView()

    const tabs = wrapper.findAll('[role="tab"]')
    expect(tabs).toHaveLength(5)
    expect(tabs[3]?.text()).toBe('admin.settings.tabs.security')

    await tabs[3]?.trigger('click')
    expect(wrapper.get('[data-testid="private-security-tab"]').exists()).toBe(true)
    expect(wrapper.find('button[type="submit"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('admin.settings.tabs.payment')
    expect(wrapper.text()).not.toContain('admin.settings.tabs.users')
  })

  it('reports save failures without refreshing either settings store', async () => {
    updateSettings.mockRejectedValueOnce(new Error('private settings write failed'))
    const wrapper = await mountView()

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('private settings write failed')
    expect(adminSettingsFetch).not.toHaveBeenCalled()
    expect(publicSettingsFetch).not.toHaveBeenCalled()
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })

  it('delegates SMTP probes and test email through the private operator API', async () => {
    testSmtpConnection.mockResolvedValue({ message: 'smtp ready' })
    sendTestEmail.mockResolvedValue({ message: 'email sent' })
    const wrapper = await mountView()

    await wrapper.findAll('[role="tab"]')[2].trigger('click')
    await wrapper.get('[data-test="test-smtp"]').trigger('click')
    await wrapper.get('[data-test="send-test-email"]').trigger('click')
    await flushPromises()

    expect(testSmtpConnection).toHaveBeenCalledWith({
      smtp_host: 'smtp.example.com',
      smtp_port: 465,
      smtp_username: 'operator',
      smtp_password: '',
      smtp_use_tls: true,
    })
    expect(sendTestEmail).toHaveBeenCalledWith(expect.objectContaining({
      email: 'operator@example.com',
      smtp_from_email: 'exapi@example.com',
      smtp_from_name: 'ExAPI',
    }))
    expect(showSuccess).toHaveBeenCalledWith('smtp ready')
    expect(showSuccess).toHaveBeenCalledWith('email sent')
  })

  it('includes an explicitly replaced SMTP password and reports one test-email error', async () => {
    sendTestEmail.mockRejectedValueOnce(new Error('test email failed'))
    const wrapper = await mountView()

    await wrapper.findAll('[role="tab"]')[2].trigger('click')
    await wrapper.get('[data-test="edit-smtp-password"]').trigger('click')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(updateSettings.mock.calls[0]?.[0]).toEqual(expect.objectContaining({ smtp_password: 'new-smtp-secret' }))

    await wrapper.get('[data-test="send-test-email"]').trigger('click')
    await flushPromises()
    expect(showError).toHaveBeenCalledTimes(1)
    expect(showError).toHaveBeenCalledWith('test email failed')
  })
})
