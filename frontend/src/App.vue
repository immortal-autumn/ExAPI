<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import { computed, onMounted, watch } from 'vue'
import Toast from '@/components/common/Toast.vue'
import NavigationProgress from '@/components/common/NavigationProgress.vue'
import { resolveRouteDocumentTitle } from '@/router/title'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { updateFavicon } from '@/utils/branding'

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()
const operatorAccessMessage = computed(() => {
  switch (authStore.accessState) {
    case 'denied':
      return 'This control-plane request was denied. Connect from an allowed WireGuard operator peer.'
    case 'unavailable':
      return 'The ExAPI control plane is unavailable. Verify the control listener and WireGuard connection.'
    case 'loading':
      return 'Connecting to the ExAPI control plane…'
    default:
      return ''
  }
})

function updateDocumentTitle() {
  const customMenuItems = [
    ...(appStore.cachedPublicSettings?.custom_menu_items ?? []),
    ...(authStore.isAdmin ? adminSettingsStore.customMenuItems : []),
  ]
  document.title = resolveRouteDocumentTitle(route, appStore.siteName, customMenuItems)
}

// Watch for site settings changes and update favicon/title
watch(
  () => appStore.siteLogo,
  (newLogo) => {
    if (newLogo) {
      updateFavicon(newLogo)
    }
  },
  { immediate: true }
)

watch(
  [
    () => route.fullPath,
    () => route.meta.title,
    () => route.meta.titleKey,
    () => appStore.siteName,
    () => appStore.cachedPublicSettings?.custom_menu_items,
    () => authStore.isAdmin,
    () => adminSettingsStore.customMenuItems,
  ],
  updateDocumentTitle,
  { deep: true }
)

onMounted(async () => {
  // Load public settings into appStore (will be cached for other components)
  await appStore.fetchPublicSettings()

  // Re-resolve document title now that site settings are available
  updateDocumentTitle()
})
</script>

<template>
  <NavigationProgress />
  <RouterView v-if="authStore.accessState === 'ready'" />
  <Toast />
  <section
    v-if="route.meta.requiresAuth !== false && authStore.accessState !== 'ready'"
    class="private-access-state"
    role="status"
    aria-live="polite"
  >
    <h1>ExAPI private control plane</h1>
    <p>{{ operatorAccessMessage }}</p>
    <button type="button" @click="authStore.checkAuth()">Retry connection</button>
  </section>
</template>
