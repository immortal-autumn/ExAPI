import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'

import SingleUserCockpitPanel from '../components/SingleUserCockpitPanel.vue'

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  getSummary: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    cockpit: {
      getSummary: mocks.getSummary,
    },
  },
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.push }),
}))

describe('SingleUserCockpitPanel Chinese product copy', () => {
  it('renders primary operator copy in Chinese without English prose', async () => {
    mocks.getSummary.mockResolvedValue({
      generated_at: '2026-08-05T12:00:00Z',
      accounts: { total: 2, active: 1, inactive: 0, error: 1, dispatch_eligible: 1, quota_warning_total: 1 },
      platforms: [{ platform: 'openai', total: 2, active: 1, error: 1, dispatch_eligible: 1 }],
      quota_warnings: [{ account_id: 42, name: 'main', platform: 'openai', scope: 'daily', used: 9, limit: 10, percent: 90, severity: 'critical' }],
    })
    const wrapper = mount(SingleUserCockpitPanel, {
      global: {
        plugins: [createPinia()],
        stubs: {
          Icon: true,
        },
      },
    })
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('单用户控制台')
    expect(text).toContain('额度监控')
    expect(text).toContain('上游账号调度')
    expect(text).toContain('本地集成')
    expect(text).not.toMatch(/Single-user cockpit|Refresh|Accounts|Wakeup ready|Quota watch|Errors|Local integration|Copy endpoints|Configure per account/)

    await wrapper.get('button[aria-label="查看账号 main，额度使用 90%"]')?.trigger('click')
    expect(mocks.push).toHaveBeenCalledWith({ path: '/admin/accounts', query: { account_id: '42' } })
  })
})
