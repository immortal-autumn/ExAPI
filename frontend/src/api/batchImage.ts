import { buildApiUrl } from './client'

export type BatchImageStatus =
  | 'queued'
  | 'running'
  | 'indexing'
  | 'processing_results'
  | 'settling'
  | 'completed'
  | 'failed'
  | 'cancelled'
  | 'output_deleted'
  | string

export interface BatchImageSubmitItem {
  custom_id: string
  prompt: string
  output_count?: number
  reference_images?: BatchImageReferenceImage[]
}

export interface BatchImageReferenceImage {
  id?: string
  type?: string
  mime_type: string
  data?: string
  file_uri?: string
}

export interface BatchImageSubmitRequest {
  model: string
  task_name?: string
  parent_batch_id?: string
  provider?: '' | 'gemini_api' | 'vertex' | string
  image_size?: '1K' | '2K' | '4K' | string
  response_mime_type?: string
  aspect_ratio?: string
  items: BatchImageSubmitItem[]
  metadata?: Record<string, string>
}

export interface BatchImageJob {
  id: string
  object: string
  task_name: string
  parent_batch_id?: string | null
  status: BatchImageStatus
  model: string
  provider: string
  item_count: number
  success_count: number
  fail_count: number
  estimated_cost: number
  hold_amount: number
  actual_cost: number | null
  created_at: number
  submitted_at: number | null
  settled_at: number | null
  downloaded_at?: number | null
  output_deleted_at?: number | null
}

export interface BatchImageItem {
  batch_id?: string
  source_task_name?: string
  custom_id: string
  status: string
  prompt_preview?: string | null
  mime_type: string | null
  file_extension: string | null
  image_count: number
  error?: {
    code: string
    message: string
    source?: 'provider' | 'system' | string
  } | null
}

export interface BatchImageItemsResponse {
  object: string
  data: BatchImageItem[]
  has_more: boolean
}

export interface BatchImageJobsResponse {
  object: string
  data: BatchImageJob[]
  has_more: boolean
}

export interface BatchImageModel {
  id: string
  object: string
  provider: string
}

export interface BatchImageModelsResponse {
  object: string
  data: BatchImageModel[]
}

export interface BatchImageJobsListOptions {
  limit?: number
  cursor?: string
  status?: string
  taskName?: string
  downloaded?: '' | 'true' | 'false' | string
  from?: string
  to?: string
}

async function parseBatchImageError(response: Response): Promise<Error> {
  try {
    const body = await response.json()
    const message = body?.error?.message || body?.message || response.statusText
    const error = new Error(message)
    ;(error as any).code = body?.error?.code || response.status
    ;(error as any).status = response.status
    ;(error as any).requestId = response.headers.get('X-Request-Id') || ''
    return error
  } catch {
    const error = new Error(response.statusText || `HTTP ${response.status}`)
    ;(error as any).code = response.status
    ;(error as any).status = response.status
    ;(error as any).requestId = response.headers.get('X-Request-Id') || ''
    return error
  }
}

function operatorHeaders(extra?: HeadersInit): HeadersInit {
  return {
    'X-ExAPI-Control-Request': '1',
    ...extra,
  }
}

function operatorBatchImageUrl(apiKeyID: number, path: string): string {
  const separator = path.includes('?') ? '&' : '?'
  return `${buildApiUrl(`/operator/batch-images${path}`)}${separator}api_key_id=${encodeURIComponent(String(apiKeyID))}`
}

export async function submitBatchImageJob(
  apiKeyID: number,
  payload: BatchImageSubmitRequest,
  idempotencyKey: string,
): Promise<BatchImageJob> {
  const response = await fetch(operatorBatchImageUrl(apiKeyID, ''), {
    method: 'POST',
    headers: operatorHeaders({
      'Content-Type': 'application/json',
      'Idempotency-Key': idempotencyKey,
    }),
    body: JSON.stringify(payload),
  })
  if (!response.ok) throw await parseBatchImageError(response)
  return response.json()
}

export async function getBatchImageJob(apiKeyID: number, batchId: string): Promise<BatchImageJob> {
  const response = await fetch(operatorBatchImageUrl(apiKeyID, `/${encodeURIComponent(batchId)}`), {
    headers: operatorHeaders(),
  })
  if (!response.ok) throw await parseBatchImageError(response)
  return response.json()
}

export async function listBatchImageJobs(apiKeyID: number, options: number | BatchImageJobsListOptions = 20): Promise<BatchImageJobsResponse> {
  const params = new URLSearchParams()
  if (typeof options === 'number') {
    params.set('limit', String(options))
  } else {
    params.set('limit', String(options.limit || 20))
    if (options.cursor) params.set('cursor', options.cursor)
    if (options.status) params.set('status', options.status)
    if (options.taskName) params.set('task_name', options.taskName)
    if (options.downloaded) params.set('downloaded', options.downloaded)
    if (options.from) params.set('from', options.from)
    if (options.to) params.set('to', options.to)
  }
  const response = await fetch(operatorBatchImageUrl(apiKeyID, `?${params.toString()}`), {
    headers: operatorHeaders(),
  })
  if (!response.ok) throw await parseBatchImageError(response)
  return response.json()
}

export async function listBatchImageModels(apiKeyID: number): Promise<BatchImageModelsResponse> {
  const response = await fetch(operatorBatchImageUrl(apiKeyID, '/models'), {
    headers: operatorHeaders(),
  })
  if (!response.ok) throw await parseBatchImageError(response)
  return response.json()
}

export async function listBatchImageItems(
  apiKeyID: number,
  batchId: string,
  status = '',
): Promise<BatchImageItemsResponse> {
  const query = status ? `?status=${encodeURIComponent(status)}` : ''
  const response = await fetch(operatorBatchImageUrl(apiKeyID, `/${encodeURIComponent(batchId)}/items${query}`), {
    headers: operatorHeaders(),
  })
  if (!response.ok) throw await parseBatchImageError(response)
  return response.json()
}

export async function cancelBatchImageJob(apiKeyID: number, batchId: string): Promise<BatchImageJob> {
  const response = await fetch(operatorBatchImageUrl(apiKeyID, `/${encodeURIComponent(batchId)}/cancel`), {
    method: 'POST',
    headers: operatorHeaders(),
  })
  if (!response.ok) throw await parseBatchImageError(response)
  return response.json()
}

export async function downloadBatchImageZip(apiKeyID: number, batchId: string): Promise<Blob> {
  const response = await fetch(operatorBatchImageUrl(apiKeyID, `/${encodeURIComponent(batchId)}/download`), {
    headers: operatorHeaders(),
  })
  if (!response.ok) throw await parseBatchImageError(response)
  return response.blob()
}

export async function getBatchImageItemContent(apiKeyID: number, batchId: string, customId: string, imageIndex = 0): Promise<Blob> {
  const response = await fetch(operatorBatchImageUrl(apiKeyID, `/${encodeURIComponent(batchId)}/items/${encodeURIComponent(customId)}/content?image_index=${encodeURIComponent(String(imageIndex))}`), {
    headers: operatorHeaders(),
  })
  if (!response.ok) throw await parseBatchImageError(response)
  return response.blob()
}

export async function deleteBatchImageJobRecord(apiKeyID: number, batchId: string): Promise<void> {
  const response = await fetch(operatorBatchImageUrl(apiKeyID, `/${encodeURIComponent(batchId)}`), {
    method: 'DELETE',
    headers: operatorHeaders(),
  })
  if (!response.ok) throw await parseBatchImageError(response)
}

export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}
