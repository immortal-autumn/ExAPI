import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import WeChatConnectPanel from '../panels/WeChatConnectPanel.vue'

describe('WeChatConnectPanel', () => {
  it('renders WeChat app variants and delegates redirect copy', async () => {
    const setAndCopyWeChatRedirectUrl = vi.fn()

    const wrapper = mount(WeChatConnectPanel, {
      props: {
        form: {
          wechat_connect_enabled: true,
          wechat_connect_open_enabled: true,
          wechat_connect_open_app_id: '',
          wechat_connect_open_app_secret: '',
          wechat_connect_open_app_secret_configured: true,
          wechat_connect_mp_enabled: true,
          wechat_connect_mp_app_id: '',
          wechat_connect_mp_app_secret: '',
          wechat_connect_mp_app_secret_configured: false,
          wechat_connect_mobile_enabled: true,
          wechat_connect_mobile_app_id: '',
          wechat_connect_mobile_app_secret: '',
          wechat_connect_mobile_app_secret_configured: false,
          wechat_connect_redirect_url: '',
          wechat_connect_frontend_redirect_url: '/auth/wechat/callback',
        },
        wechatRedirectUrlSuggestion: 'https://example.com/api/v1/auth/wechat/callback',
        setAndCopyWeChatRedirectUrl,
        handleWeChatOpenEnabledChange: vi.fn(),
        handleWeChatMPEnabledChange: vi.fn(),
        handleWeChatMobileEnabledChange: vi.fn(),
        localText: (zh: string, _en: string) => zh,
      },
      global: {
        stubs: {
          Toggle: { template: '<input type="checkbox" />' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.wechatConnect.title')
    expect(wrapper.text()).toContain('PC 应用')
    expect(wrapper.text()).toContain('公众号')
    expect(wrapper.text()).toContain('移动应用')
    expect(wrapper.text()).toContain('https://example.com/api/v1/auth/wechat/callback')

    await wrapper.find('[data-test="copy-wechat-redirect-url"]').trigger('click')
    expect(setAndCopyWeChatRedirectUrl).toHaveBeenCalledTimes(1)
  })
})
