import { apiClient } from './client'

export interface ModelCatalogGroup {
  id: number
  name: string
  platform: string
  subscription_type: string
  rate_multiplier: number
  is_exclusive: boolean
}

export interface ModelCatalogPricing {
  billing_mode: string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_read_price: number | null
  image_output_price: number | null
  per_request_price: number | null
}

export interface ModelCatalogPriceRange {
  billing_mode: string
  min_input_price: number | null
  max_input_price: number | null
  min_output_price: number | null
  max_output_price: number | null
  min_per_request_price: number | null
  max_per_request_price: number | null
}

export interface ModelCatalogChannel {
  id: number
  name: string
  pricing?: ModelCatalogPricing | null
}

export interface ModelCatalogSource {
  account_id?: number
  account_name?: string
  platform: string
  source: string
  sync_status?: string
  sync_error?: string
  pricing?: ModelCatalogPricing | null
  channels?: ModelCatalogChannel[]
  updated_at?: string
}

export interface ModelCatalogModel {
  id: string
  display_name: string
  platform: string
  usage_modes: string[]
  source_count: number
  groups: ModelCatalogGroup[]
  price_range?: ModelCatalogPriceRange | null
  sources?: ModelCatalogSource[]
  // 监控状态（由 channel_monitors 按 primary_model 匹配聚合）
  monitor_id?: number | null
  monitor_status?: string | null
  monitor_latency_ms?: number | null
  monitor_availability_7d?: number | null
}

export interface ModelCatalogResponse {
  models: ModelCatalogModel[]
}

export interface ModelCatalogParams {
  group_id?: number | null
  view?: 'merged' | 'by_group' | 'by_channel' | 'by_account'
  signal?: AbortSignal
}

export async function getUserModelCatalog(params: ModelCatalogParams = {}): Promise<ModelCatalogResponse> {
  const { data } = await apiClient.get<ModelCatalogResponse>('/model-catalog', {
    params: {
      group_id: params.group_id || undefined,
      view: params.view || 'merged'
    },
    signal: params.signal
  })
  return data
}

export async function getAdminModelCatalog(params: ModelCatalogParams = {}): Promise<ModelCatalogResponse> {
  const { data } = await apiClient.get<ModelCatalogResponse>('/admin/model-catalog', {
    params: {
      group_id: params.group_id || undefined,
      view: params.view || 'merged'
    },
    signal: params.signal
  })
  return data
}

export default {
  getUserModelCatalog,
  getAdminModelCatalog
}
