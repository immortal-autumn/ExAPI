import { describe, expect, it } from 'vitest'

import { sanitizeHtml, sanitizeSvg } from '../sanitize'

describe('HTML sanitization', () => {
  it('removes executable markup while retaining safe formatting', () => {
    const sanitized = sanitizeHtml('<p onclick="alert(1)">Safe <strong>text</strong></p><script>alert(1)</script>')

    expect(sanitized).toContain('<p>Safe <strong>text</strong></p>')
    expect(sanitized).not.toContain('onclick')
    expect(sanitized).not.toContain('<script')
  })

  it('removes SVG scripts, event handlers, and javascript links', () => {
    const sanitized = sanitizeSvg(`
      <svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)">
        <script>alert(1)</script>
        <a href="javascript:alert(1)"><path d="M0 0h1v1z" /></a>
      </svg>
    `)

    expect(sanitized).toContain('<svg')
    expect(sanitized).toContain('<path')
    expect(sanitized).not.toContain('onload')
    expect(sanitized).not.toContain('<script')
    expect(sanitized).not.toContain('javascript:')
  })
})
