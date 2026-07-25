import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(process.cwd(), 'src/views/auth/LoginView.vue'), 'utf8')

describe('private LoginView product boundary', () => {
  it('suppresses dormant customer auth surfaces and tokens', () => {
    expect(source).toContain('const privateProduct = isSingleUserPrivateControlPlaneBrowser()')
    expect(source).toContain('v-if="!privateProduct && passwordResetEnabled && !backendModeEnabled"')
    expect(source).toContain('v-if="!privateProduct && turnstileEnabled && turnstileSiteKey"')
    expect(source).toContain('<template v-if="!privateProduct && !backendModeEnabled" #footer>')
    expect(source).toContain('turnstile_token: !privateProduct && turnstileEnabled.value')
    expect(source).toContain('v-if="!privateProduct && loginAgreementEnabled"')
    expect(source).toContain('if (privateProduct) {')
    expect(source).toContain('agreementAccepted.value = true')
  })

  it('uses the registered administrator dashboard when no safe redirect is supplied', () => {
    expect(source).toContain('singleUserPostLoginRedirect(requested)')
  })
})
