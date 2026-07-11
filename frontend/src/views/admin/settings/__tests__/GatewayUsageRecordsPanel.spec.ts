import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import GatewayUsageRecordsPanel from '../gateway/GatewayUsageRecordsPanel.vue'

describe('GatewayUsageRecordsPanel', () => {
  it('renders usage records visibility toggle', () => {
    const wrapper = mount(GatewayUsageRecordsPanel, {
      props: {
        form: {
          allow_user_view_error_requests: true,
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.usageRecords.title')
    expect(wrapper.text()).toContain('admin.settings.user_error_view.label')
    expect(wrapper.find('input[type="checkbox"]').element.checked).toBe(true)
  })
})
