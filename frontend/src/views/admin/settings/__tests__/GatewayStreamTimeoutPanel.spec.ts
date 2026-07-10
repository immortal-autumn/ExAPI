import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import GatewayStreamTimeoutPanel from '../gateway/GatewayStreamTimeoutPanel.vue'

describe('GatewayStreamTimeoutPanel', () => {
  it('renders stream timeout settings and delegates save', async () => {
    const saveStreamTimeoutSettings = vi.fn()

    const wrapper = mount(GatewayStreamTimeoutPanel, {
      props: {
        streamTimeoutLoading: false,
        streamTimeoutSaving: false,
        streamTimeoutForm: {
          enabled: true,
          action: 'temp_unsched',
          temp_unsched_minutes: 10,
          threshold_count: 3,
          threshold_window_minutes: 5,
        },
        saveStreamTimeoutSettings,
      },
      global: {
        stubs: {
          Toggle: { template: '<input type="checkbox" />' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.streamTimeout.title')
    expect(wrapper.text()).toContain('admin.settings.streamTimeout.tempUnschedMinutes')

    await wrapper.find('[data-test="save-stream-timeout"]').trigger('click')

    expect(saveStreamTimeoutSettings).toHaveBeenCalledTimes(1)
  })
})
