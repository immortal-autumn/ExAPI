/**
 * Settings that remain meaningful in the private operator product. Keeping an
 * allowlist here makes newly added SaaS/auth settings fail closed: they cannot
 * be written by the private browser until they are reviewed and added.
 */
export const PRIVATE_OPERATOR_SETTING_KEYS = [
  'audit_log_retention_days',
  'site_name',
  'site_logo',
  'site_subtitle',
  'api_base_url',
  'contact_info',
  'doc_url',
  'home_content',
  'backend_mode_enabled',
  'hide_ccs_import_button',
  'table_default_page_size',
  'table_page_size_options',
  'custom_endpoints',
  'frontend_url',
  'smtp_host',
  'smtp_port',
  'smtp_username',
  'smtp_password',
  'smtp_from_email',
  'smtp_from_name',
  'smtp_use_tls',
  'api_key_acl_trust_forwarded_ip',
  'forwarded_client_ip_headers',
  'risk_control_enabled',
  'cyber_session_block_enabled',
  'cyber_session_block_ttl_seconds',
  'enable_model_fallback',
  'fallback_model_anthropic',
  'fallback_model_openai',
  'fallback_model_gemini',
  'fallback_model_antigravity',
  'enable_identity_patch',
  'identity_patch_prompt',
  'ops_monitoring_enabled',
  'ops_realtime_monitoring_enabled',
  'ops_query_mode_default',
  'ops_metrics_interval_seconds',
  'min_claude_code_version',
  'max_claude_code_version',
  'allow_ungrouped_key_scheduling',
  'openai_advanced_scheduler_enabled',
  'openai_advanced_scheduler_sticky_weighted_enabled',
  'openai_advanced_scheduler_subscription_priority_enabled',
  'openai_advanced_scheduler_lb_top_k',
  'openai_advanced_scheduler_weight_priority',
  'openai_advanced_scheduler_weight_load',
  'openai_advanced_scheduler_weight_queue',
  'openai_advanced_scheduler_weight_error_rate',
  'openai_advanced_scheduler_weight_ttft',
  'openai_advanced_scheduler_weight_reset',
  'openai_advanced_scheduler_weight_quota_headroom',
  'openai_advanced_scheduler_weight_upstream_cost',
  'openai_advanced_scheduler_weight_previous_response',
  'openai_advanced_scheduler_weight_session_sticky',
  'openai_fast_policy_settings',
  'enable_fingerprint_unification',
  'enable_metadata_passthrough',
  'enable_cch_signing',
  'enable_claude_oauth_system_prompt_injection',
  'claude_oauth_system_prompt',
  'claude_oauth_system_prompt_blocks',
  'enable_anthropic_cache_ttl_1h_injection',
  'rewrite_message_cache_control',
  'enable_client_dateline_normalization',
  'antigravity_user_agent_version',
  'openai_codex_user_agent',
  'min_codex_version',
  'max_codex_version',
  'codex_cli_only_blacklist',
  'codex_cli_only_whitelist',
  'codex_cli_only_allow_app_server_clients',
  'codex_cli_only_engine_fingerprint_signals',
  'account_quota_notify_enabled',
  'account_quota_notify_emails',
  'channel_monitor_enabled',
  'channel_monitor_default_interval_seconds',
  'allow_user_view_error_requests',
] as const

const PRIVATE_OPERATOR_SETTING_KEY_SET = new Set<string>(PRIVATE_OPERATOR_SETTING_KEYS)

export type PrivateOperatorSettingKey = typeof PRIVATE_OPERATOR_SETTING_KEYS[number]

export function buildPrivateOperatorSettings(
  source: Record<string, unknown>,
): Record<string, unknown> {
  const payload: Record<string, unknown> = {}
  for (const key of PRIVATE_OPERATOR_SETTING_KEYS) {
    if (Object.prototype.hasOwnProperty.call(source, key) && source[key] !== undefined) {
      payload[key] = source[key]
    }
  }
  return payload
}

/** Compatibility helper for legacy callers; it now enforces the allowlist. */
export function stripPrivateProductSettings(payload: Record<string, unknown>): void {
  for (const key of Object.keys(payload)) {
    if (!PRIVATE_OPERATOR_SETTING_KEY_SET.has(key)) delete payload[key]
  }
}
