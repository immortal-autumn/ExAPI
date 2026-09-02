// @vitest-environment node

import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../PrivateSettingsView.vue')
const source = readFileSync(componentPath, 'utf8')

const retainedTabs = [
  'GeneralSettingsTab',
  'EmailSettingsTab',
  'SecuritySettingsTab',
  'BackupSettingsTab',
]

describe('SettingsView operator tab loading', () => {
  it('loads only retained operator settings tabs', () => {
    for (const tab of retainedTabs) {
      expect(source).toContain(`import ${tab} from`)
    }
  })

  it('mounts retained tabs only when selected', () => {
    for (const key of ['general', 'gateway', 'email', 'security', 'backup']) {
      expect(source).toContain(`v-if="activeTab === '${key}'"`)
    }
  })

  it('never mounts unreachable customer and commercial panels', () => {
    for (const tab of ['agreement', 'features', 'users', 'payment']) {
      expect(source).not.toContain(`v-show="activeTab === '${tab}'"`)
    }
  })

  it('uses the fail-closed private settings payload', () => {
    expect(source).toContain('buildPrivateOperatorSettings')
    expect(source).not.toContain('stripPrivateProductSettings')
  })
})
