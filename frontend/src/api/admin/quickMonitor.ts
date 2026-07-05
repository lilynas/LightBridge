import { apiClient } from '../client'

export interface QuickMonitorParams {
  model_id: string
  provider: string
  api_mode?: string
  endpoint: string
  api_key: string
  interval_seconds?: number
}

export async function createQuickMonitor(params: QuickMonitorParams) {
  const { data } = await apiClient.post('/admin/channel-monitors/quick', params)
  return data
}

export interface CreateFromAccountParams {
  account_id: number
  model_id: string
  interval_seconds?: number
}

export async function createMonitorFromAccount(params: CreateFromAccountParams) {
  const { data } = await apiClient.post('/admin/channel-monitors/from-account', params)
  return data
}
