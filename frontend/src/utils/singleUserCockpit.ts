import type { Account, AccountPlatform } from '@/types'

export type QuotaSeverity = 'healthy' | 'warning' | 'critical' | 'none'

export interface AccountQuotaState {
  label: 'Healthy' | 'Watch' | 'Critical' | 'No quota limit'
  severity: QuotaSeverity
  percent: number
  used: number
  limit: number
  scope: 'total' | 'daily' | 'weekly' | 'none'
}

export interface PlatformAccountSummary {
  platform: AccountPlatform
  total: number
  active: number
  error: number
  schedulable: number
}

export interface SingleUserCockpitSummary {
  totalAccounts: number
  activeAccounts: number
  errorAccounts: number
  inactiveAccounts: number
  schedulableAccounts: number
  quotaWatchAccounts: Array<Account & { quotaState: AccountQuotaState }>
  platforms: PlatformAccountSummary[]
}

export interface LocalIntegrationInput {
  origin?: string
  apiBaseUrl?: string
  wireGuardURL?: string
}

export interface LocalIntegrationLinks {
  publicGatewayBaseURL: string
  privateControlPanelURL: string
  localControlPanelURL: string
}

function finiteNumber(value: unknown): number {
  const n = Number(value)
  return Number.isFinite(n) ? n : 0
}

function bestQuotaWindow(account: Account): Pick<AccountQuotaState, 'used' | 'limit' | 'scope'> {
  const windows: Array<Pick<AccountQuotaState, 'used' | 'limit' | 'scope'>> = [
    {
      scope: 'total',
      used: finiteNumber(account.quota_used),
      limit: finiteNumber(account.quota_limit),
    },
    {
      scope: 'daily',
      used: finiteNumber(account.quota_daily_used),
      limit: finiteNumber(account.quota_daily_limit),
    },
    {
      scope: 'weekly',
      used: finiteNumber(account.quota_weekly_used),
      limit: finiteNumber(account.quota_weekly_limit),
    },
  ]

  return windows
    .filter((window) => window.limit > 0)
    .sort((a, b) => (b.used / b.limit) - (a.used / a.limit))[0] ?? {
      scope: 'none',
      used: 0,
      limit: 0,
    }
}

export function getAccountQuotaState(account: Account): AccountQuotaState {
  const quota = bestQuotaWindow(account)
  if (quota.scope === 'none' || quota.limit <= 0) {
    return {
      label: 'No quota limit',
      severity: 'none',
      percent: 0,
      used: 0,
      limit: 0,
      scope: 'none',
    }
  }

  const percent = Math.max(0, Math.round((quota.used / quota.limit) * 100))
  if (percent >= 90) {
    return { ...quota, percent, label: 'Critical', severity: 'critical' }
  }
  if (percent >= 70) {
    return { ...quota, percent, label: 'Watch', severity: 'warning' }
  }
  return { ...quota, percent, label: 'Healthy', severity: 'healthy' }
}

export function buildSingleUserCockpitSummary(accounts: Account[]): SingleUserCockpitSummary {
  const platformMap = new Map<AccountPlatform, PlatformAccountSummary>()
  const quotaWatchAccounts: Array<Account & { quotaState: AccountQuotaState }> = []

  let activeAccounts = 0
  let errorAccounts = 0
  let inactiveAccounts = 0
  let schedulableAccounts = 0

  for (const account of accounts) {
    if (!platformMap.has(account.platform)) {
      platformMap.set(account.platform, {
        platform: account.platform,
        total: 0,
        active: 0,
        error: 0,
        schedulable: 0,
      })
    }
    const platform = platformMap.get(account.platform)!
    platform.total += 1

    if (account.status === 'active') {
      activeAccounts += 1
      platform.active += 1
    } else if (account.status === 'error') {
      errorAccounts += 1
      platform.error += 1
    } else {
      inactiveAccounts += 1
    }

    if (isAccountSchedulable(account)) {
      schedulableAccounts += 1
      platform.schedulable += 1
    }

    const quotaState = getAccountQuotaState(account)
    if (quotaState.severity === 'warning' || quotaState.severity === 'critical') {
      quotaWatchAccounts.push({ ...account, quotaState })
    }
  }

  quotaWatchAccounts.sort((a, b) => b.quotaState.percent - a.quotaState.percent)

  return {
    totalAccounts: accounts.length,
    activeAccounts,
    errorAccounts,
    inactiveAccounts,
    schedulableAccounts,
    quotaWatchAccounts: quotaWatchAccounts.slice(0, 6),
    platforms: Array.from(platformMap.values()).sort((a, b) => a.platform.localeCompare(b.platform)),
  }
}

function isAccountSchedulable(account: Account): boolean {
  if (account.status !== 'active' || !account.schedulable) return false
  const now = Date.now()
  if (account.expires_at != null && Number(account.expires_at) * 1000 <= now) return false
  for (const until of [account.rate_limit_reset_at, account.overload_until, account.temp_unschedulable_until]) {
    if (until && Date.parse(until) > now) return false
  }
  const quota = getAccountQuotaState(account)
  return quota.scope === 'none' || quota.percent < 100
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, '')
}

export function resolvePublicAPIBaseURL(configured: unknown, origin?: string): string {
  const runtimeOrigin = trimTrailingSlash(String(origin || '').trim())
  const fallback = runtimeOrigin ? `${runtimeOrigin}/v1` : '/v1'
  const candidate = String(configured || '').trim()
  if (!candidate || candidate.startsWith('//') || candidate.includes('\\')) return fallback

  try {
    if (candidate.startsWith('/')) {
      if (!runtimeOrigin) return trimTrailingSlash(candidate)
      const resolved = new URL(candidate, `${runtimeOrigin}/`)
      if (resolved.origin !== new URL(runtimeOrigin).origin) return fallback
      return trimTrailingSlash(resolved.toString())
    }

    const resolved = new URL(candidate)
    if (resolved.protocol !== 'http:' && resolved.protocol !== 'https:') return fallback
    if (resolved.username || resolved.password) return fallback
    return trimTrailingSlash(resolved.toString())
  } catch {
    return fallback
  }
}

export function getLocalIntegrationLinks(input: LocalIntegrationInput): LocalIntegrationLinks {
  const origin = trimTrailingSlash(input.origin?.trim() || '')
  const privateControlPanelURL = trimTrailingSlash(input.wireGuardURL?.trim() || origin || 'http://100.97.17.1:8027') + '/'

  return {
    publicGatewayBaseURL: resolvePublicAPIBaseURL(input.apiBaseUrl, origin),
    privateControlPanelURL,
    localControlPanelURL: 'http://127.0.0.1:8027/',
  }
}
