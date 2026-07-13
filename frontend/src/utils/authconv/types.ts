/**
 * Authconv type definitions — ported from https://github.com/ltxgit/authconv
 * Supports converting between CPA, sub2api, codex2api, Codex-Manager, and Codex auth.json formats.
 */

export const ALL_FORMATS = ['cpa', 'sub2api', 'codex2api', 'codexmanager', 'codex'] as const

export type OutputFormat = (typeof ALL_FORMATS)[number]

export type OutputMode = 'merged' | 'single'

export type OutputModes = Partial<Record<OutputFormat, OutputMode>>

export type OutputTextMode = 'json' | 'jsonl'

export type InputFormat = 'session' | 'sub2api' | 'cpa' | 'codexmanager' | 'codex2api' | 'codex' | 'unknown'

export type AuthProvider = 'openai' | 'xai'

export type NormalizedAccount = {
  provider?: AuthProvider
  platform?: 'openai' | 'grok'
  accessToken?: string
  refreshToken?: string
  idToken?: string
  idTokenSynthetic?: boolean
  sessionToken?: string
  accountId?: string
  subject?: string
  chatgptAccountId?: string
  chatgptUserId?: string
  chatgptAccountUserId?: string
  workspaceId?: string
  userId?: string
  issuer?: string
  audience?: string[]
  clientId?: string
  scopes?: string[]
  notBefore?: string
  email?: string
  name?: string
  planType?: string
  subscriptionTier?: string
  entitlementStatus?: string
  tokenType?: string
  expiresIn?: number
  lastRefresh?: string
  expiresAt?: string
  baseUrl?: string
  redirectUri?: string
  tokenEndpoint?: string
  authKind?: string
  usingApi?: boolean
  disabled?: boolean
  sourceName: string
  sourcePath: string
  warnings: string[]
  inputFormat?: InputFormat
}

export type NormalizeResult = {
  accounts: NormalizedAccount[]
  warnings: string[]
  inputFormat: InputFormat
}

export type RenderOptions = {
  now?: Date
  allowSyntheticIdToken?: boolean
}

export type CpaCodexRenderedAccount = {
  type: 'codex'
  email: string
  account_id: string
  plan_type: string
  id_token: string
  access_token: string
  refresh_token: string
  expired: string
  last_refresh: string
  disabled: false
  session_token?: string
  id_token_synthetic?: true
}

export type CpaXaiRenderedAccount = {
  type: 'xai'
  access_token: string
  refresh_token: string
  auth_kind: 'oauth'
  disabled: false
  id_token?: string
  token_type?: string
  expires_in?: number
  expired?: string
  last_refresh?: string
  email?: string
  sub?: string
  base_url?: string
  redirect_uri?: string
  token_endpoint?: string
  using_api?: boolean
  subscription_tier?: string
  entitlement_status?: string
}

export type CpaRenderedAccount = CpaCodexRenderedAccount | CpaXaiRenderedAccount

export type Codex2ApiRenderedAccount = {
  name?: string
  email?: string
  refresh_token?: string
  session_token?: string
  access_token?: string
  id_token?: string
  account_id?: string
  chatgpt_account_id?: string
  chatgpt_user_id?: string
  plan_type?: string
  expires_at?: string
}

export type Sub2ApiRenderedCredentials = {
  access_token?: string
  refresh_token?: string
  session_token?: string
  id_token?: string
  expires_at?: string
  email?: string
  account_id?: string
  sub?: string
  chatgpt_account_id?: string
  chatgpt_user_id?: string
  plan_type?: string
  token_type?: string
  expires_in?: number
  last_refresh?: string
  base_url?: string
  redirect_uri?: string
  token_endpoint?: string
  auth_kind?: string
  using_api?: boolean
  subscription_tier?: string
  entitlement_status?: string
}

export type Sub2ApiRenderedExtra = {
  import_source: 'authconv'
  id_token_synthetic?: true
}

export type Sub2ApiRenderedAccount = {
  name: string
  platform: 'openai' | 'grok'
  type: 'oauth'
  credentials: Sub2ApiRenderedCredentials
  extra: Sub2ApiRenderedExtra
  priority: 50
  concurrency: number
  auto_pause_on_expired: true
}

export type Sub2ApiRenderedData = {
  type: 'sub2api-data'
  version: 1
  exported_at: string
  proxies: []
  accounts: Sub2ApiRenderedAccount[]
}

export type CodexManagerRenderedAccount = {
  tokens: {
    access_token?: string
    refresh_token?: string
    id_token?: string
    account_id?: string
    chatgpt_account_id?: string
    chatgpt_user_id?: string
  }
  meta: {
    label?: string
    issuer: string
    workspace_id?: string
    chatgpt_account_id?: string
    chatgpt_user_id?: string
    tags: ['authconv']
  }
}

export type CodexRenderedAuth = {
  auth_mode: 'chatgpt'
  OPENAI_API_KEY: null
  tokens: {
    id_token: string
    access_token: string
    refresh_token: string
    account_id: string
    chatgpt_user_id?: string
  }
  last_refresh: string
}

export type RenderOutputByFormat = {
  cpa: CpaRenderedAccount | CpaRenderedAccount[]
  codex2api: Codex2ApiRenderedAccount[]
  sub2api: Sub2ApiRenderedData
  codexmanager: CodexManagerRenderedAccount | CodexManagerRenderedAccount[]
  codex: CodexRenderedAuth | CodexRenderedAuth[]
}

export type RenderedOutput = RenderOutputByFormat[OutputFormat]
