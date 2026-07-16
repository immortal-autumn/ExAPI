import { describe, expect, it } from 'vitest'

import { formatChineseDateTime, formatChineseNumber } from '../zhPresentation'

describe('Chinese-only presentation formatting', () => {
  it('formats numbers with the canonical Chinese locale', () => {
    expect(formatChineseNumber(1234567.89)).toBe(new Intl.NumberFormat('zh-CN').format(1234567.89))
  })

  it('formats date-time values with the canonical Chinese locale', () => {
    const value = new Date('2026-07-14T12:34:56Z')
    expect(formatChineseDateTime(value)).toBe(new Intl.DateTimeFormat('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    }).format(value))
  })
})
