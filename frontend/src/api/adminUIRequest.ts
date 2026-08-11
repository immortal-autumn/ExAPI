export const ADMIN_UI_REQUEST_HEADER = 'X-Admin-UI-Request'
export const USER_UI_REQUEST_HEADER = 'X-User-UI-Request'

function isAdminPath(path: string): boolean {
  return (
    path === '/admin' ||
    path.startsWith('/admin/') ||
    path === '/api/v1/admin' ||
    path.startsWith('/api/v1/admin/')
  )
}

function requestPath(rawURL: string): string {
  const value = rawURL.trim()
  if (!value) return ''
  try {
    const origin = typeof window !== 'undefined' ? window.location.origin : 'http://localhost'
    return new URL(value, origin).pathname
  } catch {
    return value.split(/[?#]/, 1)[0]
  }
}

/** Normalize Axios relative paths and absolute API paths to a comparable form. */
function normalizeAPIPath(path: string): string {
  const raw = requestPath(path)
  if (!raw) return ''
  if (raw === '/api/v1' || raw.startsWith('/api/v1/')) {
    return raw.slice('/api/v1'.length) || '/'
  }
  if (raw.startsWith('/')) {
    return raw
  }
  return `/${raw}`
}

/**
 * Private operator APIs that may emit Server-Timing on the control listener.
 */
export function isUserTimingAPIPath(requestURL: string): boolean {
  const path = normalizeAPIPath(requestURL)
  if (!path) return false

  if (path === '/operator/me') return true
  if (path === '/keys' || path.startsWith('/keys/')) return true
  if (path === '/groups/available' || path === '/groups/rates') return true
  if (path === '/usage' || path.startsWith('/usage/')) return true
  if (path === '/channel-monitors' || path.startsWith('/channel-monitors/')) return true
  if (path === '/operator/batch-images' || path.startsWith('/operator/batch-images/')) return true
  return false
}

export function shouldMarkAdminUIRequest(requestURL: string, pagePath?: string): boolean {
  const currentPath =
    pagePath ?? (typeof window !== 'undefined' ? window.location.pathname : '')
  return isAdminPath(requestPath(requestURL)) || isAdminPath(currentPath)
}

export function shouldMarkUserUIRequest(requestURL: string): boolean {
  return isUserTimingAPIPath(requestURL)
}
