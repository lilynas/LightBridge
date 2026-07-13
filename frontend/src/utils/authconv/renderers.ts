/**
 * Format renderers — ported from https://github.com/ltxgit/authconv
 * Renders NormalizedAccount into various output formats.
 */

import { compactObject } from './object'
import type {
  Codex2ApiRenderedAccount,
  CodexManagerRenderedAccount,
  CodexRenderedAuth,
  CpaCodexRenderedAccount,
  CpaRenderedAccount,
  CpaXaiRenderedAccount,
  NormalizedAccount,
  OutputFormat,
  RenderOptions,
  RenderOutputByFormat,
  Sub2ApiRenderedAccount,
  Sub2ApiRenderedCredentials,
  Sub2ApiRenderedData,
  Sub2ApiRenderedExtra
} from './types'

export function renderFormat<T extends OutputFormat>(
  accounts: NormalizedAccount[],
  format: T,
  options: RenderOptions = {}
): RenderOutputByFormat[T] {
  switch (format) {
    case 'cpa':
      return (accounts.length === 1
        ? renderCpaAccount(accounts[0], options)
        : accounts.map((account) => renderCpaAccount(account, options))) as RenderOutputByFormat[T]
    case 'codex2api':
      return accounts.map((account) => renderCodex2ApiAccount(account, options)) as RenderOutputByFormat[T]
    case 'sub2api':
      return renderSub2Api(accounts, options) as RenderOutputByFormat[T]
    case 'codexmanager':
      return (accounts.length === 1
        ? renderCodexManagerAccount(accounts[0], options)
        : accounts.map((account) => renderCodexManagerAccount(account, options))) as RenderOutputByFormat[T]
    case 'codex':
      return (accounts.length === 1
        ? renderCodexAuth(accounts[0], options)
        : accounts.map((account) => renderCodexAuth(account, options))) as RenderOutputByFormat[T]
    default:
      return renderSub2Api(accounts, options) as RenderOutputByFormat[T]
  }
}

function isXaiAccount(account: NormalizedAccount): boolean {
  return account.provider === 'xai' || account.platform === 'grok'
}

function renderCpaAccount(account: NormalizedAccount, options: RenderOptions): CpaRenderedAccount {
  return isXaiAccount(account)
    ? renderCpaXaiAccount(account, options)
    : renderCpaCodexAccount(account, options)
}

function renderCpaCodexAccount(account: NormalizedAccount, options: RenderOptions): CpaCodexRenderedAccount {
  const allowSynthetic = options.allowSyntheticIdToken !== false
  const rendered: CpaCodexRenderedAccount = {
    type: 'codex',
    email: account.email ?? '',
    account_id: account.accountId ?? '',
    plan_type: account.planType ?? '',
    id_token: allowSynthetic ? (account.idToken ?? '') : (account.idTokenSynthetic ? '' : (account.idToken ?? '')),
    access_token: account.accessToken ?? '',
    refresh_token: account.refreshToken ?? '',
    expired: account.expiresAt ?? '',
    last_refresh: account.lastRefresh ?? (options.now ?? new Date()).toISOString(),
    disabled: false
  }
  if (account.sessionToken) {
    rendered.session_token = account.sessionToken
  }
  if (account.idTokenSynthetic && allowSynthetic) {
    rendered.id_token_synthetic = true
  }
  return rendered
}

function renderCpaXaiAccount(account: NormalizedAccount, options: RenderOptions): CpaXaiRenderedAccount {
  const rendered: CpaXaiRenderedAccount = {
    type: 'xai',
    access_token: account.accessToken ?? '',
    refresh_token: account.refreshToken ?? '',
    auth_kind: 'oauth',
    disabled: false
  }
  if (account.idToken && !account.idTokenSynthetic) rendered.id_token = account.idToken
  if (account.tokenType) rendered.token_type = account.tokenType
  if (account.expiresIn !== undefined) rendered.expires_in = account.expiresIn
  if (account.expiresAt) rendered.expired = account.expiresAt
  if (account.lastRefresh) rendered.last_refresh = account.lastRefresh
  else rendered.last_refresh = (options.now ?? new Date()).toISOString()
  if (account.email) rendered.email = account.email
  if (account.subject ?? account.userId) rendered.sub = account.subject ?? account.userId
  if (account.baseUrl) rendered.base_url = account.baseUrl
  if (account.redirectUri) rendered.redirect_uri = account.redirectUri
  if (account.tokenEndpoint) rendered.token_endpoint = account.tokenEndpoint
  if (account.usingApi !== undefined) rendered.using_api = account.usingApi
  if (account.subscriptionTier) rendered.subscription_tier = account.subscriptionTier
  if (account.entitlementStatus) rendered.entitlement_status = account.entitlementStatus
  return rendered
}

function renderCodex2ApiAccount(account: NormalizedAccount, options: RenderOptions): Codex2ApiRenderedAccount {
  const allowSynthetic = options.allowSyntheticIdToken !== false
  return compactObject({
    name: account.name ?? account.email ?? account.subject ?? account.chatgptAccountId ?? account.accountId,
    email: account.email,
    refresh_token: account.refreshToken,
    session_token: account.sessionToken,
    access_token: account.accessToken,
    id_token: allowSynthetic ? account.idToken : (account.idTokenSynthetic ? undefined : account.idToken),
    account_id: account.accountId ?? account.subject,
    chatgpt_account_id: account.chatgptAccountId,
    chatgpt_user_id: account.chatgptUserId,
    plan_type: account.planType,
    expires_at: account.expiresAt
  }) as Codex2ApiRenderedAccount
}

function renderSub2Api(accounts: NormalizedAccount[], options: RenderOptions): Sub2ApiRenderedData {
  return {
    type: 'sub2api-data',
    version: 1,
    exported_at: (options.now ?? new Date()).toISOString(),
    proxies: [],
    accounts: accounts.map((account) => renderSub2ApiAccount(account, options))
  }
}

function renderSub2ApiAccount(account: NormalizedAccount, options: RenderOptions): Sub2ApiRenderedAccount {
  const allowSynthetic = options.allowSyntheticIdToken !== false
  const xai = isXaiAccount(account)
  const credentials = compactObject({
    access_token: account.accessToken,
    refresh_token: account.refreshToken,
    session_token: xai ? undefined : account.sessionToken,
    id_token: allowSynthetic ? account.idToken : (account.idTokenSynthetic ? undefined : account.idToken),
    expires_at: account.expiresAt,
    email: account.email,
    account_id: xai ? undefined : account.accountId,
    sub: xai ? (account.subject ?? account.userId) : undefined,
    chatgpt_account_id: xai ? undefined : account.chatgptAccountId,
    chatgpt_user_id: xai ? undefined : account.chatgptUserId,
    plan_type: xai ? undefined : account.planType,
    token_type: xai ? account.tokenType : undefined,
    expires_in: xai ? account.expiresIn : undefined,
    last_refresh: xai ? account.lastRefresh : undefined,
    base_url: xai ? account.baseUrl : undefined,
    redirect_uri: xai ? account.redirectUri : undefined,
    token_endpoint: xai ? account.tokenEndpoint : undefined,
    auth_kind: xai ? (account.authKind ?? 'oauth') : undefined,
    using_api: xai ? account.usingApi : undefined,
    subscription_tier: xai ? account.subscriptionTier : undefined,
    entitlement_status: xai ? account.entitlementStatus : undefined
  }) as Sub2ApiRenderedCredentials
  const extra = compactObject({
    import_source: 'authconv',
    id_token_synthetic: !xai && account.idTokenSynthetic && allowSynthetic ? true : undefined
  }) as Sub2ApiRenderedExtra
  return {
    name: account.name ?? account.email ?? account.subject ?? account.chatgptAccountId ?? account.accountId ?? (xai ? 'xAI' : 'authconv-account'),
    platform: xai ? 'grok' : 'openai',
    type: 'oauth',
    credentials,
    extra,
    priority: 50,
    concurrency: xai ? 1 : 3,
    auto_pause_on_expired: true
  }
}

function renderCodexManagerAccount(account: NormalizedAccount, options: RenderOptions): CodexManagerRenderedAccount {
  const allowSynthetic = options.allowSyntheticIdToken !== false
  return {
    tokens: compactObject({
      access_token: account.accessToken,
      refresh_token: account.refreshToken,
      id_token: allowSynthetic ? account.idToken : (account.idTokenSynthetic ? undefined : account.idToken),
      account_id: account.accountId ?? account.subject,
      chatgpt_account_id: account.chatgptAccountId,
      chatgpt_user_id: account.chatgptUserId
    }),
    meta: compactObject({
      label: account.name ?? account.email ?? account.subject ?? account.chatgptAccountId ?? account.accountId,
      issuer: account.issuer ?? (isXaiAccount(account) ? 'https://auth.x.ai' : 'https://auth.openai.com'),
      workspace_id: account.workspaceId,
      chatgpt_account_id: account.chatgptAccountId,
      chatgpt_user_id: account.chatgptUserId,
      tags: ['authconv']
    }) as CodexManagerRenderedAccount['meta']
  }
}

function renderCodexAuth(account: NormalizedAccount, options: RenderOptions): CodexRenderedAuth {
  const allowSynthetic = options.allowSyntheticIdToken !== false
  return {
    auth_mode: 'chatgpt',
    OPENAI_API_KEY: null,
    tokens: {
      id_token: allowSynthetic ? (account.idToken ?? '') : (account.idTokenSynthetic ? '' : (account.idToken ?? '')),
      access_token: account.accessToken ?? '',
      refresh_token: account.refreshToken ?? '',
      account_id: account.accountId ?? account.chatgptAccountId ?? account.subject ?? '',
      chatgpt_user_id: account.chatgptUserId
    },
    last_refresh: account.lastRefresh ?? (options.now ?? new Date()).toISOString()
  }
}
