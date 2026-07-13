// @vitest-environment node

import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const source = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../AccountsView.vue'),
  'utf8',
)

const lazyDialogs = [
  'CreateAccountModal',
  'EditAccountModal',
  'BulkEditAccountModal',
  'SyncFromCrsModal',
  'TempUnschedStatusModal',
  'ImportDataModal',
  'ReAuthAccountModal',
  'AccountTestModal',
  'AccountStatsModal',
  'ScheduledTestsPanel',
  'ErrorPassthroughRulesModal',
  'TLSFingerprintProfilesModal',
]

describe('AccountsView bundle boundaries', () => {
  it.each(lazyDialogs)('lazy-loads %s', (name) => {
    expect(source).toContain(`const ${name} = defineAsyncComponent(`)
  })

  it('does not eagerly import the account modal barrel', () => {
    expect(source).not.toContain("from '@/components/account'")
  })
})
