#!/usr/bin/env node

import { readdirSync, statSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(fileURLToPath(new URL('.', import.meta.url)), '../../backend/internal/web/dist/assets')
const args = new Map(process.argv.slice(2).map((arg) => {
  const [key, value] = arg.split('=')
  return [key, Number(value)]
}))

const budgets = [
  { label: 'AccountsView', prefix: 'AccountsView-', maxKB: args.get('--accounts-kb') ?? 180 },
  { label: 'GroupsView', prefix: 'GroupsView-', maxKB: args.get('--groups-kb') ?? 210 },
  { label: 'PrivateSettingsView', prefix: 'PrivateSettingsView-', maxKB: args.get('--settings-kb') ?? 210 },
  { label: 'OpsDashboard', prefix: 'OpsDashboard-', maxKB: args.get('--ops-kb') ?? 230 },
]

const files = readdirSync(root).filter((name) => name.endsWith('.js'))
let failed = false

for (const budget of budgets) {
  const matches = files.filter((name) => name.startsWith(budget.prefix))
  if (matches.length !== 1) {
    console.error(`Expected exactly one ${budget.label} chunk, found ${matches.length}`)
    failed = true
    continue
  }

  const bytes = statSync(resolve(root, matches[0])).size
  const kb = bytes / 1024
  console.log(`${budget.label}: ${kb.toFixed(2)} KB / ${budget.maxKB} KB`)
  if (kb > budget.maxKB) {
    console.error(`${budget.label} exceeds budget by ${(kb - budget.maxKB).toFixed(2)} KB`)
    failed = true
  }
}

// Account dialogs are loaded on demand from AccountsView. Track the largest
// modal chunk independently so adding a new provider flow cannot silently
// inflate the account-management interaction budget.
const accountModalFiles = files.filter((name) => /^(?:Account(?:Stats|Test)Modal|BulkEditAccountModal|CreateAccountModal|EditAccountModal|ImportDataModal|ReAuthAccountModal|SyncFromCrsModal|TempUnschedStatusModal)-.*\.js$/.test(name))
const accountModalBudgetKB = args.get('--account-modal-kb') ?? 180
if (accountModalFiles.length === 0) {
  console.error('Expected at least one account modal chunk')
  failed = true
} else {
  const largestAccountModal = accountModalFiles.reduce((largest, name) => {
    const bytes = statSync(resolve(root, name)).size
    return bytes > largest.bytes ? { name, bytes } : largest
  }, { name: '', bytes: 0 })
  const kb = largestAccountModal.bytes / 1024
  console.log(`Account modal (largest ${largestAccountModal.name}): ${kb.toFixed(2)} KB / ${accountModalBudgetKB} KB`)
  if (kb > accountModalBudgetKB) {
    console.error(`Account modal exceeds budget by ${(kb - accountModalBudgetKB).toFixed(2)} KB`)
    failed = true
  }
}

const totalBudgetKB = args.get('--total-kb') ?? 3450
const totalKB = files.reduce((sum, name) => sum + statSync(resolve(root, name)).size, 0) / 1024
console.log(`Total JavaScript assets: ${totalKB.toFixed(2)} KB / ${totalBudgetKB} KB (${files.length} files)`)
if (totalKB > totalBudgetKB) {
  console.error(`Total JavaScript assets exceed budget by ${(totalKB - totalBudgetKB).toFixed(2)} KB`)
  failed = true
}

if (failed) process.exit(1)
