import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/components/payment/PaymentProviderList.vue', () => ({
  default: {
    props: ['providers', 'loading', 'canCreate', 'enabledPaymentTypes', 'allPaymentTypes', 'redirectLabel'],
    emits: ['refresh', 'create', 'edit', 'delete', 'toggle-field', 'toggle-type', 'reorder'],
    template: `
      <div>
        payment-provider-list
        <button type="button" @click="$emit('refresh')">refresh</button>
        <button type="button" @click="$emit('create')">create</button>
        <button type="button" @click="$emit('edit', providers[0])">edit</button>
        <button type="button" @click="$emit('delete', providers[0])">delete</button>
        <button type="button" @click="$emit('toggle-field', providers[0], 'enabled')">toggle-field</button>
        <button type="button" @click="$emit('toggle-type', providers[0], 'alipay')">toggle-type</button>
        <button type="button" @click="$emit('reorder', [1, 2])">reorder</button>
      </div>
    `,
  },
}))

import PaymentProviderManagementPanel from '../payment/PaymentProviderManagementPanel.vue'

describe('PaymentProviderManagementPanel', () => {
  it('renders provider list only when payment is enabled and forwards list events', async () => {
    const handlers = {
      loadProviders: vi.fn(),
      openCreateProvider: vi.fn(),
      openEditProvider: vi.fn(),
      confirmDeleteProvider: vi.fn(),
      handleToggleField: vi.fn(),
      handleToggleType: vi.fn(),
      handleReorderProviders: vi.fn(),
    }
    const provider = { id: 1, name: 'Provider' }
    const wrapper = mount(PaymentProviderManagementPanel, {
      props: {
        form: { payment_enabled: true, payment_enabled_types: ['alipay'] },
        providers: [provider],
        providersLoading: false,
        hasAnyPaymentTypeEnabled: true,
        allPaymentTypes: [{ value: 'alipay', label: 'Alipay' }],
        ...handlers,
      },
    })

    expect(wrapper.text()).toContain('payment-provider-list')
    for (const label of ['refresh', 'create', 'edit', 'delete', 'toggle-field', 'toggle-type', 'reorder']) {
      await wrapper.findAll('button').find((button) => button.text() === label)!.trigger('click')
    }

    expect(handlers.loadProviders).toHaveBeenCalledOnce()
    expect(handlers.openCreateProvider).toHaveBeenCalledOnce()
    expect(handlers.openEditProvider).toHaveBeenCalledWith(provider)
    expect(handlers.confirmDeleteProvider).toHaveBeenCalledWith(provider)
    expect(handlers.handleToggleField).toHaveBeenCalledWith(provider, 'enabled')
    expect(handlers.handleToggleType).toHaveBeenCalledWith(provider, 'alipay')
    expect(handlers.handleReorderProviders).toHaveBeenCalledWith([1, 2])
  })
})
