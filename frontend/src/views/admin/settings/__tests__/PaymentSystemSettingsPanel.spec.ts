import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => params ? `${key}:${JSON.stringify(params)}` : key,
  }),
}))

vi.mock('@/components/common/Toggle.vue', () => ({
  default: { template: '<button type="button">toggle</button>' },
}))
vi.mock('@/components/common/Select.vue', () => ({
  default: { template: '<select></select>' },
}))
vi.mock('@/components/common/ImageUpload.vue', () => ({
  default: { template: '<div>image-upload</div>' },
}))
vi.mock('@/config/brand', () => ({
  getDefaultPaymentProductPrefix: () => 'ExAPI',
  getDefaultSiteName: () => 'ExAPI',
}))

import PaymentSystemSettingsPanel from '../payment/PaymentSystemSettingsPanel.vue'

describe('PaymentSystemSettingsPanel', () => {
  it('renders payment configuration and delegates payment type toggles', async () => {
    const togglePaymentType = vi.fn()
    const form = {
      payment_enabled: true,
      payment_product_name_prefix: '',
      payment_product_name_suffix: '',
      payment_min_amount: 1,
      payment_max_amount: 10000,
      payment_daily_limit: 50000,
      payment_balance_recharge_multiplier: 1,
      payment_subscription_usd_to_cny_rate: 7.2,
      payment_recharge_fee_rate: 2.5,
      payment_order_timeout_minutes: 30,
      payment_max_pending_orders: 3,
      payment_load_balance_strategy: 'round-robin',
      payment_cancel_rate_limit_enabled: true,
      payment_cancel_rate_limit_window_mode: 'sliding',
      payment_cancel_rate_limit_window: 10,
      payment_cancel_rate_limit_unit: 'minute',
      payment_cancel_rate_limit_max: 3,
      payment_alipay_force_qrcode: false,
      payment_enabled_types: ['alipay'],
      payment_help_image_url: '',
      payment_help_text: '',
    }

    const wrapper = mount(PaymentSystemSettingsPanel, {
      props: {
        form,
        loadBalanceOptions: [],
        cancelRateLimitModeOptions: [],
        cancelRateLimitUnitOptions: [],
        allPaymentTypes: [
          { value: 'alipay', label: 'Alipay' },
          { value: 'wechat', label: 'WeChat' },
        ],
        paymentGuideHref: 'https://example.test/payment-guide',
        paymentMethodsHref: 'https://example.test/payment-methods',
        togglePaymentType,
        isPaymentTypeEnabled: (value: string) => form.payment_enabled_types.includes(value),
      },
    })

    expect(wrapper.text()).toContain('admin.settings.payment.title')
    expect(wrapper.text()).toContain('ExAPI 100 CNY')
    expect(wrapper.text()).toContain('admin.settings.payment.balanceRechargePreview')
    expect(wrapper.text()).toContain('admin.settings.payment.rechargeFeePreview')
    expect(wrapper.text()).toContain('image-upload')

    await wrapper.findAll('button').find((button) => button.text() === 'WeChat')!.trigger('click')
    expect(togglePaymentType).toHaveBeenCalledWith('wechat')
  })
})
