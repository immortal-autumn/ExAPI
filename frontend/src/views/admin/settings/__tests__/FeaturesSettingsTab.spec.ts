import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => params?.count ? `${key} ${params.count}` : key,
  }),
}))

import FeaturesSettingsTab from '../tabs/FeaturesSettingsTab.vue'

function createForm() {
  return {
    channel_monitor_enabled: true,
    channel_monitor_default_interval_seconds: 60,
    available_channels_enabled: true,
    risk_control_enabled: false,
    cyber_session_block_enabled: false,
    cyber_session_block_ttl_seconds: 60,
    affiliate_enabled: true,
    affiliate_rebate_rate: 20,
    affiliate_rebate_freeze_hours: 24,
    affiliate_rebate_duration_days: 365,
    affiliate_rebate_per_invitee_cap: 0,
  }
}

describe('FeaturesSettingsTab', () => {
  it('renders feature cards and delegates affiliate actions', async () => {
    const openAffiliateModal = vi.fn()
    const wrapper = mount(FeaturesSettingsTab, {
      props: {
        form: createForm(),
        affiliateState: {
          search: '',
          selected: [],
          entries: [],
          loading: false,
          total: 0,
          page: 1,
          pageSize: 20,
        },
        affiliateModal: {
          open: false,
          mode: 'add',
          selectedUser: null,
          userQuery: '',
          userResults: [],
          editingEntry: null,
          code: '',
          rate: '',
          saving: false,
        },
        affiliateBatchModal: {
          open: false,
          rate: '',
          saving: false,
        },
        affiliateModalCanSubmit: false,
        openAffiliateModal,
        onAffiliateSearchInput: vi.fn(),
        openAffiliateBatchModal: vi.fn(),
        toggleAffiliateSelectAll: vi.fn(),
        toggleAffiliateSelect: vi.fn(),
        askResetAffiliateUser: vi.fn(),
        changeAffiliatePage: vi.fn(),
        closeAffiliateModal: vi.fn(),
        clearSelectedAffiliateUser: vi.fn(),
        onAffiliateUserSearchInput: vi.fn(),
        selectAffiliateUser: vi.fn(),
        submitAffiliateModal: vi.fn(),
        submitAffiliateBatchModal: vi.fn(),
      },
      global: {
        stubs: {
          Toggle: { template: '<input type="checkbox" />' },
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.features.channelMonitor.title')
    expect(wrapper.text()).toContain('admin.settings.features.affiliate.title')

    await wrapper.find('[data-test="open-affiliate-modal"]').trigger('click')
    expect(openAffiliateModal).toHaveBeenCalledWith(null)
  })
})
