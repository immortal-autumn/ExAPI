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
  router.onError((error) => {
    console.error('Router error:', error)

    const isChunkLoadError =
      error.message?.includes('Failed to fetch dynamically imported module') ||
      error.message?.includes('Loading chunk') ||
      error.message?.includes('Loading CSS chunk') ||
      error.name === 'ChunkLoadError'

    if (isChunkLoadError) {
      const reloadKey = 'chunk_reload_attempted'
      const lastReload = sessionStorage.getItem(reloadKey)
      const now = Date.now()

      if (!lastReload || now - parseInt(lastReload) > 10000) {
        sessionStorage.setItem(reloadKey, now.toString())
        console.warn('Chunk load error detected, reloading page to fetch latest version...')
        window.location.reload()
      } else {
        console.error('Chunk load error persists after reload. Please clear browser cache.')
      }
    }
  })

  return router
}

const router = createAppRouter()

export default router
