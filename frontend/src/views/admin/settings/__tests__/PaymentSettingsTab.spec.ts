import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/views/admin/settings/payment/PaymentSystemSettingsPanel.vue', () => ({
  default: { template: '<div>payment-system-settings-panel</div>' },
}))

vi.mock('@/views/admin/settings/payment/PaymentProviderManagementPanel.vue', () => ({
  default: { template: '<div>payment-provider-management-panel</div>' },
}))

import PaymentSettingsTab from '../tabs/PaymentSettingsTab.vue'

describe('PaymentSettingsTab', () => {
  it('renders payment settings panels', () => {
    const fn = vi.fn()
    const wrapper = mount(PaymentSettingsTab, {
      props: {
        form: { payment_enabled: true, payment_enabled_types: ['alipay'] },
        loadBalanceOptions: [],
        cancelRateLimitModeOptions: [],
        cancelRateLimitUnitOptions: [],
        allPaymentTypes: [],
        paymentGuideHref: 'https://example.test/guide',
        paymentMethodsHref: 'https://example.test/methods',
        togglePaymentType: fn,
        isPaymentTypeEnabled: () => true,
        providers: [],
        providersLoading: false,
        hasAnyPaymentTypeEnabled: true,
        loadProviders: fn,
        openCreateProvider: fn,
        openEditProvider: fn,
        confirmDeleteProvider: fn,
        handleToggleField: fn,
        handleToggleType: fn,
        handleReorderProviders: fn,
      },
    })

    expect(wrapper.text()).toContain('payment-system-settings-panel')
    expect(wrapper.text()).toContain('payment-provider-management-panel')
  })
})
