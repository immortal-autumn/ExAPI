// @vitest-environment node

import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppLayout.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('AppLayout private product onboarding boundary', () => {
  it('does not load or register the SaaS onboarding tour', () => {
    expect(componentSource).not.toContain('useOnboardingTour')
    expect(componentSource).not.toContain('useOnboardingStore')
    expect(componentSource).not.toContain('onboarding.css')
    expect(componentSource).not.toContain('setReplayCallback')
    expect(componentSource).not.toContain('autoStart')
  })
})
