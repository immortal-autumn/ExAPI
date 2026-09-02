export const SINGLE_USER_ADMIN_ROUTES = [
  '/admin/dashboard',
  '/admin/ops',
  '/admin/accounts',
  '/admin/batch-images',
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
] as const

export const SINGLE_USER_PUBLIC_ROUTES = [
] as const

export const SINGLE_USER_COMPATIBILITY_REDIRECTS = [
  '/',
  '/home',
  '/admin',
  '/admin/channels',
  '/keys',
] as const

/** Browser paths retained only to explain that the former customer UI is gone. */
export const SINGLE_USER_RETIRED_ROUTES = [
  '/register',
  '/email-verify',
  '/forgot-password',
  '/reset-password',
  '/batch-image',
  '/dashboard',
  '/usage',
  '/monitor',
  '/profile',
  '/subscriptions/:pathMatch(.*)*',
  '/purchase/:pathMatch(.*)*',
  '/orders/:pathMatch(.*)*',
  '/payment/:pathMatch(.*)*',
  '/redeem/:pathMatch(.*)*',
  '/affiliate/:pathMatch(.*)*',
] as const

export const SINGLE_USER_SETTINGS_TABS = [
  'general',
  'gateway',
  'email',
  'backup',
] as const

export const SINGLE_USER_LEGACY_PREFIXES = [
  '/admin/users',
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

const SINGLE_USER_POST_LOGIN_ROUTES = [
  ...SINGLE_USER_ADMIN_ROUTES,
] as readonly string[]

export function singleUserPostLoginRedirect(requested?: unknown): string {
  const fallback = '/admin/dashboard'
  if (typeof requested !== 'string' || !requested.startsWith('/') || requested.startsWith('//') || requested.includes('\\')) {
    return fallback
  }
  try {
    const parsed = new URL(requested, 'http://private.invalid')
    if (parsed.origin !== 'http://private.invalid' || !SINGLE_USER_POST_LOGIN_ROUTES.includes(parsed.pathname)) {
      return fallback
    }
    return `${parsed.pathname}${parsed.search}${parsed.hash}`
  } catch {
    return fallback
  }
}

export function filterSingleUserProductRoutes<T extends { path: string }>(routes: readonly T[]): T[] {
  return routes.filter((route) =>
    route.path === '/:pathMatch(.*)*' || isSingleUserProductRouteAllowed(route.path),
  )
}
