import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import SettingsSectionCard from '../SettingsSectionCard.vue'

describe('SettingsSectionCard', () => {
  it('renders title, optional description, and body slots in the standard settings card shell', () => {
    const wrapper = mount(SettingsSectionCard, {
      slots: {
        title: 'Card title',
        description: 'Card description',
        default: '<button data-test="body-action">Action</button>',
      },
    })

    expect(wrapper.classes()).toContain('card')
    expect(wrapper.text()).toContain('Card title')
    expect(wrapper.text()).toContain('Card description')
    expect(wrapper.find('[data-test="body-action"]').exists()).toBe(true)
  })

  it('omits the description paragraph when no description slot is provided', () => {
    const wrapper = mount(SettingsSectionCard, {
      slots: {
        title: 'Only title',
        default: '<span>Body</span>',
      },
    })

    expect(wrapper.text()).toContain('Only title')
    expect(wrapper.text()).toContain('Body')
    expect(wrapper.find('[data-test="section-description"]').exists()).toBe(false)
  })
})
