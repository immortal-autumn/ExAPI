const PRIVATE_PRODUCT_FORBIDDEN_EXACT_KEYS = new Set([
  'registration_enabled',
  'registration_email_suffix_whitelist',
  'promo_code_enabled',
  'invitation_code_enabled',
  'password_reset_enabled',
  'login_agreement_enabled',
  'login_agreement_mode',
  'login_agreement_updated_at',
  'login_agreement_documents',
  'default_balance',
  'affiliate_admin_recharge_enabled',
  'default_concurrency',
  'default_subscriptions',
  'force_email_on_third_party_signup',
  'default_user_rpm_limit',
  'default_platform_quotas',
  'balance_low_notify_enabled',
  'balance_low_notify_threshold',
  'balance_low_notify_recharge_url',
  'subscription_expiry_notify_enabled',
  'available_channels_enabled',
  'affiliate_enabled',
  'email_verify_enabled',
  'custom_menu_items',
])

const PRIVATE_PRODUCT_FORBIDDEN_PREFIXES = [
  'affiliate_rebate_',
  'auth_source_default_',
  'payment_',
  'linuxdo_connect_',
  'dingtalk_connect_',
  'wechat_connect_',
  'oidc_connect_',
  'github_oauth_',
  'google_oauth_',
  'turnstile_',
] as const

export function stripPrivateProductSettings(payload: Record<string, unknown>): void {
  for (const key of Object.keys(payload)) {
    if (
      PRIVATE_PRODUCT_FORBIDDEN_EXACT_KEYS.has(key)
      || PRIVATE_PRODUCT_FORBIDDEN_PREFIXES.some((prefix) => key.startsWith(prefix))
    ) {
      delete payload[key]
    }
  }
}
