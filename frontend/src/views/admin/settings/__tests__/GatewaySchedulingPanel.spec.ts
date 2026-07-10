import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import GatewaySchedulingPanel from '../gateway/GatewaySchedulingPanel.vue'

describe('GatewaySchedulingPanel', () => {
  it('renders scheduling toggles and advanced scheduler weights', () => {
    const form = {
      allow_ungrouped_key_scheduling: true,
      openai_advanced_scheduler_enabled: true,
      openai_advanced_scheduler_sticky_weighted_enabled: false,
      openai_advanced_scheduler_subscription_priority_enabled: true,
      openai_advanced_scheduler_weight_priority: '1.0',
    }

    const wrapper = mount(GatewaySchedulingPanel, {
      props: {
        form,
        openAIAdvancedSchedulerWeightFields: [
          {
            key: 'openai_advanced_scheduler_weight_priority',
            label: 'Priority',
            placeholder: '1.0',
          },
        ],
      },
      global: {
        stubs: {
          Toggle: { template: '<button type="button">toggle</button>' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.scheduling.title')
    expect(wrapper.text()).toContain('admin.settings.openaiExperimentalScheduler.title')
    expect(wrapper.text()).toContain('admin.settings.openaiExperimentalScheduler.weightsTitle')
    expect(wrapper.text()).toContain('Priority')
    expect(wrapper.find('input[placeholder="1.0"]').exists()).toBe(true)
  })
})
