import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getUpstreamBillingProbeSettings,
  getAllProxies,
  getAllGroups,
  batchDelete,
  showError,
  showSuccess,
  confirmAction
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getUpstreamBillingProbeSettings: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  batchDelete: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  confirmAction: vi.fn()
}))

vi.mock('@/api/operator', () => ({
  operatorAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      getUpstreamBillingProbeSettings,
      batchDelete,
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      bulkUpdate: vi.fn()
    },
    proxies: {
      getAll: getAllProxies
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    token: 'test-token'
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const makeAccounts = (count: number) => Array.from({ length: count }, (_, index) => ({
  id: index + 1,
  name: `account-${index + 1}`,
  platform: 'grok',
  type: 'oauth',
  status: 'active',
  schedulable: true,
  created_at: '2026-07-23T00:00:00Z',
  updated_at: '2026-07-23T00:00:00Z'
}))

const AccountBulkActionsBarStub = {
  props: ['selectedIds', 'totalResults', 'selectingAll', 'allResultsSelected'],
  emits: ['select-all-results', 'select-page', 'clear', 'delete', 'edit-selected'],
  template: `
    <div>
      <span data-test="selected-count">{{ selectedIds.length }}</span>
      <span data-test="total-results">{{ totalResults }}</span>
      <span data-test="all-results-selected">{{ String(allResultsSelected) }}</span>
      <button data-test="select-page" @click="$emit('select-page')">select page</button>
      <button data-test="select-all-results" @click="$emit('select-all-results')">select all</button>
      <button data-test="clear" @click="$emit('clear')">clear</button>
      <button data-test="delete" @click="$emit('delete')">delete</button>
      <button data-test="edit-selected" @click="$emit('edit-selected')">edit selected</button>
    </div>
  `
}

const BulkEditAccountModalStub = {
  props: ['show', 'target'],
  template: `
    <div
      data-test="bulk-edit-modal"
      :data-platforms="target?.selectedPlatforms?.join(',') ?? ''"
      :data-types="target?.selectedTypes?.join(',') ?? ''"
      :data-account-ids="target?.accountIds?.join(',') ?? ''"
    />
  `
}

const AccountTableFiltersStub = {
  emits: ['change'],
  template: '<button data-test="change-filter" @click="$emit(\'change\')">change filter</button>'
}

const mountedWrappers: VueWrapper[] = []

const mountView = () => {
  const wrapper = mount(AccountsView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      TablePageLayout: {
        template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
      },
      DataTable: { props: ['data'], template: '<div data-test="data-table"></div>' },
      Pagination: true,
      ConfirmDialog: true,
      AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
      AccountTableFilters: AccountTableFiltersStub,
      AccountBulkActionsBar: AccountBulkActionsBarStub,
      AccountActionMenu: true,
      ImportDataModal: true,
      ReAuthAccountModal: true,
      AccountTestModal: true,
      AccountStatsModal: true,
      ScheduledTestsPanel: true,
      SyncFromCrsModal: true,
      TempUnschedStatusModal: true,
      ErrorPassthroughRulesModal: true,
      TLSFingerprintProfilesModal: true,
      CreateAccountModal: true,
      EditAccountModal: true,
      BulkEditAccountModal: BulkEditAccountModalStub,
      PlatformTypeBadge: true,
      AccountCapacityCell: true,
      AccountStatusIndicator: true,
      AccountTodayStatsCell: true,
      AccountGroupsCell: true,
      AccountUsageCell: true,
      Icon: true
    }
  }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('admin AccountsView select all filtered results', () => {
  beforeEach(() => {
    localStorage.clear()
    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getUpstreamBillingProbeSettings.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()
    batchDelete.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    confirmAction.mockReset().mockReturnValue(true)
    vi.stubGlobal('confirm', confirmAction)

    listWithEtag.mockResolvedValue({
      notModified: true,
      etag: null,
      data: null
    })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getUpstreamBillingProbeSettings.mockResolvedValue({ enabled: true, interval_minutes: 30 })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  afterEach(() => {
    for (const wrapper of mountedWrappers.splice(0)) wrapper.unmount()
    vi.unstubAllGlobals()
  })

  it('selects all matching IDs in one commit and clears the selection when filters change', async () => {
    const allAccounts = makeAccounts(45)
    listAccounts.mockImplementation(async (_page: number, pageSize: number) => {
      if (pageSize === 1000) {
        return {
          items: allAccounts,
          total: 45,
          page: 1,
          page_size: 1000,
          pages: 1
        }
      }
      return {
        items: allAccounts.slice(0, 20),
        total: 45,
        page: 1,
        page_size: 20,
        pages: 3
      }
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="select-all-results"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('45')
    expect(wrapper.get('[data-test="total-results"]').text()).toBe('45')
    expect(wrapper.get('[data-test="all-results-selected"]').text()).toBe('true')
    expect(listAccounts).toHaveBeenCalledWith(1, 1000, expect.objectContaining({
      lite: '1',
      include_scheduler_score: '0'
    }))

    await wrapper.get('[data-test="change-filter"]').trigger('click')

    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('0')
    expect(wrapper.get('[data-test="all-results-selected"]').text()).toBe('false')
  })

  it('keeps the original page selection when loading all results fails', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    const currentPage = makeAccounts(20)
    listAccounts.mockImplementation(async (_page: number, pageSize: number) => {
      if (pageSize === 1000) {
        throw new Error('load all failed')
      }
      return {
        items: currentPage,
        total: 45,
        page: 1,
        page_size: 20,
        pages: 3
      }
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="select-page"]').trigger('click')
    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('20')

    await wrapper.get('[data-test="select-all-results"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('20')
    expect(wrapper.get('[data-test="all-results-selected"]').text()).toBe('false')
    expect(showError).toHaveBeenCalledWith('admin.accounts.bulkActions.selectAllFailed')
    consoleError.mockRestore()
  })

  it('deletes all selected results through the bounded batch endpoint', async () => {
    const accounts = makeAccounts(3)
    listAccounts.mockResolvedValue({
      items: accounts,
      total: 3,
      page: 1,
      page_size: 20,
      pages: 1
    })
    batchDelete.mockResolvedValue({ success: 3, failed: 0, success_ids: [1, 2, 3], failed_ids: [] })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="select-page"]').trigger('click')
    await wrapper.get('[data-test="delete"]').trigger('click')
    await flushPromises()

    expect(confirmAction).toHaveBeenCalledWith('admin.accounts.bulkActions.confirmDelete')
    expect(batchDelete).toHaveBeenCalledWith([1, 2, 3])
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.bulkActions.deleteSuccess')
    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('0')
  })

  it('keeps only failed account IDs selected after a partial batch delete', async () => {
    const accounts = makeAccounts(3)
    listAccounts.mockResolvedValue({
      items: accounts,
      total: 3,
      page: 1,
      page_size: 20,
      pages: 1
    })
    batchDelete.mockResolvedValue({ success: 2, failed: 1, success_ids: [1, 3], failed_ids: [2] })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="select-page"]').trigger('click')
    await wrapper.get('[data-test="delete"]').trigger('click')
    await flushPromises()

    expect(batchDelete).toHaveBeenCalledWith([1, 2, 3])
    expect(showError).toHaveBeenCalledWith('admin.accounts.bulkActions.partialSuccess')
    expect(showSuccess).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('1')
  })

  it('resolves heterogeneous metadata across the complete selected result set', async () => {
    const allAccounts = makeAccounts(45)
    allAccounts[44] = {
      ...allAccounts[44],
      platform: 'openai',
      type: 'apikey'
    }
    listAccounts.mockImplementation(async (_page: number, pageSize: number) => ({
      items: pageSize === 1000 ? allAccounts : allAccounts.slice(0, 20),
      total: 45,
      page: 1,
      page_size: pageSize,
      pages: pageSize === 1000 ? 1 : 3
    }))

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="select-all-results"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="edit-selected"]').trigger('click')
    await flushPromises()

    const modal = wrapper.get('[data-test="bulk-edit-modal"]')
    expect(modal.attributes('data-platforms')).toBe('grok,openai')
    expect(modal.attributes('data-types')).toBe('oauth,apikey')
    expect(modal.attributes('data-account-ids')?.split(',')).toHaveLength(45)
  })

  it('chunks large deletes and retains only failed or unresolved IDs after a request failure', async () => {
    const accounts = makeAccounts(120)
    listAccounts.mockImplementation(async (_page: number, pageSize: number) => ({
      items: pageSize === 1000 ? accounts : accounts.slice(0, 20),
      total: 120,
      page: 1,
      page_size: pageSize,
      pages: pageSize === 1000 ? 1 : 6
    }))
    batchDelete
      .mockResolvedValueOnce({
        total: 50,
        success: 50,
        failed: 0,
        success_ids: accounts.slice(0, 50).map(account => account.id),
        failed_ids: []
      })
      .mockRejectedValueOnce(new Error('request timed out'))

    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="select-all-results"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="delete"]').trigger('click')
    await flushPromises()

    expect(batchDelete).toHaveBeenCalledTimes(2)
    expect(batchDelete.mock.calls[0]?.[0]).toEqual(accounts.slice(0, 50).map(account => account.id))
    expect(batchDelete.mock.calls[1]?.[0]).toEqual(accounts.slice(50, 100).map(account => account.id))
    expect(showError).toHaveBeenCalledWith('admin.accounts.bulkActions.partialSuccess')
    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('70')
    expect(wrapper.get('[data-test="all-results-selected"]').text()).toBe('false')
    consoleError.mockRestore()
  })
})
