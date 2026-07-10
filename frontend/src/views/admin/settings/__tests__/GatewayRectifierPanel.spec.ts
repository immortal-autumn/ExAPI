import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import GatewayRectifierPanel from '../gateway/GatewayRectifierPanel.vue'

describe('GatewayRectifierPanel', () => {
  it('renders rectifier settings, mutates pattern list, and delegates save', async () => {
    const saveRectifierSettings = vi.fn()
    const rectifierForm = {
      enabled: true,
      thinking_signature_enabled: true,
      thinking_budget_enabled: true,
      apikey_signature_enabled: true,
      apikey_signature_patterns: ['sk-[A-Za-z0-9]+'],
    }

    const wrapper = mount(GatewayRectifierPanel, {
      props: {
        rectifierLoading: false,
        rectifierSaving: false,
        rectifierForm,
        saveRectifierSettings,
      },
      global: {
        stubs: {
          Toggle: { template: '<input type="checkbox" />' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.rectifier.title')
    expect(wrapper.text()).toContain('admin.settings.rectifier.apikeyPatterns')

    await wrapper.find('[data-test="add-rectifier-pattern"]').trigger('click')
    expect(rectifierForm.apikey_signature_patterns).toHaveLength(2)

    await wrapper.find('[data-test="save-rectifier"]').trigger('click')
    expect(saveRectifierSettings).toHaveBeenCalledTimes(1)
  })
})
