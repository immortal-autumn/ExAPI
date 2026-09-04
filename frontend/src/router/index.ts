/**
 * Vue Router configuration for ExAPI frontend
 * Defines all application routes with lazy loading and navigation guards
 */

import { createRouter, createWebHistory } from 'vue-router'
import type { RouterHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { useNavigationLoadingState } from '@/composables/useNavigationLoading'
import { useRoutePrefetch } from '@/composables/useRoutePrefetch'
import { resolveRouteDocumentTitle } from './title'
import { privateRoutes } from './privateRoutes'
import i18n from '@/i18n'

const productRoutes = privateRoutes

/**
 * Create a router instance and install the application navigation lifecycle.
 *
 * Keeping construction injectable lets the real router be exercised with a
 * memory history in tests. Production still uses the browser history below.
 */
export function createAppRouter(history: RouterHistory = createWebHistory(import.meta.env.BASE_URL)) {
  const router = createRouter({
    history,
    routes: productRoutes,
    scrollBehavior(_to, _from, savedPosition) {
      // Scroll to saved position when using browser back/forward
      if (savedPosition) {
        return savedPosition
      }
      // Scroll to top for new routes
      return { top: 0 }
    }
  })

  /**
   * Navigation guard: peer-authenticated operator bootstrap and feature gates.
   */
  let authInitialized = false
  const navigationLoading = useNavigationLoadingState()
  let routePrefetch: ReturnType<typeof useRoutePrefetch> | null = null
  router.beforeEach(async (to, _from, next) => {
    navigationLoading.startNavigation()

    const authStore = useAuthStore()

    // Peer authentication is established by the control listener. Bootstrap
    // identity before entering a private route, but leave the route mounted so
    // the app can render a useful denied/unavailable state (never `/login`).
    if (!authInitialized) {
      await authStore.checkAuth()
      authInitialized = true
    }

    const appStore = useAppStore()
    const adminSettingsStore = useAdminSettingsStore()
    const customMenuItems = [
      ...(appStore.cachedPublicSettings?.custom_menu_items ?? []),
      ...(authStore.isAdmin ? adminSettingsStore.customMenuItems : []),
    ]
    document.title = resolveRouteDocumentTitle(to, appStore.siteName, customMenuItems)

    // Risk-control settings may not be loaded before the first navigation.
    if (to.meta.requiresRiskControl && !appStore.publicSettingsLoaded) {
      try {
        await appStore.fetchPublicSettings()
      } catch (error) {
        console.warn('Failed to load public settings in route guard', error)
      }
    }

    // Only an explicit value from successfully loaded settings can disable a route.
    // A transient settings failure is unknown state, not a confirmed feature toggle.
    if (
      to.meta.requiresRiskControl &&
      appStore.publicSettingsLoaded &&
      appStore.cachedPublicSettings?.risk_control_enabled === false
    ) {
      next('/admin/settings')
      return
    }

    next()
  })

  router.afterEach((to) => {
    navigationLoading.endNavigation()
    if (!routePrefetch) {
      routePrefetch = useRoutePrefetch(router)
    }
    routePrefetch.triggerPrefetch(to)
  })

  /**
   * Navigation guard: dynamic import error recovery after a deployment update.
   */
  router.onError(handleRouterError)

  return router
}

/**
 * Identify failures caused by a stale or unavailable lazy-loaded asset.
 * Keep this separate from the router factory so the recovery policy can be
 * tested without relying on browser history or a mounted application.
 */
export function isChunkLoadError(error: unknown): boolean {
  if (!(error instanceof Error) && (typeof error !== 'object' || error === null)) {
    return false
  }

  const candidate = error as { message?: unknown; name?: unknown }
  const message = typeof candidate.message === 'string' ? candidate.message : ''
  const name = typeof candidate.name === 'string' ? candidate.name : ''

  return (
    message.includes('Failed to fetch dynamically imported module') ||
    message.includes('Loading chunk') ||
    message.includes('Loading CSS chunk') ||
    name === 'ChunkLoadError'
  )
}

/**
 * Report router failures without forcing a destructive page reload. A reload
 * can repeat indefinitely when a proxy/CDN serves stale HTML for a missing
 * asset; the operator should choose when it is safe to refresh instead.
 */
export function handleRouterError(error: unknown): void {
  console.error('Router error:', error)

  if (!isChunkLoadError(error)) {
    return
  }

  const appStore = useAppStore()
  const key = 'common.privateControl.chunkLoadFailed'
  const fallbackMessage = 'This page was updated. Refresh when it is safe to continue.'
  const message = i18n.global.te(key) ? i18n.global.t(key) : fallbackMessage

  // Keep the notice visible long enough for an operator to finish an in-flight
  // action, while still allowing normal toast dismissal.
  appStore.showWarning(message, 30000)
}

const router = createAppRouter()

export default router
