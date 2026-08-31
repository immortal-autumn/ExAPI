import { defineComponent } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { AdminGroup } from '@/types'
import GroupsView from '../GroupsView.vue'

const {
  listGroups,
  getModelsListCandidates,
  getUsageSummary,
  getCapacitySummary,
  getLiveCapability,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  listGroups: vi.fn(),
  getModelsListCandidates: vi.fn(),
  getUsageSummary: vi.fn(),
  getCapacitySummary: vi.fn(),
  getLiveCapability: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/operator', () => ({
  operatorAPI: {
    groups: {
      list: listGroups,
      getModelsListCandidates,
      getUsageSummary,
      getCapacitySummary,
      getLiveCapability,
      getAll: vi.fn().mockResolvedValue([]),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      duplicate: vi.fn(),
      updateSortOrder: vi.fn(),
    },
    accounts: {
      list: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 }),
      getById: vi.fn(),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({
    isCurrentStep: vi.fn(() => false),
    nextStep: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const group: AdminGroup = {
  id: 42,
  name: 'Primary',
  description: null,
  platform: 'openai',
  rate_multiplier: 1,
  rpm_limit: 0,
  is_exclusive: false,
  status: 'active',
  subscription_type: 'standard',
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  allow_image_generation: false,
  allow_batch_image_generation: false,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  batch_image_discount_multiplier: 0.5,
  batch_image_hold_multiplier: 0.6,
  image_price_1k: null,
  image_price_2k: null,
  image_price_4k: null,
  video_rate_independent: false,
  video_rate_multiplier: 1,
  video_price_480p: null,
  video_price_720p: null,
  video_price_1080p: null,
  web_search_price_per_call: null,
  peak_rate_enabled: false,
  peak_start: '',
  peak_end: '',
  peak_rate_multiplier: 1,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  allow_messages_dispatch: false,
  default_mapped_model: '',
  messages_dispatch_model_config: undefined,
  require_oauth_only: false,
  require_privacy_set: false,
  created_at: '2026-08-31T00:00:00Z',
  updated_at: '2026-08-31T00:00:00Z',
  model_routing: null,
  model_routing_enabled: false,
  mcp_xml_inject: true,
  supported_model_scopes: [],
  account_count: 1,
  active_account_count: 1,
  rate_limited_account_count: 0,
  models_list_config: undefined,
  sort_order: 10,
}

const SelectStub = defineComponent({
  inheritAttrs: false,
  props: ['modelValue', 'options'],
  emits: ['update:modelValue', 'change'],
  template: `
    <select
      v-bind="$attrs"
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value); $emit('change')"
    >
      <option v-for="option in options" :key="String(option.value)" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  `,
})

const DataTableStub = defineComponent({
  props: ['data'],
  template: '<div><div v-for="row in data" :key="row.id"><slot name="cell-actions" :row="row" /></div></div>',
})

const mountedWrappers: VueWrapper[] = []

const mountView = async () => {
  const wrapper = mount(GroupsView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        TablePageLayout: {
          template: '<section><slot name="filters" /><slot name="table" /><slot name="pagination" /></section>',
        },
        DataTable: DataTableStub,
        Pagination: true,
        BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
        ConfirmDialog: true,
        EmptyState: true,
        Select: SelectStub,
        PlatformIcon: true,
        Icon: true,
        GroupCapacityBadge: true,
        GroupRateMultipliersModal: true,
        GroupRPMOverridesModal: true,
        VueDraggable: { template: '<div><slot /></div>' },
      },
    },
  })
  mountedWrappers.push(wrapper)
  await flushPromises()
  return wrapper
}

const openCreateModelsList = async (wrapper: VueWrapper) => {
  await wrapper.get('[data-tour="groups-create-btn"]').trigger('click')
  await wrapper.get('[data-testid="create-models-list-toggle"]').trigger('click')
}

const deferred = <T>() => {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

describe('GroupsView model-list candidate loading', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.spyOn(console, 'error').mockImplementation(() => {})
    for (const mock of [
      listGroups,
      getModelsListCandidates,
      getUsageSummary,
      getCapacitySummary,
      getLiveCapability,
      showError,
      showSuccess,
    ]) mock.mockReset()

    listGroups.mockResolvedValue({ items: [group], total: 1, page: 1, page_size: 20, pages: 1 })
    getModelsListCandidates.mockResolvedValue([])
    getUsageSummary.mockResolvedValue([])
    getCapacitySummary.mockResolvedValue([])
    getLiveCapability.mockResolvedValue({ supported: false })
  })

  afterEach(async () => {
    await flushPromises()
    for (const wrapper of mountedWrappers.splice(0)) wrapper.unmount()
    await flushPromises()
    vi.restoreAllMocks()
  })

  it('announces loading and then renders successful candidates', async () => {
    const wrapper = await mountView()
    const request = deferred<string[]>()
    getModelsListCandidates.mockImplementationOnce(() => request.promise)

    await openCreateModelsList(wrapper)

    expect(wrapper.get('[data-testid="create-models-list-loading"]').attributes('role')).toBe('status')
    request.resolve(['gpt-5'])
    await flushPromises()

    expect(wrapper.find('[data-testid="create-models-list-loading"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('gpt-5')
  })

  it('shows the legitimate empty state only after a successful empty response', async () => {
    const wrapper = await mountView()
    getModelsListCandidates.mockResolvedValueOnce([])

    await openCreateModelsList(wrapper)
    await flushPromises()

    expect(wrapper.text()).toContain('admin.groups.modelsList.empty')
    expect(wrapper.find('[data-testid="create-models-list-error"]').exists()).toBe(false)
  })

  it('shows an actionable error instead of empty and recovers on retry', async () => {
    const wrapper = await mountView()
    getModelsListCandidates
      .mockRejectedValueOnce(new Error('candidate outage'))
      .mockResolvedValueOnce(['claude-sonnet'])

    await openCreateModelsList(wrapper)
    await flushPromises()

    const error = wrapper.get('[data-testid="create-models-list-error"]')
    expect(error.attributes('role')).toBe('alert')
    expect(error.text()).toContain('admin.groups.modelsList.loadFailed')
    expect(wrapper.text()).not.toContain('admin.groups.modelsList.empty')

    await wrapper.get('[data-testid="create-models-list-retry"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="create-models-list-error"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('claude-sonnet')
    expect(getModelsListCandidates).toHaveBeenLastCalledWith(0, 'anthropic')
  })

  it('ignores a stale failure after a newer platform request succeeds', async () => {
    const wrapper = await mountView()
    const staleRequest = deferred<string[]>()
    getModelsListCandidates
      .mockImplementationOnce(() => staleRequest.promise)
      .mockResolvedValueOnce(['gpt-5'])

    await openCreateModelsList(wrapper)
    await wrapper.get('[data-tour="group-form-platform"]').setValue('openai')
    await flushPromises()
    // Platform changes intentionally reset this optional feature to disabled.
    await wrapper.get('[data-testid="create-models-list-toggle"]').trigger('click')

    staleRequest.reject(new Error('stale outage'))
    await flushPromises()

    expect(wrapper.find('[data-testid="create-models-list-error"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('gpt-5')
  })

  it('keeps create and edit candidate failures isolated', async () => {
    const wrapper = await mountView()
    getModelsListCandidates
      .mockRejectedValueOnce(new Error('create outage'))
      .mockResolvedValueOnce(['gpt-5'])

    await openCreateModelsList(wrapper)
    await flushPromises()
    expect(wrapper.find('[data-testid="create-models-list-error"]').exists()).toBe(true)

    const editButton = wrapper.findAll('button').find(button => button.text() === 'common.edit')
    expect(editButton).toBeTruthy()
    await editButton!.trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="edit-models-list-toggle"]').trigger('click')

    expect(wrapper.find('[data-testid="create-models-list-error"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="edit-models-list-error"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="edit-models-list-loading"]').exists()).toBe(false)
    expect(getModelsListCandidates).toHaveBeenLastCalledWith(42, 'openai')
  })
})
