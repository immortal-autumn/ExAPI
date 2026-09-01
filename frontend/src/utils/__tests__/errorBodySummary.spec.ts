import { describe, expect, it } from 'vitest'
import { buildRedactedResponseBodySummary } from '../errorBodySummary'

describe('buildRedactedResponseBodySummary', () => {
  it('keeps only bounded diagnostic metadata from JSON bodies', () => {
    const summary = buildRedactedResponseBodySummary(
      JSON.stringify({
        provider: 'xai',
        authorization: 'Bearer secret-token',
        error: { type: 'upstream_error', code: 'RESOURCE_EXHAUSTED', message: 'private body' }
      }),
      { upstreamStatusCode: 429 }
    )

    expect(summary).toEqual({
      redacted: true,
      format: 'json',
      character_count: expect.any(Number),
      upstream_status_code: 429,
      error_type: 'upstream_error',
      error_code: 'RESOURCE_EXHAUSTED'
    })
    expect(JSON.stringify(summary)).not.toContain('secret-token')
    expect(JSON.stringify(summary)).not.toContain('private body')
    expect(JSON.stringify(summary)).not.toContain('xai')
  })

  it('does not echo non-JSON provider content', () => {
    const summary = buildRedactedResponseBodySummary('provider secret body', { upstreamStatusCode: 503 })

    expect(summary).toMatchObject({
      redacted: true,
      format: 'text',
      upstream_status_code: 503
    })
    expect(JSON.stringify(summary)).not.toContain('provider secret body')
  })

  it('returns null for an empty body', () => {
    expect(buildRedactedResponseBodySummary('')).toBeNull()
  })
})
