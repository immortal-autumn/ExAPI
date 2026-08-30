// @vitest-environment node

import { describe, expect, it } from 'vitest'
import { parseGrokBuildFileContent, parseGrokSSOFileContent } from '../grokSsoFiles'

describe('parseGrokSSOFileContent', () => {
  it('parses plain token lists split by newlines and commas', () => {
    expect(parseGrokSSOFileContent('token-a\ntoken-b\ntoken-c')).toEqual([
      'token-a',
      'token-b',
      'token-c',
    ])
    expect(parseGrokSSOFileContent('token-a, token-b')).toEqual(['token-a', 'token-b'])
  })

  it('extracts SSO values from cookie-style lines', () => {
    const content = [
      'sso=abc123; Domain=.grok.com; Path=/',
      'Cookie: sso=def456; Secure',
      'sso_token=ghi789',
    ].join('\n')

    expect(parseGrokSSOFileContent(content)).toEqual(['abc123', 'def456', 'ghi789'])
  })

  it('parses JSON arrays of strings and objects', () => {
    expect(parseGrokSSOFileContent('["a","b"]')).toEqual(['a', 'b'])
    expect(
      parseGrokSSOFileContent('[{"sso":"a"},{"sso_token":"b"},{"sso_cookie":"c"}]')
    ).toEqual(['a', 'b', 'c'])
  })

  it('parses grok2api Web export accounts without leaking metadata', () => {
    const content = JSON.stringify({
      provider: 'grok_web',
      accounts: [
        {
          name: 'account-1',
          email: 'account-1@example.com',
          user_id: 'user-1',
          sso_token: 'sso-token-a',
          token: '',
          tier: 'basic',
        },
        {
          name: 'account-2',
          email: 'account-2@example.com',
          user_id: 'user-2',
          sso_token: 'sso-token-b',
          token: '',
          tier: 'basic',
        },
      ],
    })

    expect(parseGrokSSOFileContent(content)).toEqual(['sso-token-a', 'sso-token-b'])
  })

  it('parses a single JSON object and JSONL lines', () => {
    expect(parseGrokSSOFileContent('{"sso_token":"a"}')).toEqual(['a'])
    expect(
      parseGrokSSOFileContent('{"sso":"a"}\n{"access_token":"b"}')
    ).toEqual(['a', 'b'])
  })

  it('parses key-to-token maps and trims surrounding quotes', () => {
    expect(
      parseGrokSSOFileContent('{"account1":"sso=a","account2":"cookie=b"}')
    ).toEqual(['a', 'b'])
    expect(parseGrokSSOFileContent('"a"\n"b"')).toEqual(['a', 'b'])
  })

  it('deduplicates tokens while preserving first-seen order', () => {
    expect(parseGrokSSOFileContent('a\nb\na\nc')).toEqual(['a', 'b', 'c'])
  })

  it('ignores a single-column CSV-style header', () => {
    expect(parseGrokSSOFileContent('sso_token\nabc123')).toEqual(['abc123'])
  })

  it('returns an empty array for blank content', () => {
    expect(parseGrokSSOFileContent('')).toEqual([])
    expect(parseGrokSSOFileContent('  \n\t ')).toEqual([])
  })
})

describe('parseGrokBuildFileContent', () => {
  it('parses grok2api Build export accounts into refresh tokens', () => {
    const content = JSON.stringify({
      provider: 'build',
      accounts: [
        {
          provider: 'grok_build',
          name: 'account-1',
          access_token: 'access-a',
          refresh_token: 'refresh-a',
        },
        {
          provider: 'grok_build',
          name: 'account-2',
          access_token: 'access-b',
          refresh_token: 'refresh-b',
        },
      ],
    })

    expect(parseGrokBuildFileContent(content)).toEqual(['refresh-a', 'refresh-b'])
  })

  it('parses plain refresh-token lists and deduplicates', () => {
    expect(parseGrokBuildFileContent('refresh-a\nrefresh-b\nrefresh-a')).toEqual([
      'refresh-a',
      'refresh-b',
    ])
  })
})
