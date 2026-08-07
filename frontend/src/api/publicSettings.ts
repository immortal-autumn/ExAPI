import { apiClient } from './client'
import type { PublicSettings } from '@/types'

/** Public, non-customer settings used to render the private operator UI. */
export async function getPublicSettings(): Promise<PublicSettings> {
  const { data } = await apiClient.get<PublicSettings>('/settings/public')
  return data
}

export function isWeChatWebOAuthEnabled(_settings: PublicSettings): boolean {
  return false
}
