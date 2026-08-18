export interface AccountIDRow {
  id: number
}

interface AccountListPage<T extends AccountIDRow> {
  items: T[]
  total: number
  pages?: number
}

type AccountPageFetcher<T extends AccountIDRow> = (
  page: number,
  pageSize: number,
  filters: Record<string, unknown>
) => Promise<AccountListPage<T>>

const SELECT_ALL_PAGE_SIZE = 1000

export async function fetchAllAccountRows<T extends AccountIDRow>(
  fetchPage: AccountPageFetcher<T>,
  filters: Record<string, unknown>
): Promise<T[]> {
  const requestFilters = {
    ...filters,
    lite: '1',
    include_scheduler_score: '0'
  }
  const firstPage = await fetchPage(1, SELECT_ALL_PAGE_SIZE, requestFilters)
  const pageCount = Math.max(
    firstPage.pages ?? 0,
    Math.ceil(firstPage.total / SELECT_ALL_PAGE_SIZE)
  )
  const rows = [...firstPage.items]

  for (let page = 2; page <= pageCount; page++) {
    const result = await fetchPage(page, SELECT_ALL_PAGE_SIZE, requestFilters)
    rows.push(...result.items)
  }

  const uniqueRows = new Map(rows.map(row => [row.id, row]))
  if (uniqueRows.size !== firstPage.total) {
    throw new Error('账号列表结果不完整')
  }
  return Array.from(uniqueRows.values())
}

export async function fetchAllAccountIds(
  fetchPage: AccountPageFetcher<AccountIDRow>,
  filters: Record<string, unknown>
): Promise<number[]> {
  const rows = await fetchAllAccountRows(fetchPage, filters)
  return rows.map(row => row.id)
}
