// @vitest-environment node

import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppHeader.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('AppHeader single-user gateway cleanup', () => {
  it('does not render SaaS announcement/subscription/balance widgets on the private gateway control plane', () => {
    expect(componentSource).toContain('const showSaasHeaderWidgets = computed(')
    expect(componentSource).toContain('<AnnouncementBell v-if="showSaasHeaderWidgets" />')
    expect(componentSource).toContain('<SubscriptionProgressMini v-if="showSaasHeaderWidgets" />')
    expect(componentSource).toContain('v-if="showSaasHeaderWidgets"')
    expect(componentSource).toContain('<div v-if="showSaasHeaderWidgets" class="border-b border-gray-100 px-4 py-2 dark:border-dark-700 sm:hidden">')
    expect(componentSource).toContain('isSingleUserPrivateControlPlaneBrowser')
    expect(componentSource).toContain("defineAsyncComponent(() => import('@/components/common/AnnouncementBell.vue'))")
    expect(componentSource).toContain("defineAsyncComponent(() => import('@/components/common/SubscriptionProgressMini.vue'))")
    expect(componentSource).toContain(":to=\"privateGatewayControlPlane ? '/admin/api-keys' : '/keys'\"")
    expect(componentSource).toContain('!privateGatewayControlPlane.value && !authStore.isSimpleMode')
  })
})
