import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'
import enMisc from '../locales/en/misc'
import zhMisc from '../locales/zh/misc'
import enLanding from '../locales/en/landing'
import zhLanding from '../locales/zh/landing'

function stringify(value: unknown): string {
  return JSON.stringify(value)
}

describe('i18n brand copy', () => {
  it('does not expose Sub2API in primary onboarding/setup copy', () => {
    const text = [enMisc, zhMisc, enLanding, zhLanding].map(stringify).join('\n')
    expect(text).not.toContain('Sub2API')
    expect(text).toContain('ExAPI')
  })

  it('uses Chinese ExAPI copy in the initial HTML title', () => {
    const indexHtml = readFileSync(
      resolve(dirname(fileURLToPath(import.meta.url)), '../../../index.html'),
      'utf8',
    )
    expect(indexHtml).toContain('<title>ExAPI - AI API 网关</title>')
    expect(indexHtml).not.toContain('Sub2API')
  })
})
