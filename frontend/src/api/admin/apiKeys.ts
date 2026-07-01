/**
 * Admin API Keys API endpoints
 * Handles API key management for administrators
 */

import { apiClient } from '../client'
import type { ApiKey, User } from '@/types'

export interface UpdateApiKeyGroupResult {
  api_key: ApiKey
  auto_granted_group_access: boolean
  granted_group_id?: number
  granted_group_name?: string
}

export interface ApiKeyOwnerLookupResult {
  api_key: ApiKey
  user: User
}

/**
 * Find the owner of an API key.
 * @param key - Full API key value
 * @returns API key and owning user
 */
export async function lookupOwner(key: string): Promise<ApiKeyOwnerLookupResult> {
  const { data } = await apiClient.post<ApiKeyOwnerLookupResult>('/admin/api-keys/lookup', { key })
  return data
}

/**
 * Update an API key's group binding
 * @param id - API Key ID
 * @param groupId - Group ID (0 to unbind, positive to bind, null/undefined to skip)
 * @returns Updated API key with auto-grant info
 */
export async function updateApiKeyGroup(id: number, groupId: number | null): Promise<UpdateApiKeyGroupResult> {
  const { data } = await apiClient.put<UpdateApiKeyGroupResult>(`/admin/api-keys/${id}`, {
    group_id: groupId === null ? 0 : groupId
  })
  return data
}

export const apiKeysAPI = {
  lookupOwner,
  updateApiKeyGroup
}

export default apiKeysAPI
