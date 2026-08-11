import { describe, expect, it } from 'vitest'
import {
  SINGLE_USER_ADMIN_ROUTES,
  SINGLE_USER_SETTINGS_TABS,
  isSingleUserLegacyPath,
  singleUserPostLoginRedirect,
} from '../singleUserProduct'

describe('single-user product surface', () => {
  it('contains only private gateway operator routes', () => {
    expect(SINGLE_USER_ADMIN_ROUTES).toEqual([
      '/admin/dashboard',
      '/admin/ops',
      '/admin/accounts',
      '/admin/groups',
      '/admin/api-keys',
      '/admin/channels/pricing',
      '/admin/channels/monitor',
      '/admin/proxies',
      '/admin/risk-control',
      '/admin/usage',
      '/admin/settings',
      '/admin/audit-logs',
      '/admin/prompt-audit',
    ])
  })

  it('keeps only operator-oriented settings tabs', () => {
    expect(SINGLE_USER_SETTINGS_TABS).toEqual([
      'general',
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

  it.each([
    [undefined, '/admin/dashboard'],
    [null, '/admin/dashboard'],
    [['/admin/settings', '//evil.example'], '/admin/dashboard'],
    ['/dashboard', '/admin/dashboard'],
    ['/unknown', '/admin/dashboard'],
    ['/register', '/admin/dashboard'],
    ['/login', '/admin/dashboard'],
    ['/setup', '/admin/dashboard'],
    ['/', '/admin/dashboard'],
    ['/admin/users', '/admin/dashboard'],
    ['/admin/settings?tab=backup', '/admin/settings?tab=backup'],
    ['/admin/accounts?status=active#table', '/admin/accounts?status=active#table'],
  ])('maps private login redirect %s to %s', (requested, expected) => {
    expect(singleUserPostLoginRedirect(requested as never)).toBe(expected)
  })

  it.each([
    'https://evil.example/admin/dashboard',
    '//evil.example/admin/dashboard',
    '/admin/settings%3Ftab=backup',
  ])('rejects unsafe private login redirect %s', (requested) => {
    expect(singleUserPostLoginRedirect(requested)).toBe('/admin/dashboard')
  })
})
