import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory } from 'vue-router'

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAdmin: true,
}))

const appStore = vi.hoisted(() => ({
  siteName: 'ExAPI',
  publicSettingsLoaded: false,
  cachedPublicSettings: null as Record<string, unknown> | null,
  fetchPublicSettings: vi.fn(),
  showWarning: vi.fn(),
}))

const navigation = vi.hoisted(() => ({
  startNavigation: vi.fn(),
  endNavigation: vi.fn(),
}))

const prefetch = vi.hoisted(() => ({
  triggerPrefetch: vi.fn(),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({ customMenuItems: [] }),
}))

vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => navigation,
}))

vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => prefetch,
}))

vi.mock('@/router/title', () => ({
  resolveRouteDocumentTitle: vi.fn(() => 'ExAPI'),
}))

import { createAppRouter, handleRouterError, isChunkLoadError } from '@/router'

function newRouter() {
  return createAppRouter(createMemoryHistory())
}

describe('real private router navigation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    authStore.checkAuth.mockResolvedValue(undefined)
    authStore.isAdmin = true
    appStore.publicSettingsLoaded = false
    appStore.cachedPublicSettings = null
    appStore.fetchPublicSettings.mockResolvedValue(null)
    appStore.showWarning.mockReset()
    sessionStorage.clear()
    document.title = ''
    window.scrollTo = vi.fn()
  })

  it('bootstraps the peer operator and keeps a denied peer on the requested private route', async () => {
    authStore.isAdmin = false

    const router = newRouter()
    await router.push('/admin/dashboard')

    expect(router.currentRoute.value.fullPath).toBe('/admin/dashboard')
    expect(authStore.checkAuth).toHaveBeenCalledOnce()
    expect(navigation.startNavigation).toHaveBeenCalledOnce()
    expect(navigation.endNavigation).toHaveBeenCalledOnce()
    expect(prefetch.triggerPrefetch).toHaveBeenCalledWith(router.currentRoute.value)
  })

  it('follows the concrete route table redirects through Vue Router', async () => {
    const router = newRouter()

    await router.push('/')
    expect(router.currentRoute.value.fullPath).toBe('/admin/dashboard')

    await router.push('/home')
    expect(router.currentRoute.value.fullPath).toBe('/admin/dashboard')

    await router.push('/keys')
    expect(router.currentRoute.value.fullPath).toBe('/admin/api-keys')
  })

  it('loads risk settings before entering a guarded route and redirects only when explicitly disabled', async () => {
    appStore.fetchPublicSettings.mockImplementation(async () => {
      appStore.publicSettingsLoaded = true
      appStore.cachedPublicSettings = { risk_control_enabled: false }
      return appStore.cachedPublicSettings
    })

    const router = newRouter()
    await router.push('/admin/risk-control')

    expect(appStore.fetchPublicSettings).toHaveBeenCalledOnce()
    expect(router.currentRoute.value.fullPath).toBe('/admin/settings')
  })

  it('allows the risk route when the settings request fails and state remains unknown', async () => {
    appStore.fetchPublicSettings.mockRejectedValue(new Error('temporary settings outage'))

    const router = newRouter()
    await router.push('/admin/risk-control')

    expect(appStore.fetchPublicSettings).toHaveBeenCalledOnce()
    expect(router.currentRoute.value.fullPath).toBe('/admin/risk-control')
  })

  it('does not re-bootstrap the operator or refetch settings on subsequent navigations', async () => {
    const router = newRouter()

    await router.push('/admin/dashboard')
    await router.push('/admin/accounts')
    await router.push('/admin/groups')

    expect(authStore.checkAuth).toHaveBeenCalledOnce()
    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
    expect(navigation.startNavigation).toHaveBeenCalledTimes(3)
    expect(navigation.endNavigation).toHaveBeenCalledTimes(3)
  })

  it('recognizes lazy-loaded chunk failures without treating ordinary errors as chunks', () => {
    expect(isChunkLoadError(new Error('Failed to fetch dynamically imported module'))).toBe(true)
    expect(isChunkLoadError(Object.assign(new Error('upstream failed'), { name: 'ChunkLoadError' }))).toBe(true)
    expect(isChunkLoadError(new Error('request failed'))).toBe(false)
    expect(isChunkLoadError(null)).toBe(false)
  })

  it('reports a chunk failure without reloading the page', () => {
    handleRouterError(new Error('Failed to fetch dynamically imported module'))

    expect(appStore.showWarning).toHaveBeenCalledWith(
      'This page was updated. Refresh when it is safe to continue.',
      30000
    )
    expect(sessionStorage.getItem('chunk_reload_attempted')).toBeNull()
  })
})
