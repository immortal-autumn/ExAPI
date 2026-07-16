import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import SingleUserCockpitPanel from '../components/SingleUserCockpitPanel.vue'

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: vi.fn().mockResolvedValue({ items: [] }),
    },
  },
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

describe('SingleUserCockpitPanel Chinese product copy', () => {
  it('renders primary operator copy in Chinese without English prose', async () => {
    const wrapper = mount(SingleUserCockpitPanel, {
      global: {
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
  })
})
