export interface RedactedResponseBodySummary {
  redacted: true
  format: 'json' | 'text'
  character_count: number
  upstream_status_code?: number
  top_level_type?: string
  error_type?: string
  error_code?: string
}

function pickString(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value.trim() : undefined
}

function pickErrorCode(value: unknown): string | undefined {
  if (typeof value === 'string' && value.trim()) return value.trim()
  if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  return undefined
}

export function buildRedactedResponseBodySummary(
  rawBody: string,
  options?: {
    upstreamStatusCode?: number | null
  }
): RedactedResponseBodySummary | null {
  const text = String(rawBody || '').trim()
  if (!text) return null

  const summary: RedactedResponseBodySummary = {
    redacted: true,
    format: 'text',
    character_count: text.length
  }

  if (typeof options?.upstreamStatusCode === 'number' && Number.isFinite(options.upstreamStatusCode)) {
    summary.upstream_status_code = options.upstreamStatusCode
  }

  try {
    const parsed = JSON.parse(text)
    if (parsed !== null && typeof parsed === 'object') {
      summary.format = 'json'

      if (!Array.isArray(parsed)) {
        const obj = parsed as Record<string, unknown>
        summary.top_level_type = pickString(obj.type)
        summary.error_code = pickErrorCode(obj.code) ?? summary.error_code

        const nested = obj.error
        if (nested && typeof nested === 'object' && !Array.isArray(nested)) {
          const nestedObj = nested as Record<string, unknown>
          summary.error_type = pickString(nestedObj.type)
          summary.error_code = summary.error_code ?? pickErrorCode(nestedObj.code)
        }
      }
    }
  } catch {
    summary.format = 'text'
  }

  return summary
}
