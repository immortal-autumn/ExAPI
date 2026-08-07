import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AxiosAdapter } from 'axios'

describe('private control API client', () => {
  beforeEach(() => {
    vi.resetModules()
    localStorage.clear()
    sessionStorage.clear()
  })

  it('marks every request as an ExAPI control request without browser auth', async () => {
    const { apiClient } = await import('@/api/client')
    const adapter = vi.fn<AxiosAdapter>().mockResolvedValue({
      data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config: {},
    })
    apiClient.defaults.adapter = adapter
    localStorage.setItem('auth_token', 'must-not-be-read')
    localStorage.setItem('refresh_token', 'must-not-be-read')

    await apiClient.get('/operator/me')

    const config = adapter.mock.calls[0][0]
    expect(config.headers.get('X-ExAPI-Control-Request')).toBe('1')
    expect(config.headers.get('Authorization')).toBeUndefined()
  })

  it('surfaces peer-boundary failures without token refresh or login redirect', async () => {
    const { apiClient } = await import('@/api/client')
    const adapter = vi.fn<AxiosAdapter>().mockRejectedValue({
      response: { status: 403, data: { code: 'CONTROL_PEER_FORBIDDEN', message: 'forbidden' } },
      config: { url: '/operator/me' },
      message: 'forbidden',
    })
    apiClient.defaults.adapter = adapter
    const before = window.location.href

    await expect(apiClient.get('/operator/me')).rejects.toMatchObject({
      status: 403,
      code: 'CONTROL_PEER_FORBIDDEN',
    })
    expect(window.location.href).toBe(before)
    expect(Object.keys(sessionStorage)).toEqual([])
  })

  it('unwraps the standard API envelope', async () => {
    const { apiClient } = await import('@/api/client')
    const adapter = vi.fn<AxiosAdapter>().mockResolvedValue({
      data: { code: 0, message: 'ok', data: { id: 9 } },
      status: 200,
      statusText: 'OK',
      headers: {},
      config: {},
    })
    apiClient.defaults.adapter = adapter

    await expect(apiClient.get('/operator/me')).resolves.toMatchObject({ data: { id: 9 } })
  })
})
