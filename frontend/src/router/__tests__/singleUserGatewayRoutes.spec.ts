// @vitest-environment node

import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const routerPath = resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts')
const routerSource = readFileSync(routerPath, 'utf8')

describe('single-user gateway route pruning', () => {
  it('does not create lazy chunks for restricted SaaS/payment/admin pages', () => {
    for (const legacyImport of [
      '@/views/user/PaymentView.vue',
      '@/views/user/UserOrdersView.vue',
      '@/views/user/SubscriptionsView.vue',
      '@/views/user/RedeemView.vue',
      '@/views/user/AffiliateView.vue',
      '@/views/admin/UsersView.vue',
      '@/views/admin/GroupsView.vue',
      '@/views/admin/orders/AdminOrdersView.vue',
      '@/views/admin/affiliates/AdminAffiliateInvitesView.vue',
    ]) {
      expect(routerSource).not.toContain(legacyImport)
    }

    expect(routerSource).toContain("@/views/SingleUserGatewayRedirectView.vue")
  })
})
