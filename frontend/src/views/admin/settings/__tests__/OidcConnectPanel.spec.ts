import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import OidcConnectPanel from '../panels/OidcConnectPanel.vue'

describe('OidcConnectPanel', () => {
  it('renders OIDC provider fields and delegates redirect copy', async () => {
    const setAndCopyOIDCRedirectUrl = vi.fn()

    const wrapper = mount(OidcConnectPanel, {
      props: {
        form: {
          oidc_connect_enabled: true,
          oidc_connect_provider_name: 'Example IDP',
          oidc_connect_client_id: '',
          oidc_connect_client_secret: '',
          oidc_connect_client_secret_configured: true,
          oidc_connect_issuer_url: '',
          oidc_connect_discovery_url: '',
          oidc_connect_authorize_url: '',
          oidc_connect_token_url: '',
          oidc_connect_userinfo_url: '',
          oidc_connect_jwks_url: '',
          oidc_connect_scopes: 'openid email profile',
          oidc_connect_redirect_url: '',
          oidc_connect_frontend_redirect_url: '/auth/oauth/callback',
          oidc_connect_token_auth_method: 'client_secret_post',
          oidc_connect_clock_skew_seconds: 60,
          oidc_connect_allowed_signing_algs: 'RS256',
          oidc_connect_use_pkce: true,
          oidc_connect_validate_id_token: true,
          oidc_connect_require_email_verified: true,
          oidc_connect_userinfo_email_path: 'email',
          oidc_connect_userinfo_id_path: 'sub',
          oidc_connect_userinfo_username_path: 'preferred_username',
        },
        oidcRedirectUrlSuggestion: 'https://example.com/api/v1/auth/oauth/oidc/callback',
        setAndCopyOIDCRedirectUrl,
      },
      global: {
        stubs: {
          Toggle: { template: '<input type="checkbox" />' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.oidc.title')
    expect(wrapper.text()).toContain('admin.settings.oidc.providerName')
    expect(wrapper.text()).toContain('https://example.com/api/v1/auth/oauth/oidc/callback')
    expect(wrapper.find('select').exists()).toBe(true)

    await wrapper.find('[data-test="copy-oidc-redirect-url"]').trigger('click')
    expect(setAndCopyOIDCRedirectUrl).toHaveBeenCalledTimes(1)
  })
})
