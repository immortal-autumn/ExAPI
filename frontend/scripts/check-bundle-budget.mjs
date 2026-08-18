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

if (failed) process.exit(1)
