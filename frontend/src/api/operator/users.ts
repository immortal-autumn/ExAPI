import { apiClient } from '../client'
import type { AdminUser, PaginatedResponse } from '@/types'

interface OperatorIdentity {
  id: number
  username: string
  email: string
  role: 'admin'
  status: 'active' | 'disabled'
  concurrency: number
}

function asAdminUser(operator: OperatorIdentity): AdminUser {
  const now = new Date(0).toISOString()
  return {
    ...operator,
    balance: 0,
    rpm_limit: 0,
    allowed_groups: null,
    balance_notify_enabled: false,
    balance_notify_threshold: null,
    balance_notify_extra_emails: [],
    created_at: now,
    updated_at: now,
    notes: '',
  }
}

async function getOperator(): Promise<AdminUser> {
  const { data } = await apiClient.get<OperatorIdentity>('/operator/me')
  return asAdminUser(data)
}

/**
 * Compatibility surface for retained per-user policy editors. Private ExAPI
 * has exactly one operator, so list/get are deliberately backed by
 * `/operator/me` and never expose the removed `/admin/users` SaaS API.
 */
export async function list(
  page = 1,
  pageSize = 20,
  filters?: { search?: string },
): Promise<PaginatedResponse<AdminUser>> {
  const operator = await getOperator()
  const query = filters?.search?.trim().toLowerCase() || ''
  const matches = !query
    || operator.email.toLowerCase().includes(query)
    || operator.username.toLowerCase().includes(query)
    || String(operator.id) === query
  const items = matches && page === 1 ? [operator] : []
  return {
    items,
    total: matches ? 1 : 0,
    page,
    page_size: pageSize,
    pages: matches ? 1 : 0,
  }
}

export async function getById(id: number, _includeDeleted = false): Promise<AdminUser> {
  const operator = await getOperator()
  if (operator.id !== id) {
    const error = new Error('Operator not found')
    ;(error as Error & { status?: number }).status = 404
    throw error
  }
  return operator
}

export default { list, getById }
