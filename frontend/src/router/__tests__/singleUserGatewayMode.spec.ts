import { describe, expect, it } from 'vitest'
import {
  SINGLE_USER_GATEWAY_RESTRICTED_PREFIXES,
  isSingleUserGatewayRestrictedPath,
} from '../singleUserGatewayMode'

describe('single-user gateway mode route restrictions', () => {
  it('blocks multi-user and payment management routes from the private gateway UI', () => {
    expect(SINGLE_USER_GATEWAY_RESTRICTED_PREFIXES).toEqual(expect.arrayContaining([
      '/admin/users',
      '/admin/groups',
      '/admin/subscriptions',
      '/admin/announcements',
      '/admin/orders',
      '/admin/affiliates',
      '/admin/promo-codes',
      '/subscriptions',
      '/purchase',
      '/orders',
      '/payment',
      '/affiliate',
    ]))

    for (const path of [
      '/admin/users',
      '/admin/groups/123',
      '/admin/orders/plans',
      '/purchase',
      '/payment/qrcode',
      '/affiliate',
    ]) {
      expect(isSingleUserGatewayRestrictedPath(path)).toBe(true)
    }
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
