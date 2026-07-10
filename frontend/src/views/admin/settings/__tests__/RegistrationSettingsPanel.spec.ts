import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import RegistrationSettingsPanel from '../panels/RegistrationSettingsPanel.vue'

describe('RegistrationSettingsPanel', () => {
  it('renders registration controls and delegates whitelist tag removal', async () => {
    const removeRegistrationEmailSuffixWhitelistTag = vi.fn()

    const wrapper = mount(RegistrationSettingsPanel, {
      props: {
        form: {
          registration_enabled: true,
          email_verify_enabled: true,
          promo_code_enabled: false,
          invitation_code_enabled: false,
          password_reset_enabled: true,
          frontend_url: 'https://example.com',
          totp_enabled: false,
          totp_encryption_key_configured: true,
        },
        registrationEmailSuffixWhitelistTags: ['warwick.ac.uk'],
        removeRegistrationEmailSuffixWhitelistTag,
        handleRegistrationEmailSuffixWhitelistDraftInput: vi.fn(),
        handleRegistrationEmailSuffixWhitelistDraftKeydown: vi.fn(),
        commitRegistrationEmailSuffixWhitelistDraft: vi.fn(),
        handleRegistrationEmailSuffixWhitelistPaste: vi.fn(),
        registrationEmailSuffixWhitelistDraft: '',
      },
      global: {
        stubs: {
          Toggle: { template: '<input type="checkbox" />' },
          Icon: { template: '<span />' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.registration.title')
    expect(wrapper.text()).toContain('warwick.ac.uk')

    await wrapper.find('[data-test="remove-registration-email-suffix-warwick.ac.uk"]').trigger('click')
    expect(removeRegistrationEmailSuffixWhitelistTag).toHaveBeenCalledWith('warwick.ac.uk')
  })
})
