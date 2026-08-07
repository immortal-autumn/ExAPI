import { apiClient } from '../client'

export type CockpitQuotaScope = 'total' | 'daily' | 'weekly'
export type CockpitQuotaSeverity = 'warning' | 'critical'

export interface CockpitAccountSummary {
  total: number
  active: number
  inactive: number
  error: number
  dispatch_eligible: number
  quota_warning_total: number
}

export interface CockpitPlatformSummary {
  platform: string
  total: number
  active: number
  error: number
  dispatch_eligible: number
}

export interface CockpitQuotaWarning {
  account_id: number
  name: string
  platform: string
  scope: CockpitQuotaScope
  used: number
  limit: number
  percent: number
  severity: CockpitQuotaSeverity
}

export interface CockpitSummaryResponse {
  generated_at: string
  accounts: CockpitAccountSummary
  platforms: CockpitPlatformSummary[]
  quota_warnings: CockpitQuotaWarning[]
}

export async function getSummary(): Promise<CockpitSummaryResponse> {
  const { data } = await apiClient.get<CockpitSummaryResponse>('/admin/cockpit-summary')
  return data
}

export default {
  getSummary,
}
