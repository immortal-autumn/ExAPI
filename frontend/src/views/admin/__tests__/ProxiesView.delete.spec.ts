import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { Proxy } from '@/types'
import ProxiesView from '@/views/admin/ProxiesView.vue'

const {
  listProxies,
  getAllWithCount,
  deleteProxy,
  batchDeleteProxies,
  showSuccess,
  showInfo,
  showError,
} = vi.hoisted(() => ({
  listProxies: vi.fn(),
  getAllWithCount: vi.fn(),
  deleteProxy: vi.fn(),
  batchDeleteProxies: vi.fn(),
  showSuccess: vi.fn(),
  showInfo: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/operator', () => ({
  operatorAPI: {
    proxies: {
      list: listProxies,
      getAllWithCount,
      delete: deleteProxy,
      batchDelete: batchDeleteProxies,
      create: vi.fn(),
      batchCreate: vi.fn(),
      update: vi.fn(),
      testProxy: vi.fn(),
      checkProxyQuality: vi.fn(),
      exportData: vi.fn(),
      getProxyAccounts: vi.fn(),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showInfo, showError }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const makeProxy = (id: number): Proxy => ({
  id,
  name: `Proxy ${id}`,
  protocol: 'http',
  host: `proxy-${id}.example.test`,
  port: 8080 + id,
  username: null,
  has_password: false,
  status: 'active',
  account_count: 0,
  expires_at: null,
  fallback_mode: 'none',
  backup_proxy_id: null,
  expiry_warn_days: 7,
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
})

const proxies = [makeProxy(7), makeProxy(8), makeProxy(9)]

const DataTableStub = defineComponent({
  props: { data: { type: Array, default: () => [] } },
  template: `
    <div>
      <slot name="header-select" />
      <div v-for="row in data" :key="row.id">
        <slot name="cell-select" :row="row" />
        <slot name="cell-actions" :row="row" />
      </div>
    </div>
  `,
})

const ConfirmDialogStub = defineComponent({
  props: ['show', 'title', 'loading'],
  emits: ['confirm', 'cancel'],
  template: `
    <div v-if="show" :data-testid="title === 'admin.proxies.deleteProxy' ? 'single-dialog' : 'batch-dialog'" :data-loading="String(loading)">
      <button
        :data-testid="title === 'admin.proxies.deleteProxy' ? 'single-confirm' : 'batch-confirm'"
        :disabled="loading"
        @click="$emit('confirm')"
      >confirm</button>
      <button
        :data-testid="title === 'admin.proxies.deleteProxy' ? 'single-cancel' : 'batch-cancel'"
        :disabled="loading"
        @click="$emit('cancel')"
      >cancel</button>
    </div>
  `,
})

const BaseDialogStub = defineComponent({
  props: { show: Boolean },
  template: '<section v-if="show"><slot /><slot name="footer" /></section>',
})

function mountView() {
  return mount(ProxiesView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        TablePageLayout: { template: '<section><slot name="filters" /><slot name="table" /><slot name="pagination" /></section>' },
        DataTable: DataTableStub,
        Pagination: true,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: ConfirmDialogStub,
        EmptyState: true,
        ImportDataModal: true,
        Select: true,
        Icon: true,
        PlatformTypeBadge: true,
      },
    },
  })
}

function rowCheckboxes(wrapper: ReturnType<typeof mount>) {
  return wrapper.findAll('input[type="checkbox"]').slice(1)
}

async function selectRows(wrapper: ReturnType<typeof mount>, indexes: number[]) {
  const checkboxes = rowCheckboxes(wrapper)
  for (const index of indexes) {
    await checkboxes[index].setValue(true)
  }
}

describe('ProxiesView destructive actions', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.spyOn(console, 'error').mockImplementation(() => {})
    listProxies.mockReset().mockResolvedValue({ items: proxies, total: proxies.length, page: 1, page_size: 20, pages: 1 })
    getAllWithCount.mockReset().mockResolvedValue(proxies)
    deleteProxy.mockReset().mockResolvedValue({ message: 'deleted' })
    batchDeleteProxies.mockReset().mockResolvedValue({ deleted_ids: [7, 8, 9], skipped: [] })
    showSuccess.mockReset()
    showInfo.mockReset()
    showError.mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('sends one single-delete request and disables confirmation while pending', async () => {
    let resolveDelete!: (value: { message: string }) => void
    deleteProxy.mockImplementationOnce(() => new Promise(resolve => { resolveDelete = resolve }))
    const wrapper = mountView()
    await flushPromises()

    const deleteButton = wrapper.findAll('button').find(button => button.text() === 'common.delete')
    expect(deleteButton).toBeTruthy()
    await deleteButton!.trigger('click')
    const confirm = wrapper.get('[data-testid="single-confirm"]')
    void confirm.trigger('click')
    void confirm.trigger('click')
    await wrapper.vm.$nextTick()

    expect(deleteProxy).toHaveBeenCalledTimes(1)
    expect(deleteProxy).toHaveBeenCalledWith(7)
    expect(confirm.attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="single-dialog"]').attributes('data-loading')).toBe('true')

    resolveDelete({ message: 'deleted' })
    await flushPromises()

    expect(showSuccess).toHaveBeenCalledWith('admin.proxies.proxyDeleted')
    expect(listProxies).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="single-dialog"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps single-delete confirmation recoverable after failure', async () => {
    deleteProxy.mockRejectedValueOnce(new Error('delete failed'))
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text() === 'common.delete')!.trigger('click')
    await wrapper.get('[data-testid="single-confirm"]').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('admin.proxies.failedToDelete')
    expect(wrapper.get('[data-testid="single-dialog"]').attributes('data-loading')).toBe('false')
    expect(wrapper.get('[data-testid="single-confirm"]').attributes('disabled')).toBeUndefined()

    deleteProxy.mockResolvedValueOnce({ message: 'deleted' })
    await wrapper.get('[data-testid="single-confirm"]').trigger('click')
    await flushPromises()
    expect(deleteProxy).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="single-dialog"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('snapshots batch IDs and keeps skipped selections after partial success', async () => {
    let resolveBatch!: (value: { deleted_ids: number[]; skipped: Array<{ id: number; reason: string }> }) => void
    batchDeleteProxies.mockImplementationOnce(() => new Promise(resolve => { resolveBatch = resolve }))
    const wrapper = mountView()
    await flushPromises()
    await selectRows(wrapper, [0, 1])

    await wrapper.findAll('button').find(button => button.text() === 'admin.proxies.batchDeleteAction')!.trigger('click')
    const confirm = wrapper.get('[data-testid="batch-confirm"]')
    void confirm.trigger('click')
    void confirm.trigger('click')
    await wrapper.vm.$nextTick()

    await selectRows(wrapper, [2])
    expect(batchDeleteProxies).toHaveBeenCalledTimes(1)
    expect(batchDeleteProxies).toHaveBeenCalledWith([7, 8])
    expect(confirm.attributes('disabled')).toBeDefined()

    resolveBatch({ deleted_ids: [7], skipped: [{ id: 8, reason: 'proxy in use' }] })
    await flushPromises()

    expect(wrapper.find('[data-testid="batch-dialog"]').exists()).toBe(false)
    expect(listProxies).toHaveBeenCalledTimes(2)
    const checkboxes = rowCheckboxes(wrapper)
    expect((checkboxes[0].element as HTMLInputElement).checked).toBe(false)
    expect((checkboxes[1].element as HTMLInputElement).checked).toBe(true)
    expect((checkboxes[2].element as HTMLInputElement).checked).toBe(true)
    wrapper.unmount()
  })

  it('keeps batch confirmation and selections after failure', async () => {
    batchDeleteProxies.mockRejectedValueOnce(new Error('batch failed'))
    const wrapper = mountView()
    await flushPromises()
    await selectRows(wrapper, [0, 1])

    await wrapper.findAll('button').find(button => button.text() === 'admin.proxies.batchDeleteAction')!.trigger('click')
    await wrapper.get('[data-testid="batch-confirm"]').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('admin.proxies.batchDeleteFailed')
    expect(wrapper.get('[data-testid="batch-dialog"]').attributes('data-loading')).toBe('false')
    expect((rowCheckboxes(wrapper)[0].element as HTMLInputElement).checked).toBe(true)
    expect((rowCheckboxes(wrapper)[1].element as HTMLInputElement).checked).toBe(true)
    await wrapper.get('[data-testid="batch-cancel"]').trigger('click')
    expect(wrapper.find('[data-testid="batch-dialog"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
