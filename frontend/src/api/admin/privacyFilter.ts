import { apiClient } from '../client'

export type PrivacyFilterModelFilterType = 'all' | 'include' | 'exclude'

export interface PrivacyFilterModelFilter {
  type: PrivacyFilterModelFilterType
  models: string[]
}

export interface PrivacyFilterRule {
  name: string
  pattern: string
  replacement: string
  enabled: boolean
}

export type PrivacyFilterTargetScope = 'all_users' | 'partial_users' | 'admin_only'
export type PrivacyFilterChannelScope = 'all' | 'group' | 'channel' | 'account'

export interface PrivacyFilterConfig {
  enabled: boolean
  filter_request: boolean
  filter_response: boolean
  builtin_rules: Record<string, boolean>
  builtin_rule_ids: string[]
  custom_rules: PrivacyFilterRule[]
  all_groups: boolean
  group_ids: number[]
  model_filter: PrivacyFilterModelFilter
  target_scope: PrivacyFilterTargetScope
  target_user_ids: number[]
  channel_scope: PrivacyFilterChannelScope
  channel_ids: number[]
  account_ids: number[]
}

export interface UpdatePrivacyFilterConfig {
  enabled?: boolean
  filter_request?: boolean
  filter_response?: boolean
  builtin_rules?: Record<string, boolean>
  custom_rules?: PrivacyFilterRule[]
  all_groups?: boolean
  group_ids?: number[]
  model_filter?: PrivacyFilterModelFilter
  target_scope?: PrivacyFilterTargetScope
  target_user_ids?: number[]
  channel_scope?: PrivacyFilterChannelScope
  channel_ids?: number[]
  account_ids?: number[]
}

export async function getConfig(): Promise<PrivacyFilterConfig> {
  const { data } = await apiClient.get<PrivacyFilterConfig>('/admin/privacy-filter/config')
  return data
}

export async function updateConfig(
  payload: UpdatePrivacyFilterConfig
): Promise<PrivacyFilterConfig> {
  const { data } = await apiClient.put<PrivacyFilterConfig>('/admin/privacy-filter/config', payload)
  return data
}

export const privacyFilterAPI = {
  getConfig,
  updateConfig,
}

export default privacyFilterAPI
