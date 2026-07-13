/**
 * Authconv integration for LightBridge
 * Provides format detection, normalization, rendering, and conversion between
 * authconv formats and LightBridge's AdminDataPayload/AdminDataAccount format.
 *
 * Based on https://github.com/ltxgit/authconv (MIT License)
 */

import type { AdminDataAccount, AdminDataPayload } from '@/types'
import { detectInputFormat, normalizeInput } from './normalize'
import { renderFormat } from './renderers'
import type {
  InputFormat,
  NormalizedAccount,
  OutputFormat,
  RenderOptions
} from './types'

export { detectInputFormat, normalizeInput } from './normalize'
export { renderFormat } from './renderers'
export type { AdminDataPayload } from '@/types'
export type {
  InputFormat,
  NormalizedAccount,
  OutputFormat,
  RenderOptions,
  CpaCodexRenderedAccount,
  CpaXaiRenderedAccount,
  CpaRenderedAccount,
  Codex2ApiRenderedAccount,
  Sub2ApiRenderedData,
  CodexManagerRenderedAccount,
  CodexRenderedAuth
} from './types'
export { ALL_FORMATS } from './types'

const FORMAT_LABELS: Record<OutputFormat, string> = {
  cpa: 'CPA',
  sub2api: 'sub2api',
  codex2api: 'codex2api',
  codexmanager: 'Codex Manager',
  codex: 'Codex Auth'
}

const INPUT_FORMAT_LABELS: Record<InputFormat, string> = {
  session: 'ChatGPT Session',
  sub2api: 'sub2api',
  cpa: 'CPA',
  codexmanager: 'Codex Manager',
  codex2api: 'Codex2Api',
  codex: 'Codex Auth',
  unknown: 'Unknown'
}

export function getFormatLabel(format: OutputFormat): string {
  return FORMAT_LABELS[format]
}

export function getInputFormatLabel(format: InputFormat): string {
  return INPUT_FORMAT_LABELS[format]
}

/**
 * Detect the input format of a parsed JSON value.
 */
export function detectFormat(input: unknown): InputFormat {
  return detectInputFormat(input)
}

/**
 * Convert any recognized authconv input format into LightBridge's AdminDataPayload.
 * Returns null if the input is not a recognized credential format.
 */
export function convertToPayload(
  input: unknown,
  options?: {
    concurrency?: number
    priority?: number
    rate_multiplier?: number
    auto_pause_on_expired?: boolean
    inputFormat?: InputFormat
  }
): AdminDataPayload | null {
  const result = normalizeInput(input, 'input', 'input', options?.inputFormat)
  if (result.accounts.length === 0) {
    return null
  }

  const accounts: AdminDataAccount[] = result.accounts.map((account) =>
    normalizedToAdminDataAccount(account, options)
  )

  return {
    type: 'LightBridge-data',
    version: 1,
    exported_at: new Date().toISOString(),
    proxies: [],
    accounts
  }
}

/**
 * Convert a LightBridge AdminDataAccount back to a NormalizedAccount.
 */
export function normalizedFromAdminDataAccount(account: AdminDataAccount): NormalizedAccount {
  const creds = account.credentials ?? {}
  const extra = account.extra ?? {}
  const isXai = account.platform === 'grok'
  const expiresAt = normalizeAdminExpiresAt(
    account.expires_at ?? creds.expires_at ?? creds.expired
  )
  const scope = stringValue(creds.scope)

  return {
    provider: isXai ? 'xai' : 'openai',
    platform: isXai ? 'grok' : 'openai',
    accessToken: stringValue(creds.access_token),
    refreshToken: stringValue(creds.refresh_token),
    idToken: stringValue(creds.id_token),
    idTokenSynthetic: !isXai && booleanValue(extra.id_token_synthetic),
    sessionToken: isXai ? undefined : stringValue(creds.session_token),
    accountId: isXai ? undefined : stringValue(creds.account_id),
    subject: isXai ? stringValue(creds.sub ?? creds.subject) : undefined,
    chatgptAccountId: isXai ? undefined : stringValue(creds.chatgpt_account_id),
    chatgptUserId: isXai ? undefined : stringValue(creds.chatgpt_user_id),
    email: stringValue(creds.email),
    name: account.name,
    planType: isXai ? undefined : stringValue(creds.plan_type),
    subscriptionTier: isXai ? stringValue(creds.subscription_tier ?? extra.subscription_tier) : undefined,
    entitlementStatus: isXai ? stringValue(creds.entitlement_status ?? extra.entitlement_status) : undefined,
    tokenType: isXai ? stringValue(creds.token_type) : undefined,
    expiresIn: isXai ? numberValue(creds.expires_in) : undefined,
    issuer: isXai ? 'https://auth.x.ai' : 'https://auth.openai.com',
    clientId: isXai ? stringValue(creds.client_id) : undefined,
    scopes: isXai && scope ? scope.split(/\s+/).filter(Boolean) : undefined,
    lastRefresh: isXai ? normalizeAdminExpiresAt(creds.last_refresh) : undefined,
    expiresAt,
    baseUrl: isXai ? stringValue(creds.base_url) : undefined,
    redirectUri: isXai ? stringValue(creds.redirect_uri) : undefined,
    tokenEndpoint: isXai ? stringValue(creds.token_endpoint) : undefined,
    authKind: isXai ? (stringValue(creds.auth_kind) ?? 'oauth') : undefined,
    usingApi: isXai ? booleanValue(creds.using_api) : undefined,
    disabled: false,
    sourceName: account.name,
    sourcePath: account.name,
    warnings: []
  }
}

function stringValue(value: unknown): string | undefined {
  if (typeof value === 'string') {
    return value.trim() || undefined
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  return undefined
}

function numberValue(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value
  }
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : undefined
  }
  return undefined
}

function booleanValue(value: unknown): boolean | undefined {
  if (typeof value === 'boolean') return value
  if (typeof value === 'number') {
    if (value === 1) return true
    if (value === 0) return false
  }
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (normalized === 'true' || normalized === '1') return true
    if (normalized === 'false' || normalized === '0') return false
  }
  return undefined
}

function normalizeAdminExpiresAt(value: unknown): string | undefined {
  if (value === null || value === undefined || value === '') {
    return undefined
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return new Date(value > 1_000_000_000_000 ? value : value * 1000).toISOString()
  }
  if (typeof value !== 'string') {
    return undefined
  }
  const trimmed = value.trim()
  if (!trimmed) {
    return undefined
  }
  if (/^\d+$/.test(trimmed)) {
    return normalizeAdminExpiresAt(Number(trimmed))
  }
  const date = new Date(trimmed)
  if (!Number.isNaN(date.getTime())) {
    return date.toISOString()
  }
  return trimmed
}

function expiresAtUnix(value: string | undefined): number | null {
  if (!value) return null
  const date = new Date(value)
  if (!Number.isNaN(date.getTime())) {
    return Math.floor(date.getTime() / 1000)
  }
  if (/^\d+$/.test(value.trim())) {
    const parsed = Number(value.trim())
    if (Number.isFinite(parsed)) {
      return Math.floor(parsed > 1_000_000_000_000 ? parsed / 1000 : parsed)
    }
  }
  return null
}

/**
 * Convert a NormalizedAccount to LightBridge's AdminDataAccount format.
 */
function normalizedToAdminDataAccount(
  account: NormalizedAccount,
  options?: {
    concurrency?: number
    priority?: number
    rate_multiplier?: number
    auto_pause_on_expired?: boolean
  }
): AdminDataAccount {
  const isXai = account.provider === 'xai' || account.platform === 'grok'
  const credentials: Record<string, unknown> = {}
  if (account.accessToken) credentials.access_token = account.accessToken
  if (account.refreshToken) credentials.refresh_token = account.refreshToken
  if (account.idToken) credentials.id_token = account.idToken
  if (!isXai && account.sessionToken) credentials.session_token = account.sessionToken
  if (account.email) credentials.email = account.email

  if (isXai) {
    if (account.subject ?? account.userId) credentials.sub = account.subject ?? account.userId
    if (account.tokenType) credentials.token_type = account.tokenType
    if (account.expiresIn !== undefined) credentials.expires_in = account.expiresIn
    if (account.lastRefresh) credentials.last_refresh = account.lastRefresh
    if (account.baseUrl) credentials.base_url = account.baseUrl
    if (account.redirectUri) credentials.redirect_uri = account.redirectUri
    if (account.tokenEndpoint) credentials.token_endpoint = account.tokenEndpoint
    if (account.clientId) credentials.client_id = account.clientId
    if (account.scopes?.length) credentials.scope = account.scopes.join(' ')
    credentials.auth_kind = account.authKind ?? 'oauth'
    if (account.usingApi !== undefined) credentials.using_api = account.usingApi
    if (account.subscriptionTier) credentials.subscription_tier = account.subscriptionTier
    if (account.entitlementStatus) credentials.entitlement_status = account.entitlementStatus
    if (account.expiresAt) credentials.expires_at = account.expiresAt
  } else {
    if (account.accountId) credentials.account_id = account.accountId
    if (account.chatgptAccountId) credentials.chatgpt_account_id = account.chatgptAccountId
    if (account.chatgptUserId) credentials.chatgpt_user_id = account.chatgptUserId
    if (account.planType) credentials.plan_type = account.planType
  }

  const extra: Record<string, unknown> = { import_source: 'authconv' }
  if (!isXai && account.idTokenSynthetic) extra.id_token_synthetic = true

  return {
    name: account.name ?? account.email ?? account.subject ?? account.accountId ?? (isXai ? 'xAI' : 'imported-account'),
    platform: isXai ? 'grok' : 'openai',
    type: 'oauth',
    credentials,
    extra,
    concurrency: options?.concurrency ?? (isXai ? 1 : 10),
    priority: options?.priority ?? 1,
    rate_multiplier: options?.rate_multiplier ?? 1,
    expires_at: expiresAtUnix(account.expiresAt),
    auto_pause_on_expired: options?.auto_pause_on_expired ?? true
  }
}

/**
 * Convert LightBridge AdminDataPayload accounts to a target authconv format.
 * Returns the rendered output as a JSON-serializable object.
 */
export function convertFromPayload(
  payload: AdminDataPayload,
  targetFormat: OutputFormat,
  options?: RenderOptions
): unknown {
  const accounts: NormalizedAccount[] = payload.accounts.map((account) =>
    normalizedFromAdminDataAccount(account)
  )

  if (accounts.length === 0) {
    return null
  }

  return renderFormat(accounts, targetFormat, options)
}
