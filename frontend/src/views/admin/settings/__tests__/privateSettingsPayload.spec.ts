import { describe, expect, it } from 'vitest'

import { stripPrivateProductSettings } from '../privateSettingsPayload'

describe('stripPrivateProductSettings', () => {
  it('removes customer, commercial, payment, and identity-provider settings', () => {
    const payload: Record<string, unknown> = {
      registration_enabled: true,
      promo_code_enabled: true,
      default_balance: 100,
      affiliate_enabled: true,
      payment_enabled: true,
      subscription_expiry_notify_enabled: true,
      github_oauth_enabled: true,
      oidc_connect_enabled: true,
      email_verify_enabled: true,
      turnstile_enabled: true,
      turnstile_site_key: 'site-key',
      turnstile_secret_key: 'secret-key',
      custom_menu_items: [{ id: 'docs' }],
      smtp_host: 'mail.internal',
      account_quota_notify_enabled: true,
      api_key_acl_trust_forwarded_ip: false,
      openai_advanced_scheduler_enabled: true,
    }

    stripPrivateProductSettings(payload)

    expect(payload).toEqual({
      smtp_host: 'mail.internal',
      account_quota_notify_enabled: true,
      api_key_acl_trust_forwarded_ip: false,
      openai_advanced_scheduler_enabled: true,
    })
  })
})