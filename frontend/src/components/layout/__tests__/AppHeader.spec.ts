// @vitest-environment node

import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppHeader.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('AppHeader single-user gateway cleanup', () => {
  it('does not render SaaS announcement/subscription/balance widgets on the private gateway control plane', () => {
    expect(componentSource).not.toContain('AnnouncementBell')
    expect(componentSource).not.toContain('SubscriptionProgressMini')
    expect(componentSource).not.toContain('showSaasHeaderWidgets')
    expect(componentSource).toContain('isSingleUserPrivateControlPlaneBrowser')
    expect(componentSource).toContain(":to=\"privateGatewayControlPlane ? '/admin/api-keys' : '/keys'\"")
    expect(componentSource).toContain('v-if="!privateGatewayControlPlane" to="/profile"')
    expect(componentSource).toContain('!privateGatewayControlPlane.value && !authStore.isSimpleMode')
  })
})
