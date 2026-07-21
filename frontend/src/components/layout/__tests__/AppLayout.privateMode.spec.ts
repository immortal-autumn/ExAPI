// @vitest-environment node

import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppLayout.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('AppLayout private product onboarding boundary', () => {
  it('does not auto-start or register the SaaS onboarding tour', () => {
    expect(componentSource).toContain('autoStart: !privateGatewayControlPlane')
    expect(componentSource).toContain('if (!privateGatewayControlPlane)')
    expect(componentSource).toContain('onboardingStore.setReplayCallback(replayTour)')
  })
})
