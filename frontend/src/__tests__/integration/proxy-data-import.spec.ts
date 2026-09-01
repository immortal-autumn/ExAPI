import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ImportDataModal from '@/components/admin/proxy/ImportDataModal.vue'

const showError = vi.fn()
const showSuccess = vi.fn()

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
  })
}))

vi.mock('@/api/operator', () => ({
  operatorAPI: {
    proxies: {
      importData: vi.fn()
    }
  }
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string) => key
  })
}))

describe('Proxy ImportDataModal', () => {
  beforeEach(async () => {
    showError.mockReset()
    showSuccess.mockReset()
    const { operatorAPI } = await import('@/api/operator')
    vi.mocked(operatorAPI.proxies.importData).mockReset()
  })

  it('未选择文件时提示错误', async () => {
    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })

    await wrapper.find('form').trigger('submit')
    expect(showError).toHaveBeenCalledWith('admin.proxies.dataImportSelectFile')
  })

  it('无效 JSON 时提示解析失败', async () => {
    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })

    const input = wrapper.find('input[type="file"]')
    const file = new File(['invalid json'], 'data.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: () => Promise.resolve('invalid json')
    })
    Object.defineProperty(input.element, 'files', {
      value: [file]
    })

    await input.trigger('change')
    await wrapper.find('form').trigger('submit')
    await Promise.resolve()

    expect(showError).toHaveBeenCalledWith('admin.proxies.dataImportParseFailed')
  })

  it('部分成功时在关闭弹窗后通知父组件刷新', async () => {
    const { operatorAPI } = await import('@/api/operator')
    vi.mocked(operatorAPI.proxies.importData).mockResolvedValue({
      proxy_created: 1,
      proxy_reused: 0,
      proxy_failed: 1,
      account_created: 0,
      account_failed: 0,
    })
    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })

    const input = wrapper.find('input[type="file"]')
    const content = JSON.stringify({ proxies: [{ name: 'ok' }, { name: 'bad' }], accounts: [] })
    const file = new File([content], 'mixed.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', { value: () => Promise.resolve(content) })
    Object.defineProperty(input.element, 'files', { value: [file] })

    await input.trigger('change')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.emitted('imported')).toBeUndefined()
    await wrapper.findAll('button.btn-secondary')[1]!.trigger('click')
    expect(wrapper.emitted('imported')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('导入请求进行中时忽略重复提交', async () => {
    const { operatorAPI } = await import('@/api/operator')
    let resolveImport!: (value: {
      proxy_created: number
      proxy_reused: number
      proxy_failed: number
      account_created: number
      account_failed: number
    }) => void
    vi.mocked(operatorAPI.proxies.importData).mockImplementationOnce(
      () => new Promise(resolve => { resolveImport = resolve })
    )
    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })

    const input = wrapper.find('input[type="file"]')
    const content = JSON.stringify({ proxies: [{ name: 'only-once' }], accounts: [] })
    const file = new File([content], 'once.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', { value: () => Promise.resolve(content) })
    Object.defineProperty(input.element, 'files', { value: [file] })
    await input.trigger('change')

    const form = wrapper.find('form')
    void form.trigger('submit')
    await flushPromises()
    void form.trigger('submit')
    await wrapper.vm.$nextTick()

    expect(operatorAPI.proxies.importData).toHaveBeenCalledTimes(1)
    expect(wrapper.find('button.btn-primary').attributes('disabled')).toBeDefined()

    resolveImport({
      proxy_created: 1,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 0,
      account_failed: 0,
    })
    await flushPromises()
  })
})
