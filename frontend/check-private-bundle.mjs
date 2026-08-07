#!/usr/bin/env node

import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs'
import { basename, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const projectRoot = resolve(fileURLToPath(new URL('.', import.meta.url)))
const options = new Map(process.argv.slice(2).map((argument) => {
  const separator = argument.indexOf('=')
  return separator < 0
    ? [argument, true]
    : [argument.slice(0, separator), argument.slice(separator + 1)]
}))
const outputRoot = resolve(projectRoot, String(options.get('--dir') || '../backend/internal/web/dist'))
const assetsRoot = resolve(outputRoot, 'assets')
const entryBudgetKB = Number(options.get('--entry-kb') || 625)
const settingsBudgetKB = Number(options.get('--settings-kb') || 850)
const dashboardBudgetKB = Number(options.get('--dashboard-kb') || 725)

if (!existsSync(resolve(outputRoot, 'index.html')) || !existsSync(assetsRoot)) {
  throw new Error(`Built frontend not found under ${outputRoot}`)
}

const html = readFileSync(resolve(outputRoot, 'index.html'), 'utf8')
const entryMatch = html.match(/<script[^>]+src=["'](?:\.\/|\/)?assets\/([^"']+\.js)["']/i)
if (!entryMatch) throw new Error('Unable to identify the module entry in index.html')

const jsFiles = readdirSync(assetsRoot).filter((name) => name.endsWith('.js'))
const forbiddenArtifactPattern = /(payment|stripe|airwallex|affiliate|subscription|redeem|captcha|passkey|totp|loginview|registerview)/i
const forbiddenArtifacts = jsFiles.filter((name) => forbiddenArtifactPattern.test(name))
if (forbiddenArtifacts.length > 0) {
  throw new Error(`Private build emitted forbidden commercial/auth chunks: ${forbiddenArtifacts.join(', ')}`)
}
const settingsFiles = jsFiles.filter((name) => /^SettingsView-.*\.js$/.test(name))
if (settingsFiles.length !== 1) {
  throw new Error(`Expected one SettingsView chunk, found ${settingsFiles.length}`)
}
const dashboardFiles = jsFiles.filter((name) => /^DashboardView-.*\.js$/.test(name))
if (dashboardFiles.length !== 1) {
  throw new Error(`Expected one DashboardView chunk, found ${dashboardFiles.length}`)
}

function staticImports(fileName) {
  const source = readFileSync(resolve(assetsRoot, fileName), 'utf8')
  const imports = new Set()
  const pattern = /\b(?:from|import)\s*["']\.\/([^"']+\.js)["']/g
  for (const match of source.matchAll(pattern)) imports.add(basename(match[1]))
  return imports
}

function staticGraph(rootFile) {
  const visited = new Set()
  const pending = [rootFile]
  while (pending.length > 0) {
    const current = pending.pop()
    if (!current || visited.has(current)) continue
    if (!existsSync(resolve(assetsRoot, current))) {
      throw new Error(`Missing statically imported chunk ${current}`)
    }
    visited.add(current)
    pending.push(...staticImports(current))
  }
  return visited
}

function graphSizeKB(graph) {
  const bytes = [...graph].reduce((sum, name) => sum + statSync(resolve(assetsRoot, name)).size, 0)
  return bytes / 1024
}

function assertPrivateGraph(label, graph, budgetKB, forbiddenPattern = /(stripe|airwallex|payment)/i) {
  const forbidden = [...graph].filter((name) => forbiddenPattern.test(name))
  if (forbidden.length > 0) {
    throw new Error(`${label} statically loads forbidden payment chunks: ${forbidden.join(', ')}`)
  }
  const sizeKB = graphSizeKB(graph)
  console.log(`${label}: ${sizeKB.toFixed(2)} KB / ${budgetKB.toFixed(2)} KB (${graph.size} static chunks)`)
  if (sizeKB > budgetKB) {
    throw new Error(`${label} exceeds its private bundle budget by ${(sizeKB - budgetKB).toFixed(2)} KB`)
  }
}

assertPrivateGraph('Private initial graph', staticGraph(entryMatch[1]), entryBudgetKB)
assertPrivateGraph('Private Settings graph', staticGraph(settingsFiles[0]), settingsBudgetKB)
assertPrivateGraph(
  'Private Dashboard graph',
  staticGraph(dashboardFiles[0]),
  dashboardBudgetKB,
  /(stripe|airwallex|payment|chart)/i,
)
