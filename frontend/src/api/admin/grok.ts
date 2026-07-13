/**
 * Admin Grok API endpoints
 * Handles xAI Grok OAuth and quota probes for administrators.
 */

import { apiClient } from '../client'
import type { Account, GrokQuotaWindow } from '@/types'

export type GrokOAuthMode = 'build_proxy' | 'official_api'
export type GrokTokenCapability = 'unknown' | 'grok_build' | 'official_api' | 'incompatible'

export interface GrokAuthUrlResponse {
  auth_url: string
  session_id: string
  state: string
  oauth_mode: GrokOAuthMode
}

export interface GrokAuthUrlRequest {
  proxy_id?: number
  redirect_uri?: string
  oauth_mode?: GrokOAuthMode
}

export interface GrokExchangeCodeRequest {
  session_id: string
  code: string
  state?: string
  redirect_uri?: string
  proxy_id?: number
}

export interface GrokTokenInfo {
  access_token?: string
  refresh_token?: string
  id_token?: string
  client_id?: string
  token_type?: string
  expires_in?: number
  expires_at?: number | string
  scope?: string
  email?: string
  name?: string
  base_url?: string
  auth_kind?: string
  using_api?: boolean
  subscription_tier?: string
  entitlement_status?: string
  oauth_mode?: GrokOAuthMode
  token_capability?: GrokTokenCapability
  token_referrer?: string
  [key: string]: unknown
}

export interface GrokRefreshTokenRequest {
  refresh_token?: string
  rt?: string
  client_id?: string
  proxy_id?: number
  oauth_mode?: GrokOAuthMode
}

export interface GrokRuntimeSanity {
  [key: string]: unknown
}

export interface GrokQuotaSnapshot {
  requests?: GrokQuotaWindow | null
  tokens?: GrokQuotaWindow | null
  retry_after_seconds?: number | null
  subscription_tier?: string
  entitlement_status?: string
  status_code?: number
  headers?: Record<string, string>
  headers_observed?: boolean
  observation_source?: string
  last_probe_at?: string
  last_headers_seen_at?: string
  updated_at?: string
}

export interface GrokQuotaProbeResult {
  source: string
  snapshot?: GrokQuotaSnapshot | null
  status_code?: number
  headers_observed: boolean
  reset_supported: boolean
  fetched_at: number
}

export interface GrokQuotaResetResult {
  supported: boolean
  code: string
  message: string
}

export async function generateAuthUrl(payload: GrokAuthUrlRequest): Promise<GrokAuthUrlResponse> {
  const { data } = await apiClient.post<GrokAuthUrlResponse>(
    '/admin/grok/oauth/auth-url',
    payload
  )
  return data
}

export async function exchangeCode(payload: GrokExchangeCodeRequest): Promise<GrokTokenInfo> {
  const { data } = await apiClient.post<GrokTokenInfo>(
    '/admin/grok/oauth/exchange-code',
    payload
  )
  return data
}

export async function refreshToken(payload: GrokRefreshTokenRequest): Promise<GrokTokenInfo> {
  const { data } = await apiClient.post<GrokTokenInfo>(
    '/admin/grok/oauth/refresh-token',
    payload
  )
  return data
}

export async function refreshAccountToken(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/grok/accounts/${id}/refresh`)
  return data
}

export async function queryQuota(id: number): Promise<GrokQuotaProbeResult> {
  const { data } = await apiClient.post<GrokQuotaProbeResult>(`/admin/grok/accounts/${id}/quota`)
  return data
}

export async function resetQuota(id: number): Promise<GrokQuotaResetResult> {
  const { data } = await apiClient.post<GrokQuotaResetResult>(`/admin/grok/accounts/${id}/reset-quota`)
  return data
}

export async function runtimeSanity(): Promise<GrokRuntimeSanity> {
  const { data } = await apiClient.get<GrokRuntimeSanity>('/admin/grok/runtime-sanity')
  return data
}

export default {
  generateAuthUrl,
  exchangeCode,
  refreshToken,
  refreshAccountToken,
  queryQuota,
  resetQuota,
  runtimeSanity
}
