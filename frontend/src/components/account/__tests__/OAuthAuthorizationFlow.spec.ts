// @vitest-environment jsdom

import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copied: { value: false },
    copyToClipboard: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

import OAuthAuthorizationFlow from '../OAuthAuthorizationFlow.vue'

const source = readFileSync(
  resolve(process.cwd(), 'src/components/account/OAuthAuthorizationFlow.vue'),
  'utf8'
)

describe('OAuthAuthorizationFlow Grok SSO file import', () => {
  it('provides an SSO file upload control inside the Grok SSO method', () => {
    expect(source).toContain('data-testid="grok-sso-files-upload"')
    expect(source).toContain('type="file"')
    expect(source).toContain('accept=".txt,.json,.jsonl,.csv,.sso,text/plain,application/json"')
    expect(source).toContain('parseGrokSSOFileContent')
    expect(source).toContain("getOAuthKey('ssoFilesSelect')")
  })

  it('labels Grok manual authorization as Build device OAuth', () => {
    expect(source).toContain('admin.accounts.oauth.grok.buildAuth')
    expect(source).toContain('manualAuthLabel')
  })

  it('parses an uploaded SSO file into the textarea and emits import-sso on submit', async () => {
    const wrapper = mount(OAuthAuthorizationFlow, {
      props: {
        addMethod: 'oauth',
        platform: 'grok',
        showSsoOption: true,
        showManualOption: false,
        showCookieOption: false,
        initialInputMethod: 'sso_cookie',
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const fileInput = wrapper.get('input[type="file"]')
    const file = {
      name: 'tokens.txt',
      text: vi.fn().mockResolvedValue('sso=abc\nCookie: sso=def'),
    } as unknown as File

    Object.defineProperty(fileInput.element, 'files', {
      configurable: true,
      value: [file],
    })
    await fileInput.trigger('change')
    await flushPromises()

    const textarea = wrapper.get('textarea')
    expect((textarea.element as HTMLTextAreaElement).value).toContain('abc')
    expect((textarea.element as HTMLTextAreaElement).value).toContain('def')

    await wrapper.get('button.btn-primary').trigger('click')
    expect(wrapper.emitted('import-sso')).toEqual([['abc\ndef']])
  })

  it('parses a Grok Build export into the refresh-token textarea and emits validate-refresh-token', async () => {
    const wrapper = mount(OAuthAuthorizationFlow, {
      props: {
        addMethod: 'oauth',
        platform: 'grok',
        showRefreshTokenOption: true,
        showManualOption: false,
        showCookieOption: false,
        initialInputMethod: 'refresh_token',
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const fileInput = wrapper.get('input[type="file"]')
    const file = {
      name: 'grok-Build-20accounts.json',
      text: vi.fn().mockResolvedValue(
        JSON.stringify({
          provider: 'build',
          accounts: [
            { refresh_token: 'refresh-a', access_token: 'access-a' },
            { refresh_token: 'refresh-b', access_token: 'access-b' },
          ],
        })
      ),
    } as unknown as File

    Object.defineProperty(fileInput.element, 'files', {
      configurable: true,
      value: [file],
    })
    await fileInput.trigger('change')
    await flushPromises()

    const textarea = wrapper.get('textarea')
    expect((textarea.element as HTMLTextAreaElement).value).toContain('refresh-a')
    expect((textarea.element as HTMLTextAreaElement).value).toContain('refresh-b')
    expect((textarea.element as HTMLTextAreaElement).value).not.toContain('access-a')

    await wrapper.get('button.btn-primary').trigger('click')
    expect(wrapper.emitted('validate-refresh-token')).toEqual([['refresh-a\nrefresh-b']])
  })
})
