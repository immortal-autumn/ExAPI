// @vitest-environment node

import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const source = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../SettingsView.vue'),
  'utf8',
)

describe('SettingsView private product boundaries', () => {
  it('hides custom-menu configuration in the private product', () => {
    expect(source).toContain('const privateProduct = isSingleUserPrivateControlPlaneBrowser();')
    expect(source).toContain('v-if="!privateProduct" class="card"')
  })

  it('scrubs dormant customer settings before every private save', () => {
    expect(source).toContain('stripPrivateProductSettings(payload as Record<string, unknown>)')
  })
})
