import { describe, expect, it } from 'vitest'

import { extractApiErrorMessage } from '../apiError'

describe('Chinese-only API error fallback', () => {
  it('uses a Chinese message when no API detail is available', () => {
    expect(extractApiErrorMessage(null)).toBe('未知错误')
    expect(extractApiErrorMessage({})).toBe('未知错误')
  })
})
