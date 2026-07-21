import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import SecuritySettingsTab from '../tabs/SecuritySettingsTab.vue'

describe('SecuritySettingsTab', () => {
  it('renders only operator security panels in the private product', () => {
    const noop = vi.fn()
    const localText = (zh: string, _en: string) => zh

    const wrapper = mount(SecuritySettingsTab, {
      props: {
        registrationEmailSuffixWhitelistDraft: '',
        form: {},
        adminApiKeyLoading: false,
        adminApiKeyExists: true,
        adminApiKeyOperating: false,
        adminApiKeyMasked: 'sk-***',
        newAdminApiKey: '',
        createAdminApiKey: noop,
        regenerateAdminApiKey: noop,
        deleteAdminApiKey: noop,
        copyNewKey: noop,
        registrationEmailSuffixWhitelistTags: [],
        removeRegistrationEmailSuffixWhitelistTag: noop,
        handleRegistrationEmailSuffixWhitelistDraftInput: noop,
        handleRegistrationEmailSuffixWhitelistDraftKeydown: noop,
        commitRegistrationEmailSuffixWhitelistDraft: noop,
        handleRegistrationEmailSuffixWhitelistPaste: noop,
        linuxdoRedirectUrlSuggestion: '',
        setAndCopyLinuxdoRedirectUrl: noop,
        isZhLocale: true,
        githubOAuthRedirectUrlSuggestion: '',
        googleOAuthRedirectUrlSuggestion: '',
        setAndCopyEmailOAuthRedirectUrl: noop,
        wechatRedirectUrlSuggestion: '',
        setAndCopyWeChatRedirectUrl: noop,
        handleWeChatOpenEnabledChange: noop,
        handleWeChatMPEnabledChange: noop,
        handleWeChatMobileEnabledChange: noop,
        oidcRedirectUrlSuggestion: '',
        setAndCopyOIDCRedirectUrl: noop,
        localText,
      },
      global: {
        stubs: {
          SecurityAdminApiKeyPanel: { template: '<section>admin-api-key-panel</section>' },
          RegistrationSettingsPanel: { template: '<section>registration-panel</section>' },
          SecurityAccessControlsPanel: { template: '<section>access-controls-panel</section>' },
          LinuxDoOAuthPanel: { template: '<section>linuxdo-panel</section>' },
          EmailOAuthPanel: { template: '<section>email-oauth-panel</section>' },
          WeChatConnectPanel: { template: '<section>wechat-panel</section>' },
          DingTalkConnectPanel: { template: '<section>dingtalk-panel</section>' },
          OidcConnectPanel: { template: '<section>oidc-panel</section>' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin-api-key-panel')
    expect(wrapper.text()).toContain('access-controls-panel')
    expect(wrapper.text()).not.toContain('admin.settings.registration.title')
    expect(wrapper.text()).not.toContain('admin.settings.linuxdo.title')
    expect(wrapper.text()).not.toContain('邮箱快捷登录')
    expect(wrapper.text()).not.toContain('admin.settings.wechatConnect.title')
    expect(wrapper.text()).not.toContain('admin.settings.dingtalk.title')
    expect(wrapper.text()).not.toContain('admin.settings.oidc.title')
  })
})
