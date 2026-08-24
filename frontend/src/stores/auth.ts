/**
 * Private operator identity store.
 *
 * ExAPI is a single-operator, peer-authenticated control plane.  The browser
 * never receives a bearer credential: the direct control listener authenticates
 * the WireGuard peer and `/operator/me` returns the current operator identity.
 * Keep the historical `useAuthStore` export for component compatibility while
 * deliberately exposing no persisted/session token state.
 */

import { defineStore } from 'pinia'
import { computed, readonly, ref } from 'vue'
import { apiClient } from '@/api/client'
import type { User } from '@/types'

export type OperatorAccessState = 'unknown' | 'loading' | 'ready' | 'denied' | 'unavailable'

export interface OperatorIdentity {
  id: number
  username?: string
  email?: string
  role: 'admin'
  status?: string
  concurrency?: number
  run_mode?: 'standard' | 'simple'
}

function asOperatorIdentity(value: unknown): OperatorIdentity {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('Invalid operator identity response')
  }

  const data = value as Record<string, unknown>
  if (typeof data.id !== 'number' || !Number.isInteger(data.id) || data.id <= 0) {
    throw new Error('Invalid operator identity response')
  }
  if (data.role !== 'admin') {
    throw new Error('Invalid operator identity response')
  }
  if (data.status !== undefined && data.status !== 'active') {
    throw new Error('Invalid operator identity response')
  }
  if (data.run_mode !== undefined && data.run_mode !== 'standard' && data.run_mode !== 'simple') {
    throw new Error('Invalid operator identity response')
  }

  return {
    id: data.id as number,
    username: typeof data.username === 'string' ? data.username : undefined,
    email: typeof data.email === 'string' ? data.email : undefined,
    role: data.role,
    status: typeof data.status === 'string' ? data.status : undefined,
    concurrency: typeof data.concurrency === 'number' ? data.concurrency : undefined,
    run_mode: data.run_mode as OperatorIdentity['run_mode'],
  }
}

function accessStateForError(error: unknown): OperatorAccessState {
  const status = Number((error as { status?: number })?.status)
  return status === 403 || status === 404 ? 'denied' : 'unavailable'
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  // Compatibility-only value. It is permanently null and is never persisted.
  const token = readonly(ref<string | null>(null))
  const runMode = ref<'standard' | 'simple'>('simple')
  const accessState = ref<OperatorAccessState>('unknown')
  const accessError = ref<unknown>(null)
  let bootstrapPromise: Promise<User> | null = null
  let refreshIntervalId: ReturnType<typeof setInterval> | null = null

  const isAuthenticated = computed(() => user.value !== null && accessState.value === 'ready')
  const isAdmin = computed(() => isAuthenticated.value && user.value?.role === 'admin')
  const isSimpleMode = computed(() => true)
  const hasPendingAuthSession = computed(() => false)

  async function refreshUser(): Promise<User> {
    if (bootstrapPromise) return bootstrapPromise
    accessState.value = 'loading'
    accessError.value = null
    bootstrapPromise = apiClient.get<OperatorIdentity>('/operator/me')
      .then(({ data }) => {
        const identity = asOperatorIdentity(data)
        const next = identity as unknown as User
        user.value = next
        runMode.value = identity.run_mode ?? 'simple'
        accessState.value = 'ready'
        return next
      })
      .catch((error) => {
        user.value = null
        runMode.value = 'simple'
        accessError.value = error
        accessState.value = accessStateForError(error)
        throw error
      })
      .finally(() => {
        bootstrapPromise = null
      })
    return bootstrapPromise
  }

  /** Bootstrap the peer-authenticated operator on app startup/navigation. */
  async function checkAuth(): Promise<void> {
    try {
      await refreshUser()
      startAutoRefresh()
    } catch {
      // The UI renders an explicit unavailable/denied state; never redirect to
      // a customer login screen because no browser login exists in private mode.
    }
  }

  function startAutoRefresh(): void {
    stopAutoRefresh()
    refreshIntervalId = setInterval(() => {
      void refreshUser().catch(() => undefined)
    }, 60_000)
  }

  function stopAutoRefresh(): void {
    if (refreshIntervalId) {
      clearInterval(refreshIntervalId)
      refreshIntervalId = null
    }
  }

  async function logout(): Promise<void> {
    stopAutoRefresh()
    user.value = null
    runMode.value = 'simple'
    accessState.value = 'unknown'
    accessError.value = null
  }

  // Legacy auth actions intentionally fail closed.  Keeping their signatures
  // avoids breaking dormant customer-auth components while ensuring they cannot
  // issue password, OAuth, passkey, TOTP, JWT, or token-storage requests.
  async function unavailableAuthAction(..._args: unknown[]): Promise<never> {
    throw new Error('Browser authentication is disabled; connect through the ExAPI control peer.')
  }

  function clearPendingAuthSession(): void {
    // Pending customer-auth sessions are not supported in private mode.
  }

  function setPendingAuthSession(_session: unknown): void {
    // Intentionally a no-op for compatibility with dormant callback code.
  }

  return {
    user,
    token,
    runMode: readonly(runMode),
    accessState: readonly(accessState),
    accessError: readonly(accessError),
    pendingAuthSession: readonly(ref(null)),
    isAuthenticated,
    isAdmin,
    isSimpleMode,
    hasPendingAuthSession,
    checkAuth,
    refreshUser,
    logout,
    login: unavailableAuthAction,
    localAdminLogin: unavailableAuthAction,
    login2FA: unavailableAuthAction,
    register: unavailableAuthAction,
    setToken: unavailableAuthAction,
    clearPendingAuthSession,
    setPendingAuthSession,
  }
})
