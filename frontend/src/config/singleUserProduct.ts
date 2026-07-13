export const SINGLE_USER_ADMIN_ROUTES = [
  '/admin/dashboard',
  '/admin/ops',
  '/admin/accounts',
  '/admin/api-keys',
  '/admin/usage',
  '/admin/settings',
] as const

export const SINGLE_USER_SETTINGS_TABS = [
  'general',
  'security',
  'gateway',
  'email',
  'backup',
] as const

export const SINGLE_USER_LEGACY_PREFIXES = [
  '/admin/users',
  '/admin/groups',
  '/admin/subscriptions',
  '/admin/redeem',
  '/admin/promo-codes',
  '/admin/announcements',
  '/admin/affiliates',
  '/admin/orders',
  '/subscriptions',
  '/purchase',
  '/orders',
  '/payment',
  '/redeem',
  '/affiliate',
  '/available-channels',
] as const

export function isSingleUserLegacyPath(path: string): boolean {
  return SINGLE_USER_LEGACY_PREFIXES.some(
    (prefix) => path === prefix || path.startsWith(`${prefix}/`),
  )
}
