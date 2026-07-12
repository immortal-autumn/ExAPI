export const SINGLE_USER_GATEWAY_RESTRICTED_PREFIXES = [
  // Multi-user administration
  '/admin/users',
  '/admin/groups',
  '/admin/subscriptions',
  '/admin/redeem',
  '/admin/promo-codes',
  '/admin/announcements',
  '/admin/affiliates',

  // Payment/order administration
  '/admin/orders',

  // User-facing SaaS/payment surfaces that do not belong in a private gateway
  '/subscriptions',
  '/purchase',
  '/orders',
  '/payment',
  '/redeem',
  '/affiliate',
  '/available-channels',
] as const

export function isSingleUserGatewayRestrictedPath(path: string): boolean {
  return SINGLE_USER_GATEWAY_RESTRICTED_PREFIXES.some((prefix) => (
    path === prefix || path.startsWith(`${prefix}/`)
  ))
}

export function singleUserGatewayRedirectPath(isAdmin: boolean): string {
  return isAdmin ? '/admin/dashboard' : '/dashboard'
}
