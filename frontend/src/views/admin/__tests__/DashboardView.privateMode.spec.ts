// @vitest-environment node

import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../DashboardView.vue')
const source = readFileSync(componentPath, 'utf8')

describe('DashboardView private product boundary', () => {
  it('renders only the cockpit and skips SaaS analytics loaders in private mode', () => {
    expect(source).toContain('v-if="privateGatewayControlPlane"')
    expect(source).toContain('if (privateGatewayControlPlane) return')
    expect(source).toContain('isSingleUserPrivateControlPlaneBrowser')
  })
})