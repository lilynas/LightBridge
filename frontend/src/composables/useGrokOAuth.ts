import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type {
  GrokAuthUrlRequest,
  GrokExchangeCodeRequest,
  GrokRefreshTokenRequest,
  GrokTokenInfo
} from '@/api/admin/grok'
import { extractApiErrorMessage } from '@/utils/apiError'

export function useGrokOAuth() {
  const appStore = useAppStore()
  const { t } = useI18n()

  const authUrl = ref('')
  const sessionId = ref('')
  const state = ref('')
  const loading = ref(false)
  const error = ref('')

  const resetState = () => {
    authUrl.value = ''
    sessionId.value = ''
    state.value = ''
    loading.value = false
    error.value = ''
  }

  const generateAuthUrl = async (
    proxyId?: number | null,
    redirectUri?: string | null
  ): Promise<boolean> => {
    loading.value = true
    authUrl.value = ''
    sessionId.value = ''
    state.value = ''
    error.value = ''

    try {
      const payload: GrokAuthUrlRequest = {}
      if (proxyId) payload.proxy_id = proxyId
      const trimmedRedirectURI = redirectUri?.trim()
      if (trimmedRedirectURI) payload.redirect_uri = trimmedRedirectURI

      const response = await adminAPI.grok.generateAuthUrl(payload)
      authUrl.value = response.auth_url
      sessionId.value = response.session_id
      state.value = response.state
      return true
    } catch (err: any) {
      error.value = extractApiErrorMessage(err, t('admin.accounts.oauth.grok.failedToGenerateUrl'))
      appStore.showError(error.value)
      return false
    } finally {
      loading.value = false
    }
  }

  const exchangeAuthCode = async (params: {
    code: string
    sessionId: string
    state?: string
    proxyId?: number | null
    redirectUri?: string | null
  }): Promise<GrokTokenInfo | null> => {
    const code = params.code?.trim()
    if (!code || !params.sessionId) {
      error.value = t('admin.accounts.oauth.grok.missingExchangeParams')
      return null
    }

    loading.value = true
    error.value = ''

    try {
      const payload: GrokExchangeCodeRequest = {
        session_id: params.sessionId,
        code
      }
      const trimmedState = params.state?.trim()
      if (trimmedState) payload.state = trimmedState
      const trimmedRedirectURI = params.redirectUri?.trim()
      if (trimmedRedirectURI) payload.redirect_uri = trimmedRedirectURI
      if (params.proxyId) payload.proxy_id = params.proxyId

      return await adminAPI.grok.exchangeCode(payload)
    } catch (err: any) {
      error.value = extractApiErrorMessage(err, t('admin.accounts.oauth.grok.failedToExchangeCode'))
      appStore.showError(error.value)
      return null
    } finally {
      loading.value = false
    }
  }

  const validateRefreshToken = async (
    refreshToken: string,
    proxyId?: number | null,
    clientId?: string | null
  ): Promise<GrokTokenInfo | null> => {
    const token = refreshToken.trim()
    if (!token) {
      error.value = t('admin.accounts.oauth.grok.pleaseEnterRefreshToken')
      return null
    }

    loading.value = true
    error.value = ''

    try {
      const payload: GrokRefreshTokenRequest = { refresh_token: token }
      if (proxyId) payload.proxy_id = proxyId
      const trimmedClientID = clientId?.trim()
      if (trimmedClientID) payload.client_id = trimmedClientID
      return await adminAPI.grok.refreshToken(payload)
    } catch (err: any) {
      error.value = extractApiErrorMessage(err, t('admin.accounts.oauth.grok.failedToValidateRT'))
      appStore.showError(error.value)
      return null
    } finally {
      loading.value = false
    }
  }

  const buildCredentials = (tokenInfo: GrokTokenInfo): Record<string, unknown> => {
    const credentials: Record<string, unknown> = {
      access_token: tokenInfo.access_token,
      token_type: tokenInfo.token_type,
      scope: tokenInfo.scope
    }

    if (tokenInfo.refresh_token) credentials.refresh_token = tokenInfo.refresh_token
    if (tokenInfo.id_token) credentials.id_token = tokenInfo.id_token
    if (tokenInfo.client_id) credentials.client_id = tokenInfo.client_id
    if (tokenInfo.email) credentials.email = tokenInfo.email
    if (tokenInfo.base_url) credentials.base_url = tokenInfo.base_url
    if (tokenInfo.subscription_tier) credentials.subscription_tier = tokenInfo.subscription_tier
    if (tokenInfo.entitlement_status) credentials.entitlement_status = tokenInfo.entitlement_status
    if (typeof tokenInfo.expires_at === 'number' && Number.isFinite(tokenInfo.expires_at)) {
      credentials.expires_at = Math.floor(tokenInfo.expires_at).toString()
    } else if (typeof tokenInfo.expires_at === 'string' && tokenInfo.expires_at.trim()) {
      credentials.expires_at = tokenInfo.expires_at.trim()
    }

    return credentials
  }

  const buildExtraInfo = (tokenInfo: GrokTokenInfo): Record<string, string> | undefined => {
    const extra: Record<string, string> = {}
    if (tokenInfo.email) extra.email = tokenInfo.email
    if (tokenInfo.name) extra.name = tokenInfo.name
    if (tokenInfo.subscription_tier) extra.subscription_tier = tokenInfo.subscription_tier
    if (tokenInfo.entitlement_status) extra.entitlement_status = tokenInfo.entitlement_status
    return Object.keys(extra).length > 0 ? extra : undefined
  }

  return {
    authUrl,
    sessionId,
    state,
    loading,
    error,
    resetState,
    generateAuthUrl,
    exchangeAuthCode,
    validateRefreshToken,
    buildCredentials,
    buildExtraInfo
  }
}
