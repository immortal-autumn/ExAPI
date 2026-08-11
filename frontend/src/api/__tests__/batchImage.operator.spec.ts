import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  buildApiUrl: (path: string) => `/api/v1${path}`,
}))

import { listBatchImageModels, submitBatchImageJob } from '@/api/batchImage'

describe('operator batch-image API', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    fetchMock.mockReset()
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ data: [] }),
    })
    vi.stubGlobal('fetch', fetchMock)
  })

  it('addresses an operator-owned key by opaque id without a bearer credential', async () => {
    await listBatchImageModels(42)

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    const headers = new Headers(init.headers)
    expect(url).toBe('/api/v1/operator/batch-images/models?api_key_id=42')
    expect(headers.get('X-ExAPI-Control-Request')).toBe('1')
    expect(headers.has('Authorization')).toBe(false)
  })

  it('does not serialize a raw gateway key in submit requests', async () => {
    await submitBatchImageJob(42, {
      model: 'gemini-2.5-flash-image',
      items: [{ custom_id: 'one', prompt: 'draw a tree' }],
    }, 'idempotency-1')

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/operator/batch-images?api_key_id=42')
    expect(init.body).toBe(JSON.stringify({
      model: 'gemini-2.5-flash-image',
      items: [{ custom_id: 'one', prompt: 'draw a tree' }],
    }))
    expect(new Headers(init.headers).has('Authorization')).toBe(false)
  })
})
