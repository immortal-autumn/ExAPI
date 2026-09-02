import { describe, expect, it, vi } from 'vitest'
import {
  SINGLE_USER_ADMIN_ROUTES,
  SINGLE_USER_COMPATIBILITY_REDIRECTS,
  SINGLE_USER_PUBLIC_ROUTES,
  SINGLE_USER_RETIRED_ROUTES,
} from '@/config/singleUserProduct'

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: false,
  isAdmin: false,
  isSimpleMode: false,
}))

const appStore = vi.hoisted(() => ({
  siteName: 'ExAPI',
  backendModeEnabled: false,
  cachedPublicSettings: null as null | Record<string, unknown>,
}))

vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({ customMenuItems: [] }),
}))
vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({
    startNavigation: vi.fn(),
    endNavigation: vi.fn(),
    isLoading: { value: false },
  }),
}))
vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}))

describe('single-user private route matrix', () => {
  it('registers exactly the product contract plus compatibility and 404 routes', async () => {
    const { default: router } = await import('@/router')
    const paths = new Set(router.getRoutes().map((record) => record.path))
    const expected = new Set([
      ...SINGLE_USER_PUBLIC_ROUTES,
      ...SINGLE_USER_ADMIN_ROUTES,
      ...SINGLE_USER_COMPATIBILITY_REDIRECTS,
      ...SINGLE_USER_RETIRED_ROUTES,
      '/:pathMatch(.*)*',
    ])

    expect(paths).toEqual(expected)
  })

  it('keeps retired customer entry points on an explicit retirement page', async () => {
    const { default: router } = await import('@/router')
    const paths = new Set(router.getRoutes().map((record) => record.path))

    for (const path of [
      '/batch-image',
      '/register',
      '/email-verify',
      '/forgot-password',
      '/reset-password',
      '/profile',
      '/dashboard',
      '/usage',
      '/monitor',
    ]) {
      expect(paths.has(path), path).toBe(true)
    }

    const retiredBatchImage = router.getRoutes().find((record) => record.path === '/batch-image')
    expect(retiredBatchImage?.name).toMatch(/^RetiredCustomerFeature/)
    expect(retiredBatchImage?.meta.titleKey).toBe('retiredFeature.title')

    for (const path of [
      '/auth/callback',
      '/auth/linuxdo/callback',
      '/auth/wechat/callback',
      '/auth/wechat/payment/callback',
      '/auth/dingtalk/callback',
      '/auth/dingtalk/email-completion',
      '/auth/oidc/callback',
      '/custom/:id',
    ]) {
      expect(paths.has(path), path).toBe(false)
    }
  })

  it('registers API-key management as an admin route', async () => {
    const { default: router } = await import('@/router')
    const route = router.getRoutes().find((record) => record.name === 'AdminAPIKeys')

    expect(route?.path).toBe('/admin/api-keys')
    expect(route?.meta.requiresAuth).toBe(true)
    expect(route?.meta.requiresAdmin).toBe(true)
  })

  it('registers batch-image guidance as an admin route', async () => {
    const { default: router } = await import('@/router')
    const route = router.getRoutes().find((record) => record.name === 'AdminBatchImages')

    expect(route?.path).toBe('/admin/batch-images')
    expect(route?.meta.requiresAuth).toBe(true)
    expect(route?.meta.requiresAdmin).toBe(true)
    expect(route?.meta.titleKey).toBe('batchImage.title')
  })

  it('keeps /keys only as a compatibility redirect', async () => {
    const { default: router } = await import('@/router')
    const route = router.getRoutes().find((record) => record.path === '/keys')

    expect(route?.redirect).toBe('/admin/api-keys')
  })
})
