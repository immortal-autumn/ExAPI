import dashboardAPI from '../admin/dashboard'
import groupsAPI from '../admin/groups'
import accountsAPI from '../admin/accounts'
import proxiesAPI from '../admin/proxies'
import {
  getBetaPolicySettings,
  getEmailTemplate,
  getEmailTemplates,
  getOverloadCooldownSettings,
  getPanelRateLimitSettings,
  getRateLimit429CooldownSettings,
  getRectifierSettings,
  getSettings,
  getStreamTimeoutSettings,
  getWebSearchEmulationConfig,
  previewEmailTemplate,
  resetWebSearchUsage,
  restoreOfficialEmailTemplate,
  sendTestEmail,
  testSmtpConnection,
  testWebSearchEmulation,
  updateBetaPolicySettings,
  updateEmailTemplate,
  updateOverloadCooldownSettings,
  updatePanelRateLimitSettings,
  updateRateLimit429CooldownSettings,
  updateRectifierSettings,
  updateSettings,
  updateStreamTimeoutSettings,
  updateWebSearchEmulationConfig,
} from '../admin/settings'
import { getVersion } from '../admin/system'
import usageAPI from '../admin/usage'
import geminiAPI from '../admin/gemini'
import antigravityAPI from '../admin/antigravity'
import grokAPI from '../admin/grok'
import opsAPI from '../admin/ops'
import errorPassthroughAPI from '../admin/errorPassthrough'
import dataManagementAPI from '../admin/dataManagement'
import apiKeysAPI from '../admin/apiKeys'
import scheduledTestsAPI from '../admin/scheduledTests'
import {
  createBackup,
  deleteBackup,
  getBackup,
  getDownloadURL,
  getImageStorageConfig,
  getS3Config,
  getSchedule,
  listBackups,
  testImageStorageConnection,
  testS3Connection,
  updateImageStorageConfig,
  updateS3Config,
  updateSchedule,
} from '../admin/backup'
import tlsFingerprintProfileAPI from '../admin/tlsFingerprintProfile'
import channelsAPI from '../admin/channels'
import channelMonitorAPI from '../admin/channelMonitor'
import channelMonitorTemplateAPI from '../admin/channelMonitorTemplate'
import riskControlAPI from '../admin/riskControl'
import auditAPI from '../admin/audit'
import cockpitAPI from '../admin/cockpit'
import usersAPI from './users'

const backupAPI = {
  getS3Config,
  updateS3Config,
  testS3Connection,
  getImageStorageConfig,
  updateImageStorageConfig,
  testImageStorageConnection,
  getSchedule,
  updateSchedule,
  createBackup,
  listBackups,
  getBackup,
  deleteBackup,
  getDownloadURL,
}

const systemAPI = { getVersion }

const settingsAPI = {
  getSettings,
  updateSettings,
  testSmtpConnection,
  sendTestEmail,
  getEmailTemplates,
  getEmailTemplate,
  updateEmailTemplate,
  restoreOfficialEmailTemplate,
  previewEmailTemplate,
  getOverloadCooldownSettings,
  updateOverloadCooldownSettings,
  getRateLimit429CooldownSettings,
  updateRateLimit429CooldownSettings,
  getPanelRateLimitSettings,
  updatePanelRateLimitSettings,
  getStreamTimeoutSettings,
  updateStreamTimeoutSettings,
  getRectifierSettings,
  updateRectifierSettings,
  getBetaPolicySettings,
  updateBetaPolicySettings,
  getWebSearchEmulationConfig,
  updateWebSearchEmulationConfig,
  testWebSearchEmulation,
  resetWebSearchUsage,
}

/** API facade reachable from the private operator route graph. */
export const operatorAPI = {
  dashboard: dashboardAPI,
  groups: groupsAPI,
  accounts: accountsAPI,
  proxies: proxiesAPI,
  settings: settingsAPI,
  system: systemAPI,
  usage: usageAPI,
  gemini: geminiAPI,
  antigravity: antigravityAPI,
  grok: grokAPI,
  ops: opsAPI,
  errorPassthrough: errorPassthroughAPI,
  dataManagement: dataManagementAPI,
  apiKeys: apiKeysAPI,
  scheduledTests: scheduledTestsAPI,
  backup: backupAPI,
  tlsFingerprintProfiles: tlsFingerprintProfileAPI,
  channels: channelsAPI,
  channelMonitor: channelMonitorAPI,
  channelMonitorTemplate: channelMonitorTemplateAPI,
  riskControl: riskControlAPI,
  audit: auditAPI,
  cockpit: cockpitAPI,
  users: usersAPI,
}

export type { AuditLog, AuditLogQuery, AuditLogListResponse } from '../admin/audit'
export type { ErrorPassthroughRule, CreateRuleRequest, UpdateRuleRequest } from '../admin/errorPassthrough'
export type { BackupAgentHealth, DataManagementConfig } from '../admin/dataManagement'
export type { TLSFingerprintProfile, CreateProfileRequest, UpdateProfileRequest } from '../admin/tlsFingerprintProfile'
export type { ContentModerationConfig, ContentModerationLog, ModerationMode } from '../admin/riskControl'
export type {
  CockpitSummaryResponse,
  CockpitAccountSummary,
  CockpitPlatformSummary,
  CockpitQuotaWarning,
  CockpitQuotaScope,
  CockpitQuotaSeverity,
} from '../admin/cockpit'

export default operatorAPI
