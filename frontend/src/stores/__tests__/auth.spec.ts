import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import { useAuthStore } from '@/stores/auth'

describe('private operator identity store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    get.mockReset()
    localStorage.clear()
    sessionStorage.clear()
  })

  it('bootstraps the singleton operator without browser credentials', async () => {
    get.mockResolvedValue({
      data: { id: 7, username: 'operator', email: 'operator@example.test', role: 'admin' },
    })
    const store = useAuthStore()

    await store.checkAuth()

    expect(get).toHaveBeenCalledWith('/operator/me')
    expect(store.user?.id).toBe(7)
    expect(store.isAuthenticated).toBe(true)
    expect(store.isAdmin).toBe(true)
    expect(store.token).toBeNull()
    expect(Object.keys(localStorage)).toEqual([])
    expect(Object.keys(sessionStorage)).toEqual([])
  })

  it.each([
    [403, 'denied'],
    [404, 'denied'],
    [503, 'unavailable'],
  ] as const)('surfaces HTTP %s as %s without redirecting', async (status, expected) => {
    get.mockRejectedValue({ status })
    const store = useAuthStore()
    const originalLocation = window.location.href

    await store.checkAuth()

    expect(store.accessState).toBe(expected)
    expect(store.isAuthenticated).toBe(false)
    expect(window.location.href).toBe(originalLocation)
  })

  it('legacy browser-auth actions fail closed without issuing a request', async () => {
    const store = useAuthStore()

    await expect(store.login({ email: 'ignored', password: 'ignored' })).rejects.toThrow(
      'Browser authentication is disabled',
    )
    expect(get).not.toHaveBeenCalled()
  })
})
