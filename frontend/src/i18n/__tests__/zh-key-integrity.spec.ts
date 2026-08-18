// @vitest-environment node

import { describe, expect, it } from 'vitest'
import zh from '../locales/zh'

function flatten(value: Record<string, unknown>, prefix = '', out = new Set<string>()): Set<string> {
  for (const [key, child] of Object.entries(value)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (child && typeof child === 'object' && !Array.isArray(child)) {
      flatten(child as Record<string, unknown>, path, out)
    } else {
      out.add(path)
    }
  }
  return out
}

const modules = import.meta.glob('../../**/*.{ts,vue}', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
const literalKeyPattern = /(?:\$t|\bt)\(\s*['"]([^'"`]+)['"]/g
const allowMissingPrefixes = [
  'auth.errors.',
  'payment.errors.',
]

describe('Chinese catalog integrity', () => {
  it('contains every statically referenced production translation key', () => {
    const catalog = flatten(zh as Record<string, unknown>)
    const missing = new Set<string>()
    const knownExistingDebt = new Set([
      'admin.accounts.fromModel',
      'admin.accounts.messages.accountCreated',
      'admin.accounts.oauth.openai.accessTokenAuth',
      'admin.accounts.oauth.openai.mobileRefreshTokenAuth',
      'admin.accounts.toModel',
      'admin.channels.emptyModelsInPricing',
      'admin.channels.noGroupsSelected',
      'admin.ops.runtime.metricThresholds',
      'admin.ops.runtime.metricThresholdsHint',
      'admin.ops.runtime.requestErrorRateMaxPercent',
      'admin.ops.runtime.requestErrorRateMaxPercentHint',
      'admin.ops.runtime.slaMinPercent',
      'admin.ops.runtime.slaMinPercentHint',
      'admin.ops.runtime.ttftP99MaxMs',
      'admin.ops.runtime.ttftP99MaxMsHint',
      'admin.ops.runtime.upstreamErrorRateMaxPercent',
      'admin.ops.runtime.upstreamErrorRateMaxPercentHint',
      'admin.users.passwordCopied',
      'common.apply',
      'common.clear',
      'common.creating',
      'common.required',
      'common.sending',
      'common.tryAgain',
    ])

    for (const [path, source] of Object.entries(modules)) {
      if (path.includes('/__tests__/') || path.endsWith('.spec.ts')) continue
      for (const match of source.matchAll(literalKeyPattern)) {
        const key = match[1]
        if (
          !key.endsWith('.') &&
          !catalog.has(key) &&
          !knownExistingDebt.has(key) &&
          !allowMissingPrefixes.some((prefix) => key.startsWith(prefix))
        ) {
          missing.add(`${key} (${path})`)
        }
      }
    }

    expect(Array.from(missing).sort()).toEqual([])
  })
})
