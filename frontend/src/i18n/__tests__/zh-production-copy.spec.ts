// @vitest-environment node

import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'

const productionFiles = [
  new URL('../../views/admin/components/SingleUserCockpitPanel.vue', import.meta.url),
  new URL('../../views/user/CustomPageView.vue', import.meta.url),
  new URL('../../api/client.ts', import.meta.url),
]

const forbiddenEnglish = [
  'Single-user cockpit',
  'Failed to load account summary.',
  'Clipboard copy failed.',
  'Page not found',
  'Failed to load page',
  'Session expired. Please log in again.',
  'Network error. Please check your connection.',
]

describe('Chinese-only production copy guard', () => {
  it('does not reintroduce confirmed English user-facing fallbacks', () => {
    const source = productionFiles.map((url) => readFileSync(url, 'utf8')).join('\n')
    for (const copy of forbiddenEnglish) {
      expect(source, copy).not.toContain(copy)
    }
  })
})
