import { describe, expect, it } from 'vitest'
import {
  SINGLE_USER_GATEWAY_RESTRICTED_PREFIXES,
  isSingleUserGatewayRestrictedPath,
  isSingleUserPrivateControlPlaneBrowser,
  isSingleUserPrivateControlPlaneHost,
} from '../singleUserGatewayMode'

describe('single-user gateway mode route restrictions', () => {
  it('uses the private route allowlist instead of shipping a customer-route denylist', () => {
    expect(SINGLE_USER_GATEWAY_RESTRICTED_PREFIXES).toEqual([])
    expect(isSingleUserGatewayRestrictedPath('/admin/users')).toBe(false)
    expect(isSingleUserGatewayRestrictedPath('/payment/qrcode')).toBe(false)
  })

  it('cannot be switched back to the SaaS product from injected browser settings', () => {
    window.__APP_CONFIG__ = { single_user_private_control_plane: true } as any
    expect(isSingleUserPrivateControlPlaneBrowser()).toBe(true)

    window.__APP_CONFIG__ = { single_user_private_control_plane: false } as any
    expect(isSingleUserPrivateControlPlaneBrowser()).toBe(true)
  })

  it('fails closed when injected settings are absent or stale', () => {
    delete window.__APP_CONFIG__
    expect(isSingleUserPrivateControlPlaneBrowser()).toBe(true)

    window.__APP_CONFIG__ = {} as any
    expect(isSingleUserPrivateControlPlaneBrowser()).toBe(true)
  })

  it('keeps the legacy host classifier informational only', () => {
    for (const host of ['localhost', '127.0.0.1', '::1', '100.97.17.1']) {
      expect(isSingleUserPrivateControlPlaneHost(host)).toBe(true)
    }

    expect(isSingleUserPrivateControlPlaneHost('sub2api.research.for-immortal.cn')).toBe(false)
  })

  it('keeps gateway/account/key/control-plane routes available', () => {
    for (const path of [
      '/admin/dashboard',
      '/admin/accounts',
      '/admin/settings',
      '/admin/usage',
      '/admin/proxies',
      '/keys',
      '/usage',
      '/profile',
    ]) {
      expect(isSingleUserGatewayRestrictedPath(path)).toBe(false)
    }
  })
})
