import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import AgreementSettingsTab from '../tabs/AgreementSettingsTab.vue'

function createForm() {
  return {
    login_agreement_enabled: false,
    login_agreement_mode: 'modal' as const,
    login_agreement_updated_at: '2026-03-31',
    login_agreement_documents: [
      { id: 'terms', title: '服务条款', content_md: '# Terms' },
    ],
  }
}

describe('AgreementSettingsTab', () => {
  it('renders agreement controls and delegates document actions', async () => {
    const addLoginAgreementDocument = vi.fn()
    const removeLoginAgreementDocument = vi.fn()
    const form = createForm()

    const wrapper = mount(AgreementSettingsTab, {
      props: {
        form,
        localText: (zh: string) => zh,
        loginAgreementRoutePath: (doc: { id?: string }, index: number) => `/legal/${doc.id || index}`,
        addLoginAgreementDocument,
        removeLoginAgreementDocument,
      },
      global: {
        stubs: {
          Toggle: { template: '<input type="checkbox" />' },
          Icon: { template: '<span />' },
        },
      },
    })

    expect(wrapper.text()).toContain('登录条款确认')
    expect(wrapper.text()).toContain('/legal/terms')

    await wrapper.find('[data-test="add-login-agreement-document"]').trigger('click')
    expect(addLoginAgreementDocument).toHaveBeenCalledTimes(1)

    await wrapper.find('[data-test="remove-login-agreement-document-0"]').trigger('click')
    expect(removeLoginAgreementDocument).toHaveBeenCalledWith(0)
  })
})
