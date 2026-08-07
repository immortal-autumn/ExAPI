import { describe, expect, it } from 'vitest'

import {
  IMPORT_MAX_FILE_BYTES,
  exceedsImportObjectLimit,
  getImportFileLimitViolation,
} from '../importLimits'

function sizedFile(name: string, size: number): File {
  const file = new File(['x'], name, { type: 'application/json' })
  Object.defineProperty(file, 'size', { value: size })
  return file
}

describe('data import limits', () => {
  it('enforces count, per-file, and aggregate byte limits', () => {
    expect(getImportFileLimitViolation(Array.from({ length: 11 }, (_, index) => sizedFile(`${index}.json`, 1)))).toBe('too_many_files')
    expect(getImportFileLimitViolation([sizedFile('large.json', IMPORT_MAX_FILE_BYTES + 1)])).toBe('file_too_large')
    expect(getImportFileLimitViolation([
      sizedFile('a.json', 9 * 1024 * 1024),
      sizedFile('b.json', 9 * 1024 * 1024),
      sizedFile('c.json', 8 * 1024 * 1024),
    ])).toBe('total_too_large')
  })

  it('enforces the combined account and proxy object limit', () => {
    expect(exceedsImportObjectLimit([{ accounts: Array(4_000), proxies: Array(1_001) }])).toBe(true)
    expect(exceedsImportObjectLimit([{ accounts: Array(4_000) }, { proxies: Array(1_000) }])).toBe(false)
  })
})
