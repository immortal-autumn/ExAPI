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

describe('single-user dormant SaaS fetches', () => {
  it('removes global subscription and announcement lifecycle work', () => {
    expect(appSource).not.toContain('fetchSubscriptions')
    expect(appSource).not.toContain('fetchAnnouncements')
    expect(appSource).not.toContain('markAnnouncement')
  })

  it('uses the curated operator facade without payment configuration fetches', () => {
    expect(adminSettingsSource).toContain("@/api/operator")
    expect(adminSettingsSource).toContain('adminAPI.settings.getSettings()')
    expect(adminSettingsSource).not.toContain('adminAPI.payment')
  })

  it('excludes commercial settings UI and external proxy assets from private operator surfaces', () => {
    expect(settingsViewSource).not.toContain('PaymentSettingsTab')
    expect(settingsViewSource).not.toContain('PaymentProviderDialog')
    expect(settingsViewSource).not.toContain('FeaturesSettingsTab')
    expect(settingsViewSource).not.toContain('affiliatesAPI')
    expect(settingsViewSource).not.toContain('adminAPI.payment')
    expect(proxiesViewSource).not.toContain('unpkg.com')
    expect(proxiesViewSource).not.toContain('sub2api.io')
  })
})
