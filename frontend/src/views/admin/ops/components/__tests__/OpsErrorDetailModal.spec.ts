import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OpsErrorDetailModal from '../OpsErrorDetailModal.vue'

const mocks = vi.hoisted(() => ({
  getRequestErrorDetail: vi.fn(),
  getUpstreamErrorDetail: vi.fn(),
  listRequestErrorUpstreamErrors: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: mocks.showError
  })
}))

vi.mock('@/api/admin/ops', () => ({
  opsAPI: {
    getRequestErrorDetail: mocks.getRequestErrorDetail,
    getUpstreamErrorDetail: mocks.getUpstreamErrorDetail,
    listRequestErrorUpstreamErrors: mocks.listRequestErrorUpstreamErrors
  }
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

describe('OpsErrorDetailModal', () => {
  beforeEach(() => {
    mocks.getRequestErrorDetail.mockReset()
    mocks.getUpstreamErrorDetail.mockReset()
    mocks.listRequestErrorUpstreamErrors.mockReset()
    mocks.showError.mockReset()
  })

  it('does not surface raw provider bodies in the request detail view', async () => {
    mocks.getRequestErrorDetail.mockResolvedValue({
      id: 11,
      created_at: '2026-09-01T00:00:00Z',
      phase: 'request',
      type: 'api_error',
      error_owner: 'client',
      error_source: 'gateway',
      severity: 'P2',
      status_code: 502,
      platform: 'openai',
      model: 'gpt-4o-mini',
      resolved: false,
      client_request_id: 'crid-11',
      request_id: 'rid-11',
      message: 'Upstream request failed',
      user_email: 'user@example.com',
      account_name: 'acc',
      group_name: 'group',
      error_body: '{"provider":"xai","secret":"provider-body-leak","message":"forbidden"}',
      upstream_status_code: 503,
      is_business_limited: false
    })
    mocks.listRequestErrorUpstreamErrors.mockResolvedValue({
      items: [
        {
          id: 99,
          created_at: '2026-09-01T00:00:00Z',
          phase: 'upstream',
          type: 'api_error',
          error_owner: 'provider',
          error_source: 'provider',
          severity: 'P2',
          status_code: 503,
          platform: 'openai',
          model: 'gpt-4o-mini',
          resolved: false,
          client_request_id: 'crid-99',
          request_id: 'rid-99',
          message: 'Provider response failed',
          user_email: 'user@example.com',
          account_name: 'acc',
          group_name: 'group',
          error_body: '{"provider":"xai","secret":"upstream-preview-leak","message":"retry"}',
          upstream_status_code: 503,
          upstream_error_detail: '{"provider":"xai","secret":"upstream-preview-leak"}',
          is_business_limited: false
        }
      ],
      total: 1
    })

    const wrapper = mount(OpsErrorDetailModal, {
      props: {
        show: true,
        errorId: 11,
        errorType: 'request',
        timeRange: '24h'
      },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /></div>' },
          Select: { template: '<div><slot /></div>' },
          OpsErrorLogTable: { template: '<div />' },
          Icon: true
        }
      }
    })

    await flushPromises()
    await flushPromises()

    expect(mocks.getRequestErrorDetail).toHaveBeenCalledWith(11)
    expect(mocks.listRequestErrorUpstreamErrors).toHaveBeenCalledWith(
      11,
      { page: 1, page_size: 100, view: 'all' },
      { include_detail: true }
    )
    expect(wrapper.text()).not.toContain('provider-body-leak')
    expect(wrapper.text()).not.toContain('upstream-preview-leak')
    expect(wrapper.text()).toContain('admin.ops.errorDetail.responseBodyRedacted')

    const expandButtons = wrapper.findAll('button')
    const previewButton = expandButtons.find((button) =>
      button.text().includes('admin.ops.errorDetail.responsePreview.expand')
    )
    expect(previewButton).toBeTruthy()
    await previewButton!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).not.toContain('upstream-preview-leak')
  })
})
