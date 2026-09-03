import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { nextTick } from 'vue'

import type { ApiKey } from '@/types'
import KeysView from '../KeysView.vue'

const {
  listKeys,
  createKey,
  deleteKey,
  getPublicSettings,
  getDashboardApiKeysUsage,
  getAvailableGroups,
  getUserGroupRates,
  updateAdminApiKeyGroup,
  updateUserApiKey,
  showError,
  showSuccess,
  copyToClipboard,
} = vi.hoisted(() => ({
  listKeys: vi.fn(),
  createKey: vi.fn(),
  deleteKey: vi.fn(),
  getPublicSettings: vi.fn(),
  getDashboardApiKeysUsage: vi.fn(),
  getAvailableGroups: vi.fn(),
  getUserGroupRates: vi.fn(),
  updateAdminApiKeyGroup: vi.fn(),
  updateUserApiKey: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  copyToClipboard: vi.fn(),
}))

const messages: Record<string, string> = {
  'common.actions': 'Actions',
  'common.name': 'Name',
  'common.refresh': 'Refresh',
  'common.status': 'Status',
  'keys.apiKey': 'API Key',
  'keys.allGroups': 'All Groups',
  'keys.allStatus': 'All Status',
  'keys.columnSettings': 'Column Settings',
  'keys.createKey': 'Create API Key',
  'keys.copyCreatedKey': 'Copy key',
  'keys.createdKeySaved': 'I have saved the key',
  'keys.createdKeyTitle': 'Save your new API key',
  'keys.createdKeyWarning': 'Shown only once',
  'keys.secretDisplayHint': 'The full API key is shown only immediately after creation.',
  'keys.keySecretUnavailable': 'The key was created without returning its one-time secret.',
  'keys.groupRequired': 'Please select a group',
  'keys.copied': 'Copied!',
  'keys.created': 'Created',
  'keys.expiresAt': 'Expires',
  'keys.group': 'Group',
  'keys.id': 'ID',
  'keys.currentConcurrency': 'Current Concurrency',
  'keys.lastUsedAt': 'Last Used',
  'keys.lastUsedIP': 'Last Used IP',
  'keys.rateLimitColumn': 'Rate Limit',
  'keys.searchPlaceholder': 'Search name or key...',
  'keys.status.active': 'Active',
  'keys.status.expired': 'Expired',
  'keys.status.inactive': 'Inactive',
  'keys.status.quota_exhausted': 'Quota exhausted',
  'keys.usage': 'Usage',
}

vi.mock('@/api', () => ({
  keysAPI: {
    list: listKeys,
    create: createKey,
    update: updateUserApiKey,
    delete: deleteKey,
    toggleStatus: vi.fn(),
  },
  usageAPI: {
    getDashboardApiKeysUsage,
  },
  userGroupsAPI: {
    getAvailable: getAvailableGroups,
    getUserGroupRates,
  },
}))

vi.mock('@/api/admin/apiKeys', () => ({
  default: {
    updateApiKeyGroup: updateAdminApiKeyGroup,
  },
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get: getPublicSettings,
  },
}))

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: { template: '<div><slot /></div>' },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const createApiKey = (): ApiKey => ({
  id: 1,
  user_id: 1,
  key: 'sk-test-key',
  name: 'test-key',
  group_id: null,
  status: 'active',
  ip_whitelist: [],
  ip_blacklist: [],
  last_used_at: null,
  last_used_ip: null,
  quota: 0,
  quota_used: 0,
  expires_at: null,
  created_at: '2026-06-27T00:00:00Z',
  updated_at: '2026-06-27T00:00:00Z',
  current_concurrency: 3,
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  usage_5h: 0,
  usage_1d: 0,
  usage_7d: 0,
  window_5h_start: null,
  window_1d_start: null,
  window_7d_start: null,
  reset_5h_at: null,
  reset_1d_at: null,
  reset_7d_at: null,
})

const AppLayoutStub = {
  template: '<div><slot /></div>',
}

const TablePageLayoutStub = {
  template: `
    <div>
      <slot name="filters" />
      <slot name="actions" />
      <slot name="table" />
      <slot name="pagination" />
    </div>
  `,
}

const BaseDialogStub = {
  name: 'BaseDialog',
  props: ['show', 'title'],
  template: '<section v-if="show"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>',
}

const ConfirmDialogStub = {
  name: 'ConfirmDialog',
  props: ['show'],
  emits: ['confirm', 'cancel'],
  template: '<button v-if="show" data-test="confirm-dialog-confirm" @click="$emit(\'confirm\')">Confirm</button>',
}

const DataTableStub = {
  name: 'DataTable',
  props: ['columns', 'data'],
  emits: ['sort'],
  template: `
    <div>
      <div data-test="columns">{{ columns.map((col) => col.key).join(',') }}</div>
      <div data-test="columns-meta">{{ JSON.stringify(columns.map((col) => ({ key: col.key, sortable: !!col.sortable }))) }}</div>
      <button data-test="sort-current-concurrency" @click="$emit('sort', 'current_concurrency', 'asc')">
        Sort Current Concurrency
      </button>
      <div v-for="row in data" :key="row.id">
        <div
          v-if="columns.some((col) => col.key === 'id')"
          data-test="key-id"
        >
          <slot name="cell-id" :value="row.id" :row="row" />
        </div>
        <slot name="cell-name" :value="row.name" :row="row" />
        <slot name="cell-group" :row="row" />
        <div data-test="current-concurrency">
          <slot name="cell-current_concurrency" :value="row.current_concurrency" :row="row" />
        </div>
        <div
          v-if="columns.some((col) => col.key === 'last_used_ip')"
          data-test="last-used-ip"
        >
          <slot name="cell-last_used_ip" :value="row.last_used_ip" :row="row" />
        </div>
        <div data-test="row-actions"><slot name="cell-actions" :row="row" /></div>
      </div>
      <slot name="empty" />
    </div>
  `,
}

const SelectStub = {
  name: 'Select',
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"></select>',
}

const SearchInputStub = {
  name: 'SearchInput',
  props: ['modelValue'],
  emits: ['update:modelValue', 'search'],
  template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
}

const PaginationStub = {
  name: 'Pagination',
  props: ['page', 'total', 'pageSize'],
  emits: ['update:page', 'update:pageSize'],
  template: `
    <div>
      <button data-test="page-size-50" @click="$emit('update:pageSize', 50)">50</button>
    </div>
  `,
}

const IconStub = {
  props: ['name'],
  template: '<span data-test="icon">{{ name }}</span>',
}

const mountView = async () => {
  const wrapper = mount(KeysView, {
    global: {
      plugins: [createPinia()],
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: PaginationStub,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: ConfirmDialogStub,
        EmptyState: true,
        Select: SelectStub,
        SearchInput: SearchInputStub,
        Icon: IconStub,
        UseKeyModal: true,
        EndpointPopover: true,
        GroupBadge: true,
        GroupOptionItem: true,
        Teleport: true,
      },
    },
  })
  await flushPromises()
  await nextTick()
  return wrapper
}

const visibleColumnKeys = (wrapper: VueWrapper) =>
  wrapper.get('[data-test="columns"]').text().split(',').filter(Boolean)

const visibleColumnMeta = (wrapper: VueWrapper): Array<{ key: string; sortable: boolean }> =>
  JSON.parse(wrapper.get('[data-test="columns-meta"]').text())

const getButtonByText = (wrapper: VueWrapper, text: string) => {
  const button = wrapper.findAll('button').find((item) => item.text().includes(text))
  if (!button) {
    throw new Error(`Button not found: ${text}`)
  }
  return button
}

describe('user KeysView column settings', () => {
  beforeEach(() => {
    localStorage.clear()

    listKeys.mockReset()
    createKey.mockReset()
    deleteKey.mockReset()
    getPublicSettings.mockReset()
    getDashboardApiKeysUsage.mockReset()
    getAvailableGroups.mockReset()
    getUserGroupRates.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    copyToClipboard.mockReset()

    listKeys.mockResolvedValue({
      items: [{ ...createApiKey(), key: '', key_prefix: 'sk-test-' }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    createKey.mockResolvedValue({ ...createApiKey(), key: 'sk-created-secret' })
    deleteKey.mockResolvedValue({ message: 'deleted' })
    getPublicSettings.mockResolvedValue({ data: {} })
    getDashboardApiKeysUsage.mockResolvedValue({ stats: {} })
    getAvailableGroups.mockResolvedValue([])
    getUserGroupRates.mockResolvedValue({})
    updateAdminApiKeyGroup.mockReset()
    updateUserApiKey.mockReset()
    updateAdminApiKeyGroup.mockResolvedValue({ api_key: createApiKey() })
    updateUserApiKey.mockResolvedValue(createApiKey())
  })

  it('does not offer use or import actions for keys loaded from storage', async () => {
    const wrapper = await mountView()

    expect(getPublicSettings).toHaveBeenCalledWith('/settings/public')
    expect(wrapper.text()).not.toContain('keys.useKey')
    expect(wrapper.text()).not.toContain('keys.importToCcSwitch')
  })

  it('shows a generated key exactly once and clears it after acknowledgement', async () => {
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'OpenAI', platform: 'openai' }])
    const wrapper = await mountView()

    await getButtonByText(wrapper, 'Create API Key').trigger('click')
    await wrapper.get('input[data-testid="key-form-name"]').setValue('operator-key')
    const formGroupSelect = wrapper.findAllComponents({ name: 'Select' })[2]
    await formGroupSelect.vm.$emit('update:modelValue', 42)
    await wrapper.get('form#key-form').trigger('submit')
    await flushPromises()

    expect(createKey).toHaveBeenCalled()
    expect(wrapper.get('[data-testid="created-api-key"]').attributes('value')).toBe(
      'sk-created-secret'
    )

    await getButtonByText(wrapper, 'Copy key').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith('sk-created-secret', 'Copied!')

    await getButtonByText(wrapper, 'I have saved the key').trigger('click')
    expect(wrapper.find('[data-testid="created-api-key"]').exists()).toBe(false)
    expect(wrapper.html()).not.toContain('sk-created-secret')
  })

  it('shows an inline group error instead of silently skipping creation', async () => {
    const wrapper = await mountView()

    await getButtonByText(wrapper, 'Create API Key').trigger('click')
    await wrapper.get('form#key-form').trigger('submit')

    expect(wrapper.get('[data-testid="key-form-group-error"]').text()).toBe('Please select a group')
    expect(showError).toHaveBeenCalledWith('Please select a group')
    expect(createKey).not.toHaveBeenCalled()
  })

  it('warns when a secret-redacted create response cannot show the key', async () => {
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'OpenAI', platform: 'openai' }])
    createKey.mockResolvedValueOnce({ ...createApiKey(), key: '' })
    const wrapper = await mountView()

    await getButtonByText(wrapper, 'Create API Key').trigger('click')
    await wrapper.get('input[data-testid="key-form-name"]').setValue('operator-key')
    const formGroupSelect = wrapper.findAllComponents({ name: 'Select' })[2]
    await formGroupSelect.vm.$emit('update:modelValue', 42)
    await wrapper.get('form#key-form').trigger('submit')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('The key was created without returning its one-time secret.')
    expect(wrapper.find('[data-testid="created-api-key"]').exists()).toBe(false)
  })

  it('prevents duplicate revocation requests while deletion is pending', async () => {
    let resolveDelete: (value: { message: string }) => void = () => undefined
    deleteKey.mockImplementation(
      () => new Promise<{ message: string }>((resolve) => { resolveDelete = resolve })
    )
    const wrapper = await mountView()

    await getButtonByText(wrapper, 'common.delete').trigger('click')
    const confirm = wrapper.get('[data-test="confirm-dialog-confirm"]')
    await confirm.trigger('click')
    await confirm.trigger('click')

    expect(deleteKey).toHaveBeenCalledTimes(1)
    resolveDelete({ message: 'deleted' })
    await flushPromises()
  })

  it('uses the admin zero sentinel when unbinding a group in operator mode', async () => {
    listKeys.mockResolvedValue({
      items: [{ ...createApiKey(), group_id: 42 }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'OpenAI', platform: 'openai' }])
    const wrapper = await mount(KeysView, {
      props: { operatorMode: true },
      global: {
        plugins: [createPinia()],
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          Pagination: PaginationStub,
          BaseDialog: BaseDialogStub,
          ConfirmDialog: ConfirmDialogStub,
          EmptyState: true,
          Select: SelectStub,
          SearchInput: SearchInputStub,
          Icon: IconStub,
          UseKeyModal: true,
          EndpointPopover: true,
          GroupBadge: true,
          GroupOptionItem: true,
          Teleport: true,
        },
      },
    })
    await flushPromises()

    await wrapper.get('[data-testid="key-group-trigger"]').trigger('click')
    await nextTick()
    await wrapper.get('[data-testid="key-group-unbind"]').trigger('click')
    await flushPromises()

    expect(updateAdminApiKeyGroup).toHaveBeenCalledWith(1, null)
    expect(updateUserApiKey).not.toHaveBeenCalled()
  })

  it('blocks a second group update for the same key while the first is pending', async () => {
    let resolveGroupUpdate: (() => void) | undefined
    updateAdminApiKeyGroup.mockImplementation(
      () => new Promise((resolve) => {
        resolveGroupUpdate = () => resolve({ api_key: createApiKey() })
      })
    )
    listKeys.mockResolvedValue({
      items: [{ ...createApiKey(), group_id: 42 }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'OpenAI', platform: 'openai' }])

    const wrapper = await mount(KeysView, {
      props: { operatorMode: true },
      global: {
        plugins: [createPinia()],
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          Pagination: PaginationStub,
          BaseDialog: BaseDialogStub,
          ConfirmDialog: ConfirmDialogStub,
          EmptyState: true,
          Select: SelectStub,
          SearchInput: SearchInputStub,
          Icon: IconStub,
          UseKeyModal: true,
          EndpointPopover: true,
          GroupBadge: true,
          GroupOptionItem: true,
          Teleport: true,
        },
      },
    })
    await flushPromises()

    const trigger = wrapper.get('[data-testid="key-group-trigger"]')
    await trigger.trigger('click')
    await nextTick()
    await wrapper.get('[data-testid="key-group-unbind"]').trigger('click')
    await nextTick()

    expect(updateAdminApiKeyGroup).toHaveBeenCalledTimes(1)
    expect(trigger.isDisabled()).toBe(true)

    await trigger.trigger('click')
    expect(updateAdminApiKeyGroup).toHaveBeenCalledTimes(1)

    resolveGroupUpdate?.()
    await flushPromises()
    expect(trigger.isDisabled()).toBe(false)
  })

  it('uses the default API key columns with low-frequency columns hidden', async () => {
    const wrapper = await mountView()

    expect(visibleColumnKeys(wrapper)).toEqual([
      'name',
      'key',
      'group',
      'current_concurrency',
      'usage',
      'expires_at',
      'status',
      'created_at',
      'actions',
    ])
    expect(visibleColumnKeys(wrapper)).not.toContain('rate_limit')
    expect(visibleColumnKeys(wrapper)).not.toContain('last_used_at')
    expect(visibleColumnKeys(wrapper)).not.toContain('last_used_ip')
    expect(visibleColumnKeys(wrapper)).not.toContain('id')
  })

  it('shows a hidden column when toggled and persists the preference', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'Rate Limit').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('rate_limit')
    expect(localStorage.getItem('api-key-hidden-columns')).toBe(
      JSON.stringify(['id', 'last_used_at', 'last_used_ip'])
    )
    expect(localStorage.getItem('api-key-column-settings-version')).toBe('3')
  })

  it('shows the API key ID column when toggled', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'ID').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('id')
    expect(wrapper.get('[data-test="key-id"]').text()).toBe('#1')
    expect(visibleColumnMeta(wrapper).find((column) => column.key === 'id')?.sortable).toBe(true)
  })

  it('shows the last used IP column when toggled', async () => {
    listKeys.mockResolvedValueOnce({
      items: [{ ...createApiKey(), last_used_ip: '203.0.113.10' }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'Last Used IP').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('last_used_ip')
    expect(wrapper.get('[data-test="last-used-ip"]').text()).toBe('203.0.113.10')
  })

  it('restores column preferences from localStorage on mount', async () => {
    localStorage.setItem('api-key-hidden-columns', JSON.stringify(['group', 'created_at']))
    localStorage.setItem('api-key-column-settings-version', '1')

    const wrapper = await mountView()

    expect(visibleColumnKeys(wrapper)).toEqual([
      'name',
      'key',
      'current_concurrency',
      'usage',
      'rate_limit',
      'expires_at',
      'status',
      'last_used_at',
      'actions',
    ])
    expect(localStorage.getItem('api-key-hidden-columns')).toBe(
      JSON.stringify(['group', 'created_at', 'last_used_ip', 'id'])
    )
    expect(localStorage.getItem('api-key-column-settings-version')).toBe('3')
  })

  it('does not include always-visible columns in the toggleable menu', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await nextTick()

    const columnMenuText = wrapper.text()
    expect(columnMenuText).toContain('API Key')
    expect(columnMenuText).toContain('ID')
    expect(columnMenuText).toContain('Current Concurrency')
    expect(columnMenuText).toContain('Rate Limit')
    expect(columnMenuText).toContain('Last Used IP')
    expect(columnMenuText).not.toContain('Name')
    expect(columnMenuText).not.toContain('Actions')
  })

  it('renders the current concurrency value', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-test="current-concurrency"]').text()).toBe('3')
  })

  it('marks current concurrency as sortable', async () => {
    const wrapper = await mountView()

    const currentConcurrencyColumn = visibleColumnMeta(wrapper).find(
      (column) => column.key === 'current_concurrency'
    )
    expect(currentConcurrencyColumn?.sortable).toBe(true)
  })

  it('keeps filters and selected page size when sorting by current concurrency', async () => {
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'OpenAI' }])
    const wrapper = await mountView()

    await wrapper.get('[data-test="page-size-50"]').trigger('click')
    await flushPromises()

    await wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('update:modelValue', 'target')
    await wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('search')
    await flushPromises()

    const selects = wrapper.findAllComponents({ name: 'Select' })
    await selects[0].vm.$emit('update:modelValue', 42)
    await flushPromises()
    await selects[1].vm.$emit('update:modelValue', 'active')
    await flushPromises()

    listKeys.mockClear()

    await wrapper.get('[data-test="sort-current-concurrency"]').trigger('click')
    await flushPromises()

    expect(listKeys).toHaveBeenLastCalledWith(
      1,
      50,
      {
        search: 'target',
        status: 'active',
        group_id: 42,
        sort_by: 'current_concurrency',
        sort_order: 'asc',
      },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
  })
})
