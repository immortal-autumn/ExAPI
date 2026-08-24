import { ref, type Ref } from 'vue'

export interface GroupUsageSummaryRecord {
  group_id: number
  today_cost: number
  total_cost: number
}

export interface GroupUsageSummary {
  today_cost: number
  total_cost: number
}

export interface GroupCapacitySummaryRecord {
  group_id: number
  concurrency_used: number
  concurrency_max: number
  sessions_used: number
  sessions_max: number
  rpm_used: number
  rpm_max: number
}

export interface GroupCapacitySummary {
  concurrencyUsed: number
  concurrencyMax: number
  sessionsUsed: number
  sessionsMax: number
  rpmUsed: number
  rpmMax: number
}

export interface GroupsTelemetryOptions {
  isUsageVisible: () => boolean
  isCapacityVisible: () => boolean
  fetchUsageSummary: (timezone: string) => Promise<readonly GroupUsageSummaryRecord[]>
  fetchCapacitySummary: () => Promise<readonly GroupCapacitySummaryRecord[]>
  resolveTimezone?: () => string
  onError?: (kind: 'usage' | 'capacity', error: unknown) => void
}

export interface GroupsTelemetry {
  usageMap: Ref<Map<number, GroupUsageSummary>>
  usageLoading: Ref<boolean>
  capacityMap: Ref<Map<number, GroupCapacitySummary>>
  loadUsageSummary: () => Promise<void>
  loadCapacitySummary: () => Promise<void>
}

function browserTimezone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone
}

export function useGroupsTelemetry(options: GroupsTelemetryOptions): GroupsTelemetry {
  const usageMap = ref(new Map<number, GroupUsageSummary>())
  const usageLoading = ref(false)
  const capacityMap = ref(new Map<number, GroupCapacitySummary>())

  const loadUsageSummary = async (): Promise<void> => {
    if (!options.isUsageVisible()) {
      usageLoading.value = false
      return
    }

    usageLoading.value = true
    try {
      const data = await options.fetchUsageSummary(
        (options.resolveTimezone ?? browserTimezone)(),
      )
      const map = new Map<number, GroupUsageSummary>()
      for (const item of data) {
        map.set(item.group_id, {
          today_cost: item.today_cost,
          total_cost: item.total_cost,
        })
      }
      usageMap.value = map
    } catch (error) {
      options.onError?.('usage', error)
    } finally {
      usageLoading.value = false
    }
  }

  const loadCapacitySummary = async (): Promise<void> => {
    if (!options.isCapacityVisible()) return

    try {
      const data = await options.fetchCapacitySummary()
      const map = new Map<number, GroupCapacitySummary>()
      for (const item of data) {
        map.set(item.group_id, {
          concurrencyUsed: item.concurrency_used,
          concurrencyMax: item.concurrency_max,
          sessionsUsed: item.sessions_used,
          sessionsMax: item.sessions_max,
          rpmUsed: item.rpm_used,
          rpmMax: item.rpm_max,
        })
      }
      capacityMap.value = map
    } catch (error) {
      options.onError?.('capacity', error)
    }
  }

  return {
    usageMap,
    usageLoading,
    capacityMap,
    loadUsageSummary,
    loadCapacitySummary,
  }
}
