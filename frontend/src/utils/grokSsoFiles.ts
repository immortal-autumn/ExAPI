/**
 * Client-side parser for Grok SSO file imports.
 *
 * grok2api-style SSO files are loose by nature: some tools export a plain list
 * of tokens, others export cookie-style strings (`sso=...; Domain=...`), and
 * some export JSON / JSONL objects with a known token key. The backend
 * `normalizeSSOImportTokens` still normalizes the final strings, so this parser
 * only needs to reliably split file content into one token per element and
 * deduplicate the obvious cases.
 */

const SSO_OBJECT_KEYS = [
  'sso_token',
  'sso_cookie',
  'sso',
  'access_token',
  'token',
  'cookie',
  'ssoToken',
  'ssoCookie',
] as const

const SKIP_PLAIN_TOKENS = new Set([
  'sso',
  'sso_token',
  'sso_cookie',
  'access_token',
  'token',
  'cookie',
  'name',
  'email',
])

function trimQuotes(value: string): string {
  let result = value.trim()
  if (
    (result.startsWith('"') && result.endsWith('"')) ||
    (result.startsWith("'") && result.endsWith("'"))
  ) {
    result = result.slice(1, -1).trim()
  }
  return result
}

function extractTokenFromLine(raw: string): string {
  let line = raw.trim()
  if (!line) return ''

  // Cookie header values are commonly pasted as `Cookie: sso=...`.
  line = line.replace(/^cookie\s*:\s*/i, '')

  const cookieMatch = line.match(
    /(?:^|[\s;,])(sso(?:_token|_cookie)?|access_token|token|cookie)\s*=\s*([^;\s,]+)/i
  )
  if (cookieMatch?.[2]) {
    return trimQuotes(cookieMatch[2])
  }

  return trimQuotes(line)
}

function pushToken(tokens: string[], seen: Set<string>, raw: string): void {
  const token = extractTokenFromLine(raw)
  if (!token || seen.has(token)) return
  if (SKIP_PLAIN_TOKENS.has(token.toLowerCase())) return
  seen.add(token)
  tokens.push(token)
}

function pushStringValue(tokens: string[], seen: Set<string>, raw: string): void {
  const trimmed = raw.trim()
  if (!trimmed) return

  if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
    try {
      handleParsedValue(JSON.parse(trimmed), tokens, seen)
      return
    } catch {
      // Fall through to plain-text handling.
    }
  }

  // Cookie-like values keep their internal separators; extract the first SSO
  // token instead of splitting on spaces.
  if (/(?:sso(?:_token|_cookie)?|access_token|token|cookie)\s*=/i.test(trimmed)) {
    pushToken(tokens, seen, trimmed)
    return
  }

  const parts = trimmed.split(/[\s,]+/)
  for (const part of parts) {
    pushToken(tokens, seen, part)
  }
}

function valueFromObject(source: Record<string, unknown>): unknown {
  for (const key of SSO_OBJECT_KEYS) {
    if (key in source && source[key] !== undefined && source[key] !== null) {
      return source[key]
    }
  }
  return undefined
}

function handleParsedValue(
  parsed: unknown,
  tokens: string[],
  seen: Set<string>
): void {
  if (typeof parsed === 'string') {
    pushStringValue(tokens, seen, parsed)
    return
  }

  if (Array.isArray(parsed)) {
    for (const item of parsed) {
      handleParsedValue(item, tokens, seen)
    }
    return
  }

  if (parsed && typeof parsed === 'object') {
    const record = parsed as Record<string, unknown>
    const accounts = record.accounts
    if (Array.isArray(accounts)) {
      for (const account of accounts) {
        handleParsedValue(account, tokens, seen)
      }
      return
    }

    const directValue = valueFromObject(record)
    if (directValue !== undefined) {
      handleParsedValue(directValue, tokens, seen)
      return
    }

    // Also accept objects shaped as `{ "account1": "sso=...", ... }`.
    for (const value of Object.values(record)) {
      if (typeof value === 'string') {
        handleParsedValue(value, tokens, seen)
      }
    }
  }
}

/**
 * Parse `.txt`, `.json`, `.jsonl`, `.csv`, or `.sso` file content into a
 * deduplicated list of SSO tokens suitable for `sso_tokens`.
 */
export function parseGrokSSOFileContent(text: string): string[] {
  const tokens: string[] = []
  const seen = new Set<string>()
  const trimmed = text.trim()
  if (!trimmed) return tokens

  if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
    try {
      handleParsedValue(JSON.parse(trimmed), tokens, seen)
      return tokens
    } catch {
      // Treat the whole file as plain text below.
    }
  }

  const lines = trimmed.split(/\r?\n/)
  for (const line of lines) {
    const trimmedLine = line.trim()
    if (!trimmedLine) continue

    if (trimmedLine.startsWith('{') || trimmedLine.startsWith('[')) {
      try {
        handleParsedValue(JSON.parse(trimmedLine), tokens, seen)
        continue
      } catch {
        // Treat this line as plain text.
      }
    }

    pushStringValue(tokens, seen, trimmedLine)
  }

  return tokens
}

const BUILD_REFRESH_TOKEN_KEYS = ['refresh_token', 'refreshToken', 'rt'] as const
const SKIP_BUILD_PLAIN_TOKENS = new Set([
  'refresh_token',
  'refreshToken',
  'rt',
  'name',
  'email',
  'token',
])

function buildValueFromObject(source: Record<string, unknown>): unknown {
  for (const key of BUILD_REFRESH_TOKEN_KEYS) {
    if (key in source && source[key] !== undefined && source[key] !== null) {
      return source[key]
    }
  }
  return undefined
}

function pushBuildToken(tokens: string[], seen: Set<string>, raw: string): void {
  let token = raw.trim()
  if (
    (token.startsWith('"') && token.endsWith('"')) ||
    (token.startsWith("'") && token.endsWith("'"))
  ) {
    token = token.slice(1, -1).trim()
  }
  if (!token || seen.has(token)) return
  if (SKIP_BUILD_PLAIN_TOKENS.has(token.toLowerCase())) return
  seen.add(token)
  tokens.push(token)
}

function pushBuildStringValue(tokens: string[], seen: Set<string>, raw: string): void {
  const trimmed = raw.trim()
  if (!trimmed) return

  if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
    try {
      handleBuildParsedValue(JSON.parse(trimmed), tokens, seen)
      return
    } catch {
      // Fall through to plain-text handling.
    }
  }

  for (const part of trimmed.split(/[\s,]+/)) {
    pushBuildToken(tokens, seen, part)
  }
}

function handleBuildParsedValue(
  parsed: unknown,
  tokens: string[],
  seen: Set<string>
): void {
  if (typeof parsed === 'string') {
    pushBuildStringValue(tokens, seen, parsed)
    return
  }

  if (Array.isArray(parsed)) {
    for (const item of parsed) {
      handleBuildParsedValue(item, tokens, seen)
    }
    return
  }

  if (parsed && typeof parsed === 'object') {
    const record = parsed as Record<string, unknown>
    const accounts = record.accounts
    if (Array.isArray(accounts)) {
      for (const account of accounts) {
        handleBuildParsedValue(account, tokens, seen)
      }
      return
    }

    const directValue = buildValueFromObject(record)
    if (directValue !== undefined) {
      handleBuildParsedValue(directValue, tokens, seen)
      return
    }

    // Also accept `{ "account1": "refresh_token", ... }` maps.
    for (const value of Object.values(record)) {
      if (typeof value === 'string') {
        handleBuildParsedValue(value, tokens, seen)
      }
    }
  }
}

/**
 * Parse Grok Build credential files into refresh tokens for the existing
 * `validate-refresh-token` batch import flow.
 */
export function parseGrokBuildFileContent(text: string): string[] {
  const tokens: string[] = []
  const seen = new Set<string>()
  const trimmed = text.trim()
  if (!trimmed) return tokens

  if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
    try {
      handleBuildParsedValue(JSON.parse(trimmed), tokens, seen)
      return tokens
    } catch {
      // Treat the whole file as plain text below.
    }
  }

  const lines = trimmed.split(/\r?\n/)
  for (const line of lines) {
    const trimmedLine = line.trim()
    if (!trimmedLine) continue

    if (trimmedLine.startsWith('{') || trimmedLine.startsWith('[')) {
      try {
        handleBuildParsedValue(JSON.parse(trimmedLine), tokens, seen)
        continue
      } catch {
        // Treat this line as plain text.
      }
    }

    pushBuildStringValue(tokens, seen, trimmedLine)
  }

  return tokens
}
