// @vitest-environment node

import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../SettingsView.vue')
const source = readFileSync(componentPath, 'utf8')

const retainedTabs = [
  'GeneralSettingsTab',
  'SecuritySettingsTab',
  'GatewaySettingsTab',
  'EmailSettingsTab',
  'BackupSettingsTab',
]

describe('SettingsView operator tab loading', () => {
  it('lazy-loads each retained settings tab', () => {
    for (const tab of retainedTabs) {
      expect(source).toContain(
        `const ${tab} = defineAsyncComponent(() => import("@/views/admin/settings/tabs/${tab}.vue"))`,
      )
      expect(source).not.toContain(`import ${tab} from`)
    }
  })

  it('mounts retained tabs only when selected', () => {
    for (const key of ['general', 'security', 'gateway', 'email', 'backup']) {
      expect(source).toContain(`v-if="activeTab === '${key}'"`)
    }
  })

  it('never mounts unreachable customer and commercial panels', () => {
    for (const tab of ['agreement', 'features', 'users', 'payment']) {
      expect(source).not.toContain(`v-show="activeTab === '${tab}'"`)
    }
  })

  it('guards customer and commercial loaders outside private mode', () => {
    expect(source).toContain('if (!isSingleUserPrivateControlPlaneBrowser())')
    expect(source).toContain('loadSubscriptionGroups();')
    expect(source).toContain('loadProviders();')
  })
})
