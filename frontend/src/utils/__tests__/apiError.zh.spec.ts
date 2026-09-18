import { describe, expect, it } from 'vitest'

import { extractApiErrorMessage } from '../apiError'

describe('API error fallback and normalization', () => {
  it('uses the English product-default fallback and allows a localized override', () => {
    expect(extractApiErrorMessage(null)).toBe('Unknown error occurred')
    expect(extractApiErrorMessage({}, '未知错误')).toBe('未知错误')
  })

  it('extracts nested provider messages instead of rendering object coercion', () => {
    expect(extractApiErrorMessage({ error: { message: 'upstream rejected the request' } })).toBe(
      'upstream rejected the request'
    )
    expect(extractApiErrorMessage({ response: { data: { error: { detail: '读取上游失败' } } } })).toBe(
      '读取上游失败'
    )
    expect(extractApiErrorMessage({ error: { code: 'NO_MESSAGE' } }, 'fallback')).toBe('fallback')
  })
})
