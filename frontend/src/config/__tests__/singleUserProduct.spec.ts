import { describe, expect, it } from 'vitest'
import {
  SINGLE_USER_ADMIN_ROUTES,
  SINGLE_USER_SETTINGS_TABS,
  isSingleUserLegacyPath,
} from '../singleUserProduct'

describe('single-user product surface', () => {
  it('contains only private gateway operator routes', () => {
    expect(SINGLE_USER_ADMIN_ROUTES).toEqual([
      '/admin/dashboard',
      '/admin/ops',
      '/admin/accounts',
      '/admin/api-keys',
      '/admin/channels/pricing',
      '/admin/channels/monitor',
      '/admin/proxies',
      '/admin/risk-control',
      '/admin/usage',
      '/admin/settings',
    ])
  })

  it('keeps only operator-oriented settings tabs', () => {
    expect(SINGLE_USER_SETTINGS_TABS).toEqual([
      'general',
      'security',
      'gateway',
      'email',
      'backup',
    ])
  })

  it.each([
    '/admin/users',
    '/admin/orders',
    '/purchase',
    '/affiliate',
    '/redeem',
  ])('marks %s as legacy', (path) => {
    expect(isSingleUserLegacyPath(path)).toBe(true)
  })

  it('does not classify active operator routes as legacy', () => {
    for (const path of SINGLE_USER_ADMIN_ROUTES) {
      expect(isSingleUserLegacyPath(path)).toBe(false)
    }
  })
})
