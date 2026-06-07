import { ref } from 'vue'
import { useAppStore } from '@/stores/app'

export type AddMethod = 'oauth' | 'setup-token'
export type AuthInputMethod = 'manual' | 'cookie' | 'refresh_token' | 'mobile_refresh_token' | 'session_token' | 'access_token' | 'codex_session'

const ANTHROPIC_PROVIDER_ID = 'anthropic-oauth'
const ANTHROPIC_MODULE_ID = 'lightbridge-provider-anthropic-oauth'
const CLAUDE_OAUTH_AUTHORIZE_URL = 'https://claude.ai/oauth/authorize'
const CLAUDE_OAUTH_CLIENT_ID = '9d1c250a-e61b-44d9-88ed-5944d1962f5e'
const CLAUDE_OAUTH_REDIRECT_URI = 'https://platform.claude.com/oauth/code/callback'
const CLAUDE_OAUTH_SCOPE_FULL = 'org:create_api_key user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload'
const CLAUDE_OAUTH_SCOPE_INFERENCE = 'user:inference'

export interface OAuthState {
  authUrl: string
  authCode: string
  sessionId: string
  sessionKey: string
  loading: boolean
  error: string
}

export interface TokenInfo {
  org_uuid?: string
  account_uuid?: string
  email_address?: string
  [key: string]: unknown
}

function base64UrlEncode(bytes: Uint8Array): string {
  let binary = ''
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte)
  })
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}

function randomString(bytes = 32): string {
  const values = new Uint8Array(bytes)
  crypto.getRandomValues(values)
  return base64UrlEncode(values)
}

async function sha256Base64Url(input: string): Promise<string> {
  const data = new TextEncoder().encode(input)
  const digest = await crypto.subtle.digest('SHA-256', data)
  return base64UrlEncode(new Uint8Array(digest))
}

function parseAuthorizationCode(raw: string): { code: string; state?: string } {
  const trimmed = raw.trim()
  if (!trimmed) return { code: '' }
  try {
    const parsed = new URL(trimmed)
    return {
      code: parsed.searchParams.get('code') || trimmed,
      state: parsed.searchParams.get('state') || undefined
    }
  } catch {
    const [code, state] = trimmed.split('#')
    return { code: code.trim(), state: state?.trim() || undefined }
  }
}

export function useAccountOAuth() {
  const appStore = useAppStore()

  // State
  const authUrl = ref('')
  const authCode = ref('')
  const sessionId = ref('')
  const sessionKey = ref('')
  const loading = ref(false)
  const error = ref('')

  const codeVerifier = ref('')
  const oauthState = ref('')

  // Reset state
  const resetState = () => {
    authUrl.value = ''
    authCode.value = ''
    sessionId.value = ''
    sessionKey.value = ''
    loading.value = false
    error.value = ''
    codeVerifier.value = ''
    oauthState.value = ''
  }

  // Generate Claude OAuth URL locally. Token exchange is handled by the provider module.
  const generateAuthUrl = async (
    addMethod: AddMethod,
    _proxyId?: number | null
  ): Promise<boolean> => {
    loading.value = true
    authUrl.value = ''
    sessionId.value = ''
    error.value = ''

    try {
      const verifier = randomString(48)
      const state = randomString(32)
      const challenge = await sha256Base64Url(verifier)
      const scope = addMethod === 'setup-token' ? CLAUDE_OAUTH_SCOPE_INFERENCE : CLAUDE_OAUTH_SCOPE_FULL
      const params = new URLSearchParams({
        code: 'true',
        client_id: CLAUDE_OAUTH_CLIENT_ID,
        response_type: 'code',
        redirect_uri: CLAUDE_OAUTH_REDIRECT_URI,
        scope,
        code_challenge: challenge,
        code_challenge_method: 'S256',
        state
      })

      codeVerifier.value = verifier
      oauthState.value = state
      sessionId.value = state
      authUrl.value = `${CLAUDE_OAUTH_AUTHORIZE_URL}?${params.toString()}`
      return true
    } catch (err: any) {
      error.value = err?.message || 'Failed to generate auth URL'
      appStore.showError(error.value)
      return false
    } finally {
      loading.value = false
    }
  }

  // Build provider-module credentials from the authorization code.
  const exchangeAuthCode = async (
    addMethod: AddMethod,
    _proxyId?: number | null
  ): Promise<TokenInfo | null> => {
    const parsed = parseAuthorizationCode(authCode.value)
    if (!parsed.code || !codeVerifier.value) {
      error.value = 'Missing auth code or verifier'
      return null
    }

    loading.value = true
    error.value = ''

    try {
      const scope = addMethod === 'setup-token' ? CLAUDE_OAUTH_SCOPE_INFERENCE : CLAUDE_OAUTH_SCOPE_FULL
      return {
        provider_id: ANTHROPIC_PROVIDER_ID,
        module_id: ANTHROPIC_MODULE_ID,
        authorization_code: parsed.code,
        code_verifier: codeVerifier.value,
        oauth_state: parsed.state || oauthState.value,
        scope
      }
    } finally {
      loading.value = false
    }
  }

  // Cookie-based authentication is completed by the provider module during account creation/update.
  const cookieAuth = async (
    addMethod: AddMethod,
    sessionKeyValue: string,
    _proxyId?: number | null
  ): Promise<TokenInfo | null> => {
    if (!sessionKeyValue.trim()) {
      error.value = 'Please enter sessionKey'
      return null
    }

    const scope = addMethod === 'setup-token' ? CLAUDE_OAUTH_SCOPE_INFERENCE : CLAUDE_OAUTH_SCOPE_FULL
    return {
      provider_id: ANTHROPIC_PROVIDER_ID,
      module_id: ANTHROPIC_MODULE_ID,
      session_key: sessionKeyValue.trim(),
      scope
    }
  }

  // Parse multiple session keys
  const parseSessionKeys = (input: string): string[] => {
    return input
      .split('\n')
      .map((k) => k.trim())
      .filter((k) => k)
  }

  // Build extra info from module login credentials/metadata.
  const buildExtraInfo = (tokenInfo: TokenInfo): Record<string, string | boolean> | undefined => {
    const extra: Record<string, string | boolean> = {
      provider_id: ANTHROPIC_PROVIDER_ID,
      module_id: ANTHROPIC_MODULE_ID,
      platform: 'module',
      setup_token: tokenInfo.scope === CLAUDE_OAUTH_SCOPE_INFERENCE
    }
    if (typeof tokenInfo.org_uuid === 'string' && tokenInfo.org_uuid) {
      extra.org_uuid = tokenInfo.org_uuid
    }
    if (typeof tokenInfo.account_uuid === 'string' && tokenInfo.account_uuid) {
      extra.account_uuid = tokenInfo.account_uuid
    }
    if (typeof tokenInfo.email_address === 'string' && tokenInfo.email_address) {
      extra.email_address = tokenInfo.email_address
    }
    if (typeof tokenInfo.scope === 'string' && tokenInfo.scope) {
      extra.oauth_scope = tokenInfo.scope
    }
    return extra
  }

  return {
    // State
    authUrl,
    authCode,
    sessionId,
    sessionKey,
    loading,
    error,
    // Methods
    resetState,
    generateAuthUrl,
    exchangeAuthCode,
    cookieAuth,
    parseSessionKeys,
    buildExtraInfo
  }
}
