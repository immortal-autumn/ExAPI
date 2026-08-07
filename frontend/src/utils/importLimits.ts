export const IMPORT_MAX_FILES = 10
export const IMPORT_MAX_FILE_BYTES = 10 * 1024 * 1024
export const IMPORT_MAX_TOTAL_BYTES = 25 * 1024 * 1024
export const IMPORT_MAX_OBJECTS = 5_000

export type ImportLimitViolation =
  | 'too_many_files'
  | 'file_too_large'
  | 'total_too_large'
  | 'too_many_objects'

export function getImportFileLimitViolation(files: readonly File[]): ImportLimitViolation | null {
  if (files.length > IMPORT_MAX_FILES) return 'too_many_files'
  if (files.some((file) => file.size > IMPORT_MAX_FILE_BYTES)) return 'file_too_large'
  const total = files.reduce((sum, file) => sum + file.size, 0)
  if (total > IMPORT_MAX_TOTAL_BYTES) return 'total_too_large'
  return null
}

export function getImportObjectCount(payload: unknown): number {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) return 0
  const record = payload as Record<string, unknown>
  const proxies = Array.isArray(record.proxies) ? record.proxies.length : 0
  const accounts = Array.isArray(record.accounts) ? record.accounts.length : 0
  return proxies + accounts
}

export function exceedsImportObjectLimit(payloads: readonly unknown[]): boolean {
  return payloads.reduce<number>((sum, payload) => sum + getImportObjectCount(payload), 0) > IMPORT_MAX_OBJECTS
}
