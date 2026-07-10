import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import GatewayCooldownPanel from '../gateway/GatewayCooldownPanel.vue'

describe('GatewayCooldownPanel', () => {
  it('renders overload and 429 cooldown settings and delegates saves', async () => {
    const saveOverloadCooldownSettings = vi.fn()
    const saveRateLimit429CooldownSettings = vi.fn()

    const wrapper = mount(GatewayCooldownPanel, {
      props: {
        overloadCooldownLoading: false,
        overloadCooldownSaving: false,
        overloadCooldownForm: {
          enabled: true,
          cooldown_minutes: 15,
        },
        saveOverloadCooldownSettings,
        rateLimit429CooldownLoading: false,
        rateLimit429CooldownSaving: false,
        rateLimit429CooldownForm: {
          enabled: true,
          cooldown_seconds: 60,
        },
        saveRateLimit429CooldownSettings,
      },
      global: {
        stubs: {
          Toggle: { template: '<input type="checkbox" />' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.overloadCooldown.title')
    expect(wrapper.text()).toContain('admin.settings.rateLimit429Cooldown.title')

    await wrapper.find('[data-test="save-overload-cooldown"]').trigger('click')
    await wrapper.find('[data-test="save-rate-limit429-cooldown"]').trigger('click')

    expect(saveOverloadCooldownSettings).toHaveBeenCalledTimes(1)
    expect(saveRateLimit429CooldownSettings).toHaveBeenCalledTimes(1)
  })
})
