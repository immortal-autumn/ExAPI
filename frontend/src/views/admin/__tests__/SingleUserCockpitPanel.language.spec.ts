import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'

import SingleUserCockpitPanel from '../components/SingleUserCockpitPanel.vue'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  getSummary: vi.fn(),
}))

vi.mock('@/api/operator', () => ({
  operatorAPI: {
    cockpit: {
      getSummary: mocks.getSummary,
    },
  },
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.push }),
}))

const summary = {
  generated_at: '2026-08-05T12:00:00Z',
  accounts: { total: 2, active: 1, inactive: 0, error: 1, dispatch_eligible: 1, quota_warning_total: 1 },
  platforms: [{ platform: 'openai', total: 2, active: 1, error: 1, dispatch_eligible: 1 }],
  quota_warnings: [{ account_id: 42, name: 'main', platform: 'openai', scope: 'daily', used: 9, limit: 10, percent: 90, severity: 'critical' }],
}

function mountPanel(locale: 'en' | 'zh-CN') {
  const i18n = createI18n({
    legacy: false,
    locale,
    messages: { en, 'zh-CN': zh },
  })
  return mount(SingleUserCockpitPanel, {
    global: {
      plugins: [createPinia(), i18n],
      stubs: { Icon: true },
    },
  })
}

afterEach(() => {
  mocks.getSummary.mockReset()
  mocks.push.mockReset()
})

describe('SingleUserCockpitPanel bilingual product copy', () => {
  it('renders primary operator copy in English by default', async () => {
    mocks.getSummary.mockResolvedValue(summary)
    const wrapper = mountPanel('en')
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Single-user cockpit')
    expect(text).toContain('Quota watch')
    expect(text).toContain('Upstream account dispatch')
    expect(text).toContain('Local integration')
    expect(text).not.toMatch(/[\u4e00-\u9fff]/)
    expect(wrapper.get('button[aria-label="View account main, quota usage 90%"]')).toBeTruthy()
  })

  it('renders primary operator copy in Chinese without English prose', async () => {
    mocks.getSummary.mockResolvedValue(summary)
    const wrapper = mountPanel('zh-CN')
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
