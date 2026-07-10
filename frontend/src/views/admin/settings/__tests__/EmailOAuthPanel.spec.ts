import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    locale: { value: 'zh-CN' },
    t: (key: string) => key,
  }),
}))

import EmailOAuthPanel from '../panels/EmailOAuthPanel.vue'

describe('EmailOAuthPanel', () => {
  it('renders GitHub and Google OAuth fields and delegates redirect copy', async () => {
    const setAndCopyEmailOAuthRedirectUrl = vi.fn()

    const wrapper = mount(EmailOAuthPanel, {
      props: {
        form: {
          github_oauth_enabled: true,
          github_oauth_client_id: '',
          github_oauth_client_secret: '',
          github_oauth_client_secret_configured: true,
          github_oauth_redirect_url: '',
          github_oauth_frontend_redirect_url: '/auth/oauth/callback',
          google_oauth_enabled: true,
          google_oauth_client_id: '',
          google_oauth_client_secret: '',
          google_oauth_client_secret_configured: false,
          google_oauth_redirect_url: '',
          google_oauth_frontend_redirect_url: '/auth/oauth/callback',
        },
        isZhLocale: true,
        githubOAuthRedirectUrlSuggestion: 'https://example.com/api/v1/auth/oauth/github/callback',
        googleOAuthRedirectUrlSuggestion: 'https://example.com/api/v1/auth/oauth/google/callback',
        setAndCopyEmailOAuthRedirectUrl,
        localText: (zh: string, _en: string) => zh,
      },
      global: {
        stubs: {
          Toggle: { template: '<input type="checkbox" />' },
        },
      },
    })

    expect(wrapper.text()).toContain('GitHub')
    expect(wrapper.text()).toContain('Google')
    expect(wrapper.text()).toContain('https://example.com/api/v1/auth/oauth/github/callback')
    expect(wrapper.text()).toContain('https://example.com/api/v1/auth/oauth/google/callback')

    await wrapper.find('[data-test="copy-github-oauth-redirect-url"]').trigger('click')
    await wrapper.find('[data-test="copy-google-oauth-redirect-url"]').trigger('click')

    expect(setAndCopyEmailOAuthRedirectUrl).toHaveBeenNthCalledWith(1, 'github')
    expect(setAndCopyEmailOAuthRedirectUrl).toHaveBeenNthCalledWith(2, 'google')
  })
})
