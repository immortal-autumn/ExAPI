import { flushPromises, shallowMount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { defineComponent, h } from 'vue'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as authApi from '@/api/auth'
import { useAppStore, useAuthStore } from '@/stores'
import LoginView from '../LoginView.vue'

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  missingWarn: false,
  fallbackWarn: false,
  messages: { en: {} },
})

const TotpLoginModalStub = defineComponent({
  name: 'TotpLoginModal',
  emits: ['verify', 'cancel'],
  setup(_, { expose }) {
    expose({ setVerifying: vi.fn(), setError: vi.fn() })
    return () => h('div')
  },
})

type MountedLogin = {
  wrapper: VueWrapper
  router: Router
  login: ReturnType<typeof vi.spyOn>
  login2FA: ReturnType<typeof vi.spyOn>
}

async function mountPrivateLogin(loginResponse: unknown): Promise<MountedLogin> {
  const pinia = createPinia()
  setActivePinia(pinia)
  const authStore = useAuthStore(pinia)
  const appStore = useAppStore(pinia)
  const login = vi.spyOn(authStore, 'login').mockResolvedValue(loginResponse as never)
  const login2FA = vi.spyOn(authStore, 'login2FA').mockResolvedValue(undefined as never)
  vi.spyOn(appStore, 'showError').mockImplementation(() => undefined)
  vi.spyOn(appStore, 'showWarning').mockImplementation(() => undefined)
  vi.spyOn(appStore, 'showSuccess').mockImplementation(() => undefined)

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/login', component: LoginView },
      { path: '/admin/dashboard', component: { template: '<div />' } },
    ],
  })
  await router.push({
    path: '/login',
    query: { redirect: ['/admin/settings', '//evil.example'] },
  })
  await router.isReady()

  const wrapper = shallowMount(LoginView, {
    global: {
      plugins: [pinia, router, i18n],
      stubs: {
        AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
        Icon: true,
        RouterLink: true,
        TurnstileWidget: true,
        LoginAgreementPrompt: true,
        LinuxDoOAuthSection: true,
        DingTalkOAuthSection: true,
        WechatOAuthSection: true,
        OidcOAuthSection: true,
        EmailOAuthButtons: true,
        TotpLoginModal: TotpLoginModalStub,
      },
    },
  })
  await flushPromises()
  await wrapper.get('#email').setValue('admin@example.invalid')
  await wrapper.get('#password').setValue('correct-horse-battery-staple')
  return { wrapper, router, login, login2FA }
}

describe('private LoginView product boundary', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    sessionStorage.clear()
    localStorage.clear()
    vi.spyOn(authApi, 'getPublicSettings').mockResolvedValue({
      turnstile_enabled: true,
      turnstile_site_key: 'stale-site-key',
      linuxdo_oauth_enabled: true,
      dingtalk_oauth_enabled: true,
      wechat_oauth_enabled: true,
      backend_mode_enabled: false,
      oidc_oauth_enabled: true,
      oidc_oauth_provider_name: 'Stale Corp Login',
      github_oauth_enabled: true,
      google_oauth_enabled: true,
      password_reset_enabled: true,
      login_agreement_enabled: true,
      login_agreement_mode: 'checkbox',
      login_agreement_revision: 'stale-revision',
      login_agreement_documents: [{ id: 'terms', title: 'Stale Terms' }],
    } as never)
  })

  it('suppresses stale customer login controls in private mode', async () => {
    const { wrapper } = await mountPrivateLogin({})

    expect(wrapper.findComponent({ name: 'TurnstileWidget' }).exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'LoginAgreementPrompt' }).exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'EmailOAuthButtons' }).exists()).toBe(false)
    expect(wrapper.find('router-link-stub[to="/forgot-password"]').exists()).toBe(false)
  })

  it('falls back safely after normal login when redirect is repeated', async () => {
    const { wrapper, router, login } = await mountPrivateLogin({})
    const push = vi.spyOn(router, 'push')

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(login).toHaveBeenCalledOnce()
    expect(push).toHaveBeenCalledWith('/admin/dashboard')
  })

  it('falls back safely after 2FA completion when redirect is repeated', async () => {
    const { wrapper, router, login2FA } = await mountPrivateLogin({
      requires_2fa: true,
      temp_token: 'temporary-test-token',
      user_email_masked: 'a***@example.invalid',
    })
    const push = vi.spyOn(router, 'push')

    await wrapper.get('form').trigger('submit')
    await flushPromises()
    const modal = wrapper.findComponent({ name: 'TotpLoginModal' })
    expect(modal.exists()).toBe(true)
    modal.vm.$emit('verify', '123456')
    await flushPromises()

    expect(login2FA).toHaveBeenCalledWith('temporary-test-token', '123456')
    expect(push).toHaveBeenCalledWith('/admin/dashboard')
  })
})
