import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import { getSummary } from '@/api/admin/cockpit'

describe('admin cockpit API', () => {
  beforeEach(() => get.mockReset())

  it('loads the bounded admin summary endpoint', async () => {
    const response = {
      generated_at: '2026-08-05T12:00:00Z',
      accounts: { total: 0, active: 0, inactive: 0, error: 0, dispatch_eligible: 0, quota_warning_total: 0 },
      platforms: [],
      quota_warnings: [],
    }
    get.mockResolvedValue({ data: response })

    await expect(getSummary()).resolves.toEqual(response)
    expect(get).toHaveBeenCalledWith('/admin/cockpit-summary')
  })
})
