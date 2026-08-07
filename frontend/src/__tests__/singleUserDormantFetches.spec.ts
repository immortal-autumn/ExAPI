// @vitest-environment node

import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const appSource = readFileSync(resolve(root, 'App.vue'), 'utf8')
const adminSettingsSource = readFileSync(resolve(root, 'stores/adminSettings.ts'), 'utf8')
const settingsViewSource = readFileSync(resolve(root, 'views/admin/SettingsView.vue'), 'utf8')
const proxiesViewSource = readFileSync(resolve(root, 'views/admin/ProxiesView.vue'), 'utf8')

const privateModeHelper = 'isSingleUserPrivateControlPlaneBrowser'

describe('single-user dormant SaaS fetches', () => {
  it('guards global subscription and announcement lifecycle work in private mode', () => {
    expect(appSource).toContain(privateModeHelper)
    expect(appSource).toContain('const saasFeaturesEnabled = !isSingleUserPrivateControlPlaneBrowser()')
    expect(appSource).toContain('if (saasFeaturesEnabled && document.visibilityState')
    expect(appSource).toContain('if (saasFeaturesEnabled) {')
  })

  it('does not couple operator settings fetches to payment config in private mode', () => {
    expect(adminSettingsSource).toContain(privateModeHelper)
    expect(adminSettingsSource).toContain('isSingleUserPrivateControlPlaneBrowser()')
    expect(adminSettingsSource).toContain('Promise.resolve(null)')
  })

  it('keeps payment UI lazy and external proxy assets out of private operator surfaces', () => {
    expect(settingsViewSource).toContain(
      "const PaymentSettingsTab = defineAsyncComponent(() => import(\"@/views/admin/settings/tabs/PaymentSettingsTab.vue\"))",
    )
    expect(settingsViewSource).toContain('v-if="!privateProduct && showProviderDialog"')
    expect(settingsViewSource).not.toContain('import PaymentProviderDialog from')
    expect(settingsViewSource).not.toContain('import PaymentSettingsTab from')
    expect(proxiesViewSource).not.toContain('unpkg.com')
    expect(proxiesViewSource).not.toContain('sub2api.io')
  })
})
