import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import UserErrorDetailModal from '../UserErrorDetailModal.vue'

const mocks = vi.hoisted(() => ({
  getMyErrorDetail: vi.fn()
}))

vi.mock('@/api/usage', () => ({
  getMyErrorDetail: mocks.getMyErrorDetail
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

describe('UserErrorDetailModal', () => {
  beforeEach(() => {
    mocks.getMyErrorDetail.mockReset()
  })

  it('redacts provider response bodies by default', async () => {
    mocks.getMyErrorDetail.mockResolvedValue({
      id: 7,
      created_at: '2026-09-01T00:00:00Z',
      model: 'gpt-4o-mini',
      inbound_endpoint: '/v1/chat/completions',
      status_code: 502,
      category: 'upstream',
      platform: 'openai',
      message: 'Upstream request failed',
      key_name: 'primary',
      key_deleted: false,
      error_body: '{"provider":"xai","api_key":"secret-token","message":"forbidden"}',
      upstream_status_code: 503
    })

    const wrapper = mount(UserErrorDetailModal, {
      props: { show: false, errorId: 7 },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /></div>' }
        }
      }
    })

    await wrapper.setProps({ show: true })
    await flushPromises()
    await flushPromises()

    expect(mocks.getMyErrorDetail).toHaveBeenCalledWith(7)
    expect(wrapper.text()).not.toContain('secret-token')
    expect(wrapper.text()).not.toContain('xai')
    expect(wrapper.text()).toContain('usage.errors.detail.responseBodyRedacted')
  })
})
