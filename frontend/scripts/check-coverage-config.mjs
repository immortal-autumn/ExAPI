import { resolve } from 'node:path'
import { loadConfigFromFile } from 'vite'

const loaded = await loadConfigFromFile({ command: 'serve', mode: 'test' }, resolve('vitest.config.ts'))
const config = loaded?.config
const thresholds = config?.test?.coverage?.thresholds
const metrics = ['statements', 'branches', 'functions', 'lines']

if (!thresholds || typeof thresholds !== 'object') {
  throw new Error('Vitest coverage thresholds are missing')
}
if ('global' in thresholds) {
  throw new Error('Vitest v2 ignores nested coverage.thresholds.global; put metric thresholds directly under coverage.thresholds')
}
for (const metric of metrics) {
  const value = thresholds[metric]
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
    throw new Error(`Coverage threshold ${metric} must be a positive number`)
  }
}

console.log(`Coverage thresholds enforced: ${metrics.map((metric) => `${metric}=${thresholds[metric]}`).join(', ')}`)
