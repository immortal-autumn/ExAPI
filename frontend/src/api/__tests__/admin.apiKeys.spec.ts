import { beforeEach, describe, expect, it, vi } from 'vitest'

const { put } = vi.hoisted(() => ({ put: vi.fn() }))

vi.mock('@/api/client', () => ({
  apiClient: { put },
}))

import { updateApiKeyGroup } from '@/api/admin/apiKeys'

describe('admin API key group API', () => {
  beforeEach(() => {
    put.mockReset()
    put.mockResolvedValue({ data: { api_key: { id: 42, group_id: null } } })
  })

  it('uses the explicit zero sentinel to unbind a key from its group', async () => {
    await updateApiKeyGroup(42, null)

    expect(put).toHaveBeenCalledWith('/admin/api-keys/42', { group_id: 0 })
  })

  it('keeps positive group IDs as admin group bindings', async () => {
    await updateApiKeyGroup(42, 7)

    expect(put).toHaveBeenCalledWith('/admin/api-keys/42', { group_id: 7 })
  })
})
