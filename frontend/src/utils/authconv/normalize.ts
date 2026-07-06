/**
 * Input normalization — ported from https://github.com/ltxgit/authconv
 * Detects input format and normalizes any recognized credential format into NormalizedAccount.
 */

import {
  applySyntheticIdTokenSignature,
  claimNumber,
  claimString,
  claimStringArray,
  createSyntheticIdToken,
  decodeJwtPayload,
  openAIAuthClaims,
  openAIProfileClaims
} from './jwt'
import { firstRecord, firstString, isRecord } from './object'
import type { InputFormat, NormalizedAccount, NormalizeResult } from './types'

type Candidate = {
  records: Record<string, unknown>[]
  sourceName: string
  sourcePath: string
  inputFormat: InputFormat
}

export function detectInputFormat(input: unknown): InputFormat {
  if (Array.isArray(input)) {
    const formats = uniqueFormats(input.filter(isRecord).map(detectArrayItemFormat))
    if (formats.length === 1) {
      return formats[0]
    }
    return 'unknown'
  }

  if (!isRecord(input)) {
    return 'unknown'
  }

  return detectRecordInputFormat(input)
}

function detectArrayItemFormat(input: Record<string, unknown>): InputFormat {
  const format = detectRecordInputFormat(input)
  if (format !== 'unknown') {
    return format
  }
  if (typeof input.refresh_token === 'string' && typeof input.session_token === 'string') {
    return 'codex2api'
  }
  return 'unknown'
}

function detectRecordInputFormat(input: Record<string, unknown>): InputFormat {
  // ChatGPT Session JSON
  if (typeof input.accessToken === 'string' && (isRecord(input.user) || isRecord(input.account) || typeof input.sessionToken === 'string')) {
    return 'session'
  }

  // sub2api
  if (Array.isArray(input.accounts)) {
    const hasSub2ApiAccount = input.accounts.some((item) => {
      return isRecord(item) && (isRecord(item.credentials) || typeof item.platform === 'string')
    })
    if (hasSub2ApiAccount) {
      return 'sub2api'
    }
  }
  if (isRecord(input.credentials)) {
    return 'sub2api'
  }

  // CPA
  if (input.type === 'codex' && (typeof input.access_token === 'string' || typeof input.refresh_token === 'string' || typeof input.session_token === 'string')) {
    return 'cpa'
  }

  // Codex auth.json
  if (isCodexAuthRecord(input)) {
    return 'codex'
  }

  // Codex-Manager
  if (isRecord(input.tokens) && isRecord(input.meta)) {
    return 'codexmanager'
  }

  // Codex2Api single object
  if (typeof input.refresh_token === 'string' && typeof input.session_token === 'string' && !isRecord(input.tokens)) {
    return 'codex2api'
  }

  return 'unknown'
}

function isCodexAuthRecord(record: Record<string, unknown>): boolean {
  return (
    record.auth_mode === 'chatgpt' &&
    isRecord(record.tokens) &&
    Boolean(firstString([record.tokens], ['access_token', 'refresh_token', 'id_token']))
  )
}

export function normalizeInput(
  input: unknown,
  sourceName: string = 'input',
  sourcePath: string = 'input',
  forcedFormat?: InputFormat
): NormalizeResult {
  const warnings: string[] = []
  const formatOverride = forcedFormat && forcedFormat !== 'unknown' ? forcedFormat : undefined
  const detectedFormat = formatOverride ?? detectInputFormat(input)
  const candidates = extractAutoCandidates(input, sourceName, sourcePath, formatOverride)
  const accounts = candidates
    .map((candidate, index) => {
      const account = normalizeCandidate(candidate, index)
      if (account) {
        account.inputFormat = candidate.inputFormat
      }
      return account
    })
    .filter((account): account is NormalizedAccount => account !== undefined)

  if (accounts.length === 0) {
    warnings.push(`No recognizable token fields found in ${sourceName}`)
  }

  return {
    accounts,
    warnings: warnings.concat(accounts.flatMap((account) => account.warnings)),
    inputFormat: commonAccountInputFormat(accounts) ?? detectedFormat
  }
}

function uniqueFormats(formats: InputFormat[]): InputFormat[] {
  return Array.from(new Set(formats))
}

function commonAccountInputFormat(accounts: NormalizedAccount[]): InputFormat | undefined {
  if (accounts.length === 0) {
    return undefined
  }
  const formats = uniqueFormats(accounts.map((account) => account.inputFormat ?? 'unknown'))
  return formats.length === 1 ? formats[0] : 'unknown'
}

function extractAutoCandidates(input: unknown, sourceName: string, sourcePath: string, forcedFormat?: InputFormat): Candidate[] {
  if (Array.isArray(input)) {
    return input.flatMap((item, index) => {
      return isRecord(item) ? extractAutoCandidatesFromRecord(item, sourceName, sourcePath, index, forcedFormat) : []
    })
  }

  if (!isRecord(input)) {
    return []
  }

  return extractAutoCandidatesFromRecord(input, sourceName, sourcePath, 0, forcedFormat)
}

function extractAutoCandidatesFromRecord(record: Record<string, unknown>, sourceName: string, sourcePath: string, index: number, forcedFormat?: InputFormat): Candidate[] {
  const inputFormat = forcedFormat && forcedFormat !== 'unknown' ? forcedFormat : detectArrayItemFormat(record)
  if (inputFormat === 'sub2api' && Array.isArray(record.accounts)) {
    return record.accounts
      .filter(isRecord)
      .map((item, accountIndex) => candidateFromRecord(item, sourceName, sourcePath, accountIndex, 'sub2api'))
  }

  return [candidateFromRecord(record, sourceName, sourcePath, index, inputFormat)]
}

function candidateFromRecord(
  record: Record<string, unknown>,
  sourceName: string,
  sourcePath: string,
  index: number,
  inputFormat: InputFormat
): Candidate {
  const records = [record]
  const credentials = firstRecord(records, 'credentials')
  if (credentials) {
    records.unshift(credentials)
  }
  const tokens = firstRecord(records, 'tokens')
  if (tokens) {
    records.unshift(tokens)
  }
  const account = firstRecord([record], 'account')
  if (account) {
    records.push(accountAliases(account))
  }
  const providerSpecificData = firstRecord([record], 'providerSpecificData')
  if (providerSpecificData) {
    records.push(providerSpecificData)
  }
  const meta = firstRecord([record], 'meta')
  if (meta) {
    records.push(meta)
  }
  const user = firstRecord([record], 'user')
  if (user) {
    records.push(userAliases(user))
    records.push(user)
  }
  return {
    records,
    sourceName,
    sourcePath: index === 0 ? sourcePath : `${sourcePath}#${index + 1}`,
    inputFormat
  }
}

function accountAliases(account: Record<string, unknown>): Record<string, unknown> {
  return {
    account_id: account.account_id ?? account.accountId ?? account.id,
    chatgpt_account_id: account.chatgpt_account_id ?? account.chatgptAccountId ?? account.id,
    plan_type: account.plan_type ?? account.planType ?? account.chatgpt_plan_type ?? account.chatgptPlanType,
    workspace_id: account.workspace_id ?? account.workspaceId
  }
}

function userAliases(user: Record<string, unknown>): Record<string, unknown> {
  return {
    user_id: user.user_id ?? user.userId ?? user.id,
    chatgpt_user_id: user.chatgpt_user_id ?? user.chatgptUserId ?? user.id,
    email: user.email,
    name: user.name
  }
}

function normalizeCandidate(candidate: Candidate, index: number): NormalizedAccount | undefined {
  const { records } = candidate
  const accessToken = firstString(records, ['access_token', 'accessToken'])
  const refreshToken = firstString(records, ['refresh_token', 'refreshToken'])
  const sessionToken = firstString(records, ['session_token', 'sessionToken'])
  let idToken = firstString(records, ['id_token', 'idToken'])

  if (!accessToken && !refreshToken && !sessionToken && !idToken) {
    return undefined
  }

  const idClaims = decodeJwtPayload(idToken)
  const accessClaims = decodeJwtPayload(accessToken)
  const accessFirstClaimRecords = [accessClaims, idClaims].filter((claims): claims is Record<string, unknown> => claims !== undefined)
  const expiryClaimRecords = [accessClaims, idClaims].filter((claims): claims is Record<string, unknown> => claims !== undefined)
  const accessAuthClaims = openAIAuthClaims(accessClaims)
  const idAuthClaims = openAIAuthClaims(idClaims)
  const warnings: string[] = []

  const preferClaimIdentity = candidate.inputFormat === 'session' || candidate.inputFormat === 'codex'

  // Account ID
  const claimedAccountId =
    claimString(accessAuthClaims, 'chatgpt_account_id') ??
    claimString(accessClaims, 'chatgpt_account_id') ??
    claimString(idAuthClaims, 'chatgpt_account_id') ??
    claimString(idClaims, 'chatgpt_account_id')
  const recordAccountId = firstString(records, ['account_id', 'accountId'])
  const accountId = preferClaimIdentity ? claimedAccountId ?? recordAccountId : recordAccountId ?? claimedAccountId
  const chatgptAccountId =
    (preferClaimIdentity
      ? claimedAccountId ?? firstString(records, ['chatgpt_account_id', 'chatgptAccountId'])
      : firstString(records, ['chatgpt_account_id', 'chatgptAccountId']) ?? claimedAccountId) ?? accountId

  // User ID
  const claimedChatgptUserId =
    claimString(accessAuthClaims, 'chatgpt_user_id') ??
    claimString(accessAuthClaims, 'user_id') ??
    claimString(accessClaims, 'chatgpt_user_id') ??
    claimString(idAuthClaims, 'chatgpt_user_id') ??
    claimString(idAuthClaims, 'user_id') ??
    claimString(idClaims, 'chatgpt_user_id') ??
    claimString(accessClaims, 'sub') ??
    claimString(idClaims, 'sub')
  const recordChatgptUserId = firstString(records, ['chatgpt_user_id', 'chatgptUserId'])
  const chatgptUserId = preferClaimIdentity ? claimedChatgptUserId ?? recordChatgptUserId : recordChatgptUserId ?? claimedChatgptUserId

  // Issuer
  const claimedIssuer = firstClaimString(accessFirstClaimRecords, ['iss'])
  const recordIssuer = firstString(records, ['issuer', 'iss'])
  const issuer = preferClaimIdentity ? claimedIssuer ?? recordIssuer : recordIssuer ?? claimedIssuer

  // Email
  const claimedEmail =
    claimString(accessClaims, 'email') ??
    claimString(openAIProfileClaims(accessClaims), 'email') ??
    claimString(idClaims, 'email') ??
    claimString(openAIProfileClaims(idClaims), 'email')
  const recordEmail = firstString(records, ['email', 'email_address', 'emailAddress'])
  const email = preferClaimIdentity ? claimedEmail ?? recordEmail : recordEmail ?? claimedEmail

  // Name
  const claimedName =
    claimString(accessClaims, 'name') ??
    claimString(openAIProfileClaims(accessClaims), 'name') ??
    claimString(idClaims, 'name') ??
    claimString(openAIProfileClaims(idClaims), 'name')
  const recordName = firstString(records, ['name', 'label'])
  const name = preferClaimIdentity ? claimedName ?? recordName : recordName ?? claimedName

  // Plan type
  const claimedPlanType =
    claimString(accessAuthClaims, 'chatgpt_plan_type') ??
    claimString(accessAuthClaims, 'plan_type') ??
    claimString(accessClaims, 'chatgpt_plan_type') ??
    claimString(accessClaims, 'plan_type') ??
    claimString(idAuthClaims, 'chatgpt_plan_type') ??
    claimString(idAuthClaims, 'plan_type') ??
    claimString(idClaims, 'chatgpt_plan_type') ??
    claimString(idClaims, 'plan_type')
  const recordPlanType = firstString(records, ['plan_type', 'planType', 'chatgpt_plan_type', 'chatgptPlanType'])
  const planType = preferClaimIdentity ? claimedPlanType ?? recordPlanType : recordPlanType ?? claimedPlanType

  // Workspace ID
  const claimedWorkspaceId =
    claimString(accessClaims, 'workspace_id') ??
    claimString(idClaims, 'workspace_id')
  const recordWorkspaceId = firstString(records, ['workspace_id', 'workspaceId'])
  const workspaceId = preferClaimIdentity ? claimedWorkspaceId ?? recordWorkspaceId : recordWorkspaceId ?? claimedWorkspaceId

  // Expiry
  const preserveRawTimeFields = candidate.inputFormat === 'cpa' || candidate.inputFormat === 'codex'
  const recordExpiresAt = normalizeInputTimeValue(firstString(records, ['expires_at', 'expiresAt', 'expired', 'expires']), preserveRawTimeFields)
  const claimedExpiresAt = normalizeTimeValue(firstClaimNumber(expiryClaimRecords, 'exp'))
  const expiresAt = preferClaimIdentity ? claimedExpiresAt ?? recordExpiresAt : recordExpiresAt ?? claimedExpiresAt

  // Last refresh
  const recordLastRefresh = normalizeInputTimeValue(firstString(records, ['last_refresh', 'lastRefresh']), preserveRawTimeFields)
  const claimedLastRefresh = normalizeTimeValue(firstClaimNumber(accessFirstClaimRecords, 'iat'))
  const lastRefresh = preferClaimIdentity ? claimedLastRefresh ?? recordLastRefresh : recordLastRefresh ?? claimedLastRefresh

  // Audience
  const audience = firstClaimStringArray(accessFirstClaimRecords, 'aud')

  // Client ID
  const clientId = firstClaimString(accessFirstClaimRecords, ['client_id'])

  // Scopes
  const scopes = firstClaimStringArray(accessFirstClaimRecords, 'scp')

  // Not before
  const claimedNotBeforeNumber = firstClaimNumber(accessFirstClaimRecords, 'nbf')
  const notBefore = normalizeTimeValue(claimedNotBeforeNumber)

  // ID token synthetic handling
  let idTokenSynthetic = firstBoolean(records, ['id_token_synthetic', 'idTokenSynthetic']) ?? false
  if (idToken && idTokenSynthetic) {
    idToken = applySyntheticIdTokenSignature(idToken)
  }
  if (!idToken) {
    const syntheticClaims = buildSyntheticClaims({
      claims: accessClaims ?? idClaims,
      email,
      name,
      chatgptAccountId,
      chatgptUserId,
      chatgptAccountUserId: buildChatGptAccountUserId(chatgptUserId, chatgptAccountId),
      userId: firstClaimString(accessFirstClaimRecords, ['sub']),
      planType,
      workspaceId,
      expiresAt
    })
    if (syntheticClaims) {
      idToken = createSyntheticIdToken(syntheticClaims)
      idTokenSynthetic = true
      warnings.push(`Generated synthetic id_token for ${candidate.sourceName}`)
    } else {
      warnings.push(`Missing id_token for ${candidate.sourceName}`)
    }
  }

  if (!refreshToken) {
    warnings.push(`Missing refresh_token for ${candidate.sourceName}`)
  }
  if (!accessToken) {
    warnings.push(`Missing access_token for ${candidate.sourceName}`)
  }

  return {
    accessToken,
    refreshToken,
    idToken,
    idTokenSynthetic,
    sessionToken,
    accountId,
    chatgptAccountId,
    chatgptUserId,
    chatgptAccountUserId: buildChatGptAccountUserId(chatgptUserId, chatgptAccountId),
    workspaceId,
    userId: firstClaimString(accessFirstClaimRecords, ['sub']),
    issuer,
    audience,
    clientId,
    scopes,
    notBefore,
    email,
    name,
    planType,
    lastRefresh,
    expiresAt,
    sourceName: candidate.sourceName,
    sourcePath: candidate.sourcePath || `${candidate.sourceName}#${index + 1}`,
    warnings
  }
}

function buildChatGptAccountUserId(userId: string | undefined, accountId: string | undefined): string | undefined {
  if (!userId || !accountId) {
    return undefined
  }
  return `${userId}__${accountId}`
}

function buildSyntheticClaims(input: {
  claims?: Record<string, unknown>
  email?: string
  name?: string
  chatgptAccountId?: string
  chatgptUserId?: string
  chatgptAccountUserId?: string
  userId?: string
  planType?: string
  workspaceId?: string
  expiresAt?: string
}): Record<string, unknown> | undefined {
  const sub = input.chatgptUserId ?? input.userId ?? claimString(input.claims, 'sub')
  const exp = input.expiresAt ? Math.floor(new Date(input.expiresAt).getTime() / 1000) : claimNumber(input.claims, 'exp')
  if (!input.email && !input.chatgptAccountId && !input.chatgptUserId && !sub && !input.planType) {
    return undefined
  }
  const auth: Record<string, unknown> = {}
  if (input.chatgptAccountId) auth.chatgpt_account_id = input.chatgptAccountId
  if (input.planType) auth.chatgpt_plan_type = input.planType
  if (input.chatgptUserId) {
    auth.chatgpt_user_id = input.chatgptUserId
    auth.user_id = input.chatgptUserId
  }
  const chatgptAccountUserId = input.chatgptAccountUserId ?? buildChatGptAccountUserId(input.chatgptUserId, input.chatgptAccountId)
  if (chatgptAccountUserId) auth.chatgpt_account_user_id = chatgptAccountUserId
  if (input.workspaceId) auth.workspace_id = input.workspaceId
  const claims: Record<string, unknown> = {}
  if (Number.isFinite(exp)) claims.exp = exp
  if (sub) claims.sub = sub
  if (input.email) claims.email = input.email
  if (input.name) claims.name = input.name
  if (input.workspaceId) claims.workspace_id = input.workspaceId
  if (Object.keys(auth).length > 0) claims['https://api.openai.com/auth'] = auth
  return claims
}

function normalizeTimeValue(value: unknown): string | undefined {
  if (value === undefined || value === null || value === '') {
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
    return normalizeTimeValue(Number(trimmed))
  }
  const date = new Date(trimmed)
  if (!Number.isNaN(date.valueOf())) {
    return date.toISOString()
  }
  return trimmed
}

function normalizeInputTimeValue(value: string | undefined, preserveRaw: boolean): string | undefined {
  return preserveRaw ? emptyToUndefined(value) : normalizeTimeValue(value)
}

function emptyToUndefined(value: string | undefined): string | undefined {
  const trimmed = value?.trim()
  return trimmed || undefined
}

function firstBoolean(records: Record<string, unknown>[], keys: string[]): boolean | undefined {
  for (const record of records) {
    for (const key of keys) {
      const value = record[key]
      if (typeof value === 'boolean') {
        return value
      }
    }
  }
  return undefined
}

function firstClaimString(records: Record<string, unknown>[], keys: string[]): string | undefined {
  for (const record of records) {
    const value = firstString([record], keys)
    if (value) {
      return value
    }
  }
  return undefined
}

function firstClaimNumber(records: Record<string, unknown>[], key: string): number | undefined {
  for (const record of records) {
    const value = claimNumber(record, key)
    if (value !== undefined) {
      return value
    }
  }
  return undefined
}

function firstClaimStringArray(records: Record<string, unknown>[], key: string): string[] | undefined {
  for (const record of records) {
    const value = claimStringArray(record, key)
    if (value) {
      return value
    }
  }
  return undefined
}

/**
 * Deduplicate accounts by credential content, ignoring source metadata.
 */
export function dedupeAccounts(accounts: NormalizedAccount[]): NormalizedAccount[] {
  const DEDUPE_IGNORED_KEYS = new Set(['sourceName', 'sourcePath', 'warnings', 'inputFormat'])
  const seen = new Set<string>()
  const result: NormalizedAccount[] = []
  for (const account of accounts) {
    const entries = (Object.keys(account) as (keyof NormalizedAccount)[])
      .filter((key) => !DEDUPE_IGNORED_KEYS.has(key as string) && account[key] !== undefined)
      .sort()
      .map((key) => [key, account[key]] as const)
    const key = JSON.stringify(entries)
    if (seen.has(key)) {
      continue
    }
    seen.add(key)
    result.push(account)
  }
  return result
}
