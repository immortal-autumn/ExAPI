import { describe, expect, it } from 'vitest'

import { buildPrivateOperatorSettings, stripPrivateProductSettings } from '../privateSettingsPayload'

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
      tencent_captcha_enabled: true,
      aliyun_captcha_scene_id: 'login',
      totp_enabled: true,
      passkey_enabled: true,
      session_binding_enabled: true,
      step_up_enabled: true,
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

  it('copies only explicitly reviewed private operator settings', () => {
    expect(buildPrivateOperatorSettings({
      site_name: 'ExAPI',
      smtp_password: undefined,
      risk_control_enabled: true,
      unknown_future_customer_setting: true,
      payment_enabled: true,
      totp_enabled: true,
      turnstile_secret_key: 'secret',
    })).toEqual({
      site_name: 'ExAPI',
      risk_control_enabled: true,
    })
  })
})
