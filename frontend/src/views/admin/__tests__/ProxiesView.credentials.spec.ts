import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { Proxy } from '@/types'
import ProxiesView from '@/views/admin/ProxiesView.vue'

const { listProxies, getAllWithCount, createProxy, batchCreateProxies, updateProxy, copyToClipboard, showError } = vi.hoisted(() => ({
  listProxies: vi.fn(),
  getAllWithCount: vi.fn(),
  createProxy: vi.fn(),
  batchCreateProxies: vi.fn(),
  updateProxy: vi.fn(),
  copyToClipboard: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/operator', () => ({
  operatorAPI: {
    proxies: {
      list: listProxies,
      getAllWithCount,
      create: createProxy,
      batchCreate: batchCreateProxies,
      update: updateProxy,
      testProxy: vi.fn(),
      checkProxyQuality: vi.fn(),
      exportData: vi.fn(),
      delete: vi.fn(),
      batchDelete: vi.fn(),
      getProxyAccounts: vi.fn(),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: vi.fn(), showInfo: vi.fn(), showError }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const proxy: Proxy & { password?: string } = {
  id: 7,
  name: 'Primary proxy',
  protocol: 'http',
  host: 'proxy.example.test',
  port: 8080,
  username: 'operator',
  has_password: true,
  password: 'legacy-plaintext-must-not-render',
  status: 'active',
  expires_at: null,
  fallback_mode: 'none',
  backup_proxy_id: null,
  expiry_warn_days: 7,
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
}

const DataTableStub = defineComponent({
  props: { data: { type: Array, default: () => [] } },
  template: `
    <div>
      <div v-for="row in data" :key="row.id">
        <slot name="cell-address" :row="row" />
        <slot name="cell-auth" :row="row" />
        <slot name="cell-actions" :row="row" />
      </div>
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
        ConfirmDialog: true,
        EmptyState: true,
        ImportDataModal: true,
        Select: true,
        Icon: true,
        PlatformTypeBadge: true,
      },
    },
  })
}

describe('ProxiesView credential minimization', () => {
  beforeEach(() => {
    localStorage.clear()
    listProxies.mockReset().mockResolvedValue({
      items: [proxy], total: 1, page: 1, page_size: 20, pages: 1,
    })
    getAllWithCount.mockReset().mockResolvedValue([proxy])
    createProxy.mockReset().mockResolvedValue(proxy)
    batchCreateProxies.mockReset().mockResolvedValue({ created: 1, skipped: 0 })
    updateProxy.mockReset().mockResolvedValue(proxy)
    copyToClipboard.mockReset()
    showError.mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('never renders or copies a plaintext proxy credential', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('admin.proxies.credentialConfigured')
    expect(wrapper.text()).not.toContain('legacy-plaintext-must-not-render')

    await wrapper.get('button[title="admin.proxies.copyProxyUrl"]').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith(
      'http://proxy.example.test:8080',
      'admin.proxies.urlCopied',
    )
    expect(JSON.stringify(copyToClipboard.mock.calls)).not.toContain('operator')
    expect(JSON.stringify(copyToClipboard.mock.calls)).not.toContain('legacy-plaintext-must-not-render')

    wrapper.unmount()
  })

  it('keeps the stored credential when an edit does not replace it', async () => {
    const wrapper = mountView()
    await flushPromises()

    const editButton = wrapper.findAll('button').find(button => button.text() === 'common.edit')
    expect(editButton).toBeTruthy()
    await editButton!.trigger('click')

    const passwordInput = wrapper.get('input[type="password"]')
    expect((passwordInput.element as HTMLInputElement).value).toBe('')

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(updateProxy).toHaveBeenCalledTimes(1)
    expect(updateProxy).toHaveBeenCalledWith(7, expect.not.objectContaining({ password: expect.anything() }))
    wrapper.unmount()
  })

  it('sends explicit nulls when an operator clears username and password', async () => {
    const wrapper = mountView()
    await flushPromises()

    const editButton = wrapper.findAll('button').find(button => button.text() === 'common.edit')
    expect(editButton).toBeTruthy()
    await editButton!.trigger('click')

    const vm = wrapper.vm as any
    vm.editForm.username = ''
    const passwordInput = wrapper.get('#edit-proxy-form input[type="password"]')
    await passwordInput.setValue('')
    await wrapper.get('#edit-proxy-form').trigger('submit')
    await flushPromises()

    expect(updateProxy).toHaveBeenCalledTimes(1)
    expect(updateProxy).toHaveBeenCalledWith(7, expect.objectContaining({
      username: null,
      password: null,
    }))
    wrapper.unmount()
  })

  it('ignores repeated standard-create submissions while the request is pending', async () => {
    let resolveCreate!: (value: Proxy) => void
    createProxy.mockImplementationOnce(() => new Promise(resolve => { resolveCreate = resolve }))
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text() === 'admin.proxies.createProxy')!.trigger('click')
    const form = wrapper.get('#create-proxy-form')
    const vm = wrapper.vm as any
    vm.createForm.name = 'New proxy'
    vm.createForm.host = 'new.proxy.example'
    vm.createForm.port = 8080
    void form.trigger('submit')
    void form.trigger('submit')
    await wrapper.vm.$nextTick()

    expect(createProxy).toHaveBeenCalledTimes(1)
    resolveCreate(proxy)
    await flushPromises()
    wrapper.unmount()
  })

  it('ignores repeated edit submissions while the request is pending', async () => {
    let resolveUpdate!: (value: Proxy) => void
    updateProxy.mockImplementationOnce(() => new Promise(resolve => { resolveUpdate = resolve }))
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text() === 'common.edit')!.trigger('click')
    const form = wrapper.get('#edit-proxy-form')
    void form.trigger('submit')
    void form.trigger('submit')
    await wrapper.vm.$nextTick()

    expect(updateProxy).toHaveBeenCalledTimes(1)
    resolveUpdate(proxy)
    await flushPromises()
    wrapper.unmount()
  })

  it('ignores repeated batch-create submissions while the request is pending', async () => {
    let resolveBatch!: (value: { created: number; skipped: number }) => void
    batchCreateProxies.mockImplementationOnce(() => new Promise(resolve => { resolveBatch = resolve }))
    const wrapper = mountView()
    await flushPromises()

    const vm = wrapper.vm as any
    vm.createMode = 'batch'
    vm.batchParseResult.valid = 1
    vm.batchParseResult.proxies = [{ protocol: 'http', host: 'batch.proxy.example', port: 8080, username: '', password: '' }]
    void vm.handleBatchCreate()
    void vm.handleBatchCreate()
    await wrapper.vm.$nextTick()

    expect(batchCreateProxies).toHaveBeenCalledTimes(1)
    resolveBatch({ created: 1, skipped: 0 })
    await flushPromises()
    wrapper.unmount()
  })
})
