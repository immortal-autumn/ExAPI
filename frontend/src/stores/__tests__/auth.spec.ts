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
      data: {
        id: 7,
        username: 'operator',
        email: 'operator@example.test',
        role: 'admin',
        status: 'active',
        run_mode: 'standard',
      },
    })
    const store = useAuthStore()

    await store.checkAuth()

    expect(get).toHaveBeenCalledWith('/operator/me')
    expect(store.user?.id).toBe(7)
    expect(store.isAuthenticated).toBe(true)
    expect(store.isAdmin).toBe(true)
    expect(store.token).toBeNull()
    expect(store.runMode).toBe('standard')
    expect(Object.keys(localStorage)).toEqual([])
    expect(Object.keys(sessionStorage)).toEqual([])
  })

  it.each([
    ['non-object payload', null],
    ['missing identity fields', {}],
    ['zero id', { id: 0, role: 'admin' }],
    ['fractional id', { id: 1.5, role: 'admin' }],
    ['string id', { id: '7', role: 'admin' }],
    ['missing role', { id: 7 }],
    ['non-admin role', { id: 7, role: 'user' }],
    ['disabled status', { id: 7, role: 'admin', status: 'disabled' }],
    ['invalid run mode', { id: 7, role: 'admin', run_mode: 'unknown' }],
  ])('fails closed for %s', async (_name, data) => {
    get
      .mockResolvedValueOnce({ data: { id: 7, role: 'admin', status: 'active' } })
      .mockResolvedValueOnce({ data })
    const store = useAuthStore()
    await store.refreshUser()
    expect(store.isAuthenticated).toBe(true)

    await expect(store.refreshUser()).rejects.toThrow('Invalid operator identity response')

    expect(store.user).toBeNull()
    expect(store.runMode).toBe('simple')
    expect(store.accessState).toBe('unavailable')
    expect(store.isAuthenticated).toBe(false)
    expect(store.isAdmin).toBe(false)
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
