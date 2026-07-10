import { describe, expect, it } from 'vitest'
import {
  buildSingleUserCockpitSummary,
  getAccountQuotaState,
  getLocalIntegrationLinks,
} from '../singleUserCockpit'
import type { Account } from '@/types'

function account(overrides: Partial<Account>): Account {
  return {
    id: 1,
    name: 'acct',
    platform: 'openai',
    type: 'apikey',
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: false,
    created_at: '2026-07-10T00:00:00Z',
    updated_at: '2026-07-10T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides,
  }
}

describe('single user cockpit summary', () => {
  it('groups account health by platform and counts wakeup-ready accounts', () => {
    const summary = buildSingleUserCockpitSummary([
      account({ id: 1, name: 'openai-main', platform: 'openai', status: 'active', schedulable: true }),
      account({ id: 2, name: 'openai-error', platform: 'openai', status: 'error', schedulable: false }),
      account({ id: 3, name: 'gemini-main', platform: 'gemini', status: 'active', schedulable: true }),
    ])

    expect(summary.totalAccounts).toBe(3)
    expect(summary.activeAccounts).toBe(2)
    expect(summary.errorAccounts).toBe(1)
    expect(summary.wakeupReadyAccounts).toBe(2)
    expect(summary.platforms).toEqual([
      { platform: 'gemini', total: 1, active: 1, error: 0, schedulable: 1 },
      { platform: 'openai', total: 2, active: 1, error: 1, schedulable: 1 },
    ])
  })

  it('classifies quota usage using the highest configured quota window', () => {
    expect(getAccountQuotaState(account({ quota_limit: 100, quota_used: 91 }))).toMatchObject({
      label: 'Critical',
      percent: 91,
      severity: 'critical',
    })
    expect(getAccountQuotaState(account({ quota_daily_limit: 100, quota_daily_used: 70 }))).toMatchObject({
      label: 'Watch',
      percent: 70,
      severity: 'warning',
    })
    expect(getAccountQuotaState(account({ quota_weekly_limit: 100, quota_weekly_used: 10 }))).toMatchObject({
      label: 'Healthy',
      percent: 10,
      severity: 'healthy',
    })
  })

  it('builds local integration links from runtime origin plus configured public and wg URLs', () => {
    const links = getLocalIntegrationLinks({
      origin: 'http://100.97.17.1:8027',
      publicHost: 'sub2api.research.for-immortal.cn',
      wireGuardURL: 'http://100.97.17.1:8027',
    })

    expect(links.publicGatewayBaseURL).toBe('https://sub2api.research.for-immortal.cn/v1')
    expect(links.privateControlPanelURL).toBe('http://100.97.17.1:8027/')
    expect(links.localControlPanelURL).toBe('http://127.0.0.1:8027/')
  })
})
