import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/components/common/ProxySelector.vue', () => ({
  default: { template: '<div>proxy-selector</div>' },
}))

import GatewayWebSearchEmulationPanel from '../gateway/GatewayWebSearchEmulationPanel.vue'

describe('GatewayWebSearchEmulationPanel', () => {
  it('renders web search providers and delegates actions', async () => {
    const addProvider = vi.fn()
    const removeProvider = vi.fn()
    const toggleProvider = vi.fn()
    const copyApiKey = vi.fn()
    const resetUsage = vi.fn()
    const openTestDialog = vi.fn()

    const wrapper = mount(GatewayWebSearchEmulationPanel, {
      props: {
        webSearchConfig: {
          enabled: true,
          providers: [
            {
              type: 'brave',
              api_key: 'secret',
              api_key_configured: true,
              quota_used: 25,
              quota_limit: 100,
              subscribed_at: 1704067200,
              proxy_id: null,
            },
          ],
        },
        expandedProviders: { 0: true },
        apiKeyVisible: { 0: false },
        webSearchProxies: [],
        addWebSearchProvider: addProvider,
        removeWebSearchProvider: removeProvider,
        toggleProviderExpand: toggleProvider,
        copyApiKey,
        formatSubscribedAt: () => '2024-01-01',
        parseSubscribedAt: () => 1704067200,
        quotaPercentage: () => 25,
        resetWebSearchUsage: resetUsage,
        openTestDialog,
      },
      global: {
        stubs: {
          Toggle: { template: '<button type="button">toggle</button>' },
          Select: { template: '<select><slot /></select>' },
          ProxySelector: { template: '<div>proxy-selector</div>' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.webSearchEmulation.title')
    expect(wrapper.text()).toContain('admin.settings.webSearchEmulation.providers')
    expect(wrapper.text()).toContain('25 / 100')
    expect(wrapper.text()).toContain('proxy-selector')

    await wrapper.findAll('button').find((button) => button.text().includes('admin.settings.webSearchEmulation.addProvider'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.settings.webSearchEmulation.removeProvider'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.settings.webSearchEmulation.resetUsage'))!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.settings.webSearchEmulation.test'))!.trigger('click')

    expect(addProvider).toHaveBeenCalledTimes(1)
    expect(removeProvider).toHaveBeenCalledWith(0)
    expect(resetUsage).toHaveBeenCalledWith(0)
    expect(openTestDialog).toHaveBeenCalledTimes(1)
  })
})
