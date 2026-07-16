export const SINGLE_USER_ADMIN_ROUTES = [
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
] as const

export const SINGLE_USER_PUBLIC_ROUTES = [
  '/setup',
  '/login',
  '/key-usage',
] as const

export const SINGLE_USER_COMPATIBILITY_REDIRECTS = [
  '/',
  '/home',
  '/admin',
  '/admin/channels',
  '/keys',
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

export function isSingleUserProductRouteAllowed(path: string): boolean {
  return (
    (SINGLE_USER_PUBLIC_ROUTES as readonly string[]).includes(path)
    || (SINGLE_USER_ADMIN_ROUTES as readonly string[]).includes(path)
    || (SINGLE_USER_COMPATIBILITY_REDIRECTS as readonly string[]).includes(path)
  )
}

export function filterSingleUserProductRoutes<T extends { path: string }>(routes: readonly T[]): T[] {
  return routes.filter((route) =>
    route.path === '/:pathMatch(.*)*' || isSingleUserProductRouteAllowed(route.path),
  )
}
