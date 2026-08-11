// @vitest-environment node

import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const routerPath = resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts')
const routerSource = readFileSync(routerPath, 'utf8')
const frontendSourceRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const accountTestModalSources = [
  'components/account/AccountTestModal.vue',
  'components/admin/account/AccountTestModal.vue',
].map((relativePath) => readFileSync(resolve(frontendSourceRoot, relativePath), 'utf8'))

describe('single-user gateway route pruning', () => {
  it('does not create lazy chunks for restricted SaaS/payment/admin pages', () => {
    for (const legacyImport of [
      '@/views/auth/RegisterView.vue',
      '@/views/auth/EmailVerifyView.vue',
      '@/views/auth/OAuthCallbackView.vue',
      '@/views/auth/LinuxDoCallbackView.vue',
      '@/views/auth/WechatCallbackView.vue',
      '@/views/auth/WechatPaymentCallbackView.vue',
      '@/views/auth/DingTalkCallbackView.vue',
      '@/views/auth/DingTalkEmailCompletionView.vue',
      '@/views/auth/OidcCallbackView.vue',
      '@/views/auth/ForgotPasswordView.vue',
      '@/views/auth/ResetPasswordView.vue',
      '@/views/public/LegalDocumentView.vue',
      '@/views/user/DashboardView.vue',
      '@/views/user/UsageView.vue',
      '@/views/user/ProfileView.vue',
      '@/views/user/CustomPageView.vue',
      '@/views/user/PaymentView.vue',
      '@/views/user/UserOrdersView.vue',
      '@/views/user/SubscriptionsView.vue',
      '@/views/user/RedeemView.vue',
      '@/views/user/AffiliateView.vue',
      '@/views/admin/UsersView.vue',
      '@/views/admin/orders/AdminOrdersView.vue',
      '@/views/admin/affiliates/AdminAffiliateInvitesView.vue',
    ]) {
      expect(routerSource).not.toContain(legacyImport)
    }
    expect(routerSource).toContain("import { privateRoutes } from './privateRoutes'")
    expect(routerSource).not.toContain('SingleUserGatewayRedirectView')
  })

  it('keeps account-test client fallbacks localized', () => {
    for (const source of accountTestModalSources) {
      for (const englishFallback of ['HTTP error!', 'No response body', 'Unknown error', 'Test failed', '`Error: ${msg}`']) {
        expect(source).not.toContain(englishFallback)
      }
      expect(source).toContain("t('admin.accounts.testHttpError'")
      expect(source).toContain("t('admin.accounts.testNoResponseBody')")
      expect(source).toContain("t('admin.accounts.testErrorLine'")
      expect(source).toContain("t('common.unknownError')")
    }
  })
})
