import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import RetiredFeatureView from '@/views/RetiredFeatureView.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('RetiredFeatureView', () => {
  it('explains the retired feature and links back to the admin control plane', () => {
    const wrapper = mount(RetiredFeatureView, {
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })

    expect(wrapper.text()).toContain('410')
    expect(wrapper.text()).toContain('retiredFeature.title')
    expect(wrapper.text()).toContain('retiredFeature.description')
    expect(wrapper.find('a').text()).toBe('retiredFeature.returnToControlPlane')
  })
})
