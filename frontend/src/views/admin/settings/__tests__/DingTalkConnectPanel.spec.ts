import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import DingTalkConnectPanel from '../panels/DingTalkConnectPanel.vue'

describe('DingTalkConnectPanel', () => {
  it('renders DingTalk OAuth and internal corp sync controls', () => {
    const wrapper = mount(DingTalkConnectPanel, {
      props: {
        form: {
          dingtalk_connect_enabled: true,
          dingtalk_connect_client_id: '',
          dingtalk_connect_client_secret: '',
          dingtalk_connect_client_secret_configured: true,
          dingtalk_connect_redirect_url: '',
          dingtalk_connect_corp_restriction_policy: 'internal_only',
          dingtalk_connect_bypass_registration: true,
          dingtalk_connect_sync_display_name: true,
          dingtalk_connect_sync_display_name_attr_key: 'dingtalk_name',
          dingtalk_connect_sync_display_name_attr_name: '',
          dingtalk_connect_sync_corp_email: true,
          dingtalk_connect_sync_corp_email_attr_key: 'dingtalk_email',
          dingtalk_connect_sync_corp_email_attr_name: '',
          dingtalk_connect_sync_dept: true,
          dingtalk_connect_sync_dept_attr_key: 'dingtalk_department',
          dingtalk_connect_sync_dept_attr_name: '',
        },
        localText: (zh: string, _en: string) => zh,
      },
      global: {
        stubs: {
          Toggle: { template: '<input type="checkbox" />' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.dingtalk.title')
    expect(wrapper.text()).toContain('admin.settings.dingtalk.corpPolicy.label')
    expect(wrapper.find('input[placeholder="dingtalk_name"]').exists()).toBe(true)
    expect(wrapper.find('input[placeholder="dingtalk_email"]').exists()).toBe(true)
    expect(wrapper.find('input[placeholder="dingtalk_department"]').exists()).toBe(true)
  })
})
