import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => params?.n ? `${key} ${params.n}` : key,
  }),
}))

import GeneralSettingsTab from '../tabs/GeneralSettingsTab.vue'

function createForm() {
  return {
    backend_mode_enabled: false,
    site_name: 'ExAPI',
    site_subtitle: 'Private gateway',
    api_base_url: '',
    table_default_page_size: 20,
    custom_endpoints: [
      { name: 'OpenAI', endpoint: 'https://api.example.com/v1', description: 'Primary endpoint' },
    ],
    contact_info: 'admin@example.com',
    doc_url: 'https://docs.example.com',
    site_logo: '',
    home_content: 'Welcome',
    hide_ccs_import_button: false,
  }
}

describe('GeneralSettingsTab', () => {
  it('renders site identity fields and keeps parent form wiring', async () => {
    const addEndpoint = vi.fn()
    const removeEndpoint = vi.fn()
    const form = createForm()

    const wrapper = mount(GeneralSettingsTab, {
      props: {
        form,
        tablePageSizeOptionsInput: '10, 20, 50',
        addEndpoint,
        removeEndpoint,
      },
      global: {
        stubs: {
          Toggle: { template: '<input type="checkbox" />' },
          ImageUpload: { template: '<div data-test="image-upload" />' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.site.title')
    expect(wrapper.text()).toContain('admin.settings.site.siteLogo')
    expect(wrapper.find('input[type="text"]').exists()).toBe(true)

    await wrapper.find('[data-test="add-endpoint"]').trigger('click')
    expect(addEndpoint).toHaveBeenCalledTimes(1)

    await wrapper.find('[data-test="remove-endpoint-0"]').trigger('click')
    expect(removeEndpoint).toHaveBeenCalledWith(0)
  })
})
