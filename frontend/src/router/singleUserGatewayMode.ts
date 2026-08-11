export const SINGLE_USER_PRIVATE_CONTROL_PLANE_HOSTS = [
  'localhost',
  '127.0.0.1',
  '::1',
  '100.97.17.1',
] as const

export const SINGLE_USER_GATEWAY_RESTRICTED_PREFIXES: readonly string[] = []

export function isSingleUserGatewayRestrictedPath(_path: string): boolean {
  // The private router is an allowlist, so shared navigation has no second
  // customer-route denylist to ship to the browser.
  return false
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

export function singleUserGatewayRedirectPath(_isAdmin: boolean): string {
  return '/admin/dashboard'
}
