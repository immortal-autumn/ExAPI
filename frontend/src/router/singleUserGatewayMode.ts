import {
  SINGLE_USER_LEGACY_PREFIXES,
  isSingleUserLegacyPath,
} from '@/config/singleUserProduct'

export const SINGLE_USER_PRIVATE_CONTROL_PLANE_HOSTS = [
  'localhost',
  '127.0.0.1',
  '::1',
  '100.97.17.1',
] as const

export const SINGLE_USER_GATEWAY_RESTRICTED_PREFIXES = SINGLE_USER_LEGACY_PREFIXES

export function isSingleUserGatewayRestrictedPath(path: string): boolean {
  return isSingleUserLegacyPath(path)
}

export function isSingleUserPrivateControlPlaneHost(hostname: string): boolean {
  const normalized = hostname.trim().toLowerCase()
  return SINGLE_USER_PRIVATE_CONTROL_PLANE_HOSTS.some((host) => normalized === host)
}

export function isSingleUserPrivateControlPlaneBrowser(): boolean {
  // ExAPI has one product mode. There is no browser flag that can re-enable
  // customer/SaaS routes after the private cutover.
  return true
}

export function singleUserGatewayRedirectPath(isAdmin: boolean): string {
  return isAdmin ? '/admin/dashboard' : '/dashboard'
}
