import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'

import { useGroupsTelemetry } from '../groupsTelemetry'

describe('useGroupsTelemetry', () => {
  it('loads and normalizes usage and capacity summaries only when their columns are visible', async () => {
    const usageVisible = ref(true)
    const capacityVisible = ref(true)
    const fetchUsageSummary = vi.fn().mockResolvedValue([
      { group_id: 7, today_cost: 1.25, total_cost: 14.5 },
    ])
    const fetchCapacitySummary = vi.fn().mockResolvedValue([
      {
        group_id: 7,
        concurrency_used: 2,
        concurrency_max: 10,
        sessions_used: 1,
        sessions_max: 4,
        rpm_used: 3,
        rpm_max: 20,
      },
    ])

    const telemetry = useGroupsTelemetry({
      isUsageVisible: () => usageVisible.value,
      isCapacityVisible: () => capacityVisible.value,
      fetchUsageSummary,
      fetchCapacitySummary,
      resolveTimezone: () => 'Asia/Shanghai',
    })

    await telemetry.loadUsageSummary()
    await telemetry.loadCapacitySummary()

    expect(fetchUsageSummary).toHaveBeenCalledWith('Asia/Shanghai')
    expect(telemetry.usageMap.value.get(7)).toEqual({ today_cost: 1.25, total_cost: 14.5 })
    expect(telemetry.capacityMap.value.get(7)).toEqual({
      concurrencyUsed: 2,
      concurrencyMax: 10,
      sessionsUsed: 1,
      sessionsMax: 4,
      rpmUsed: 3,
      rpmMax: 20,
    })
    expect(telemetry.usageLoading.value).toBe(false)
  })

  it('does not issue hidden-column requests and clears usage loading state', async () => {
    const fetchUsageSummary = vi.fn()
    const fetchCapacitySummary = vi.fn()
    const telemetry = useGroupsTelemetry({
      isUsageVisible: () => false,
      isCapacityVisible: () => false,
      fetchUsageSummary,
      fetchCapacitySummary,
    })

    telemetry.usageLoading.value = true
    await telemetry.loadUsageSummary()
    await telemetry.loadCapacitySummary()

    expect(fetchUsageSummary).not.toHaveBeenCalled()
    expect(fetchCapacitySummary).not.toHaveBeenCalled()
    expect(telemetry.usageLoading.value).toBe(false)
  })

  it('keeps failures non-blocking while reporting the summary kind', async () => {
    const usageError = new Error('usage unavailable')
    const capacityError = new Error('capacity unavailable')
    const onError = vi.fn()
    const telemetry = useGroupsTelemetry({
      isUsageVisible: () => true,
      isCapacityVisible: () => true,
      fetchUsageSummary: vi.fn().mockRejectedValue(usageError),
      fetchCapacitySummary: vi.fn().mockRejectedValue(capacityError),
      onError,
    })

    await expect(telemetry.loadUsageSummary()).resolves.toBeUndefined()
    await expect(telemetry.loadCapacitySummary()).resolves.toBeUndefined()

    expect(onError).toHaveBeenNthCalledWith(1, 'usage', usageError)
    expect(onError).toHaveBeenNthCalledWith(2, 'capacity', capacityError)
    expect(telemetry.usageLoading.value).toBe(false)
  })
})
