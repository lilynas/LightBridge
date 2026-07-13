import { describe, expect, it } from 'vitest'

import { convertFromPayload, convertToPayload } from '@/utils/authconv'
import type { AdminDataPayload } from '@/types'

describe('authconv LightBridge bridge', () => {
  it('preserves expires_at seconds and chatgpt_user_id when exporting non-native formats', () => {
    const payload: AdminDataPayload = {
      type: 'lightbridge',
      version: 1,
      exported_at: '2026-01-01T00:00:00.000Z',
      proxies: [],
      accounts: [
        {
          name: 'codex@example.com',
          platform: 'openai',
          type: 'oauth',
          credentials: {
            refresh_token: 'rt',
            chatgpt_account_id: 'acc_1',
            chatgpt_user_id: 'user_1',
          },
          concurrency: 10,
          priority: 1,
          expires_at: 1893456000,
        },
      ],
    }

    const exported = convertFromPayload(payload, 'sub2api') as any
    expect(exported.accounts[0].credentials.expires_at).toBe('2030-01-01T00:00:00.000Z')
    expect(exported.accounts[0].credentials.chatgpt_user_id).toBe('user_1')
  })

  it('preserves chatgpt_user_id when importing to LightBridge native payload', () => {
    const converted = convertToPayload({
      refresh_token: 'rt',
      session_token: 'st',
      chatgpt_account_id: 'acc_1',
      chatgpt_user_id: 'user_1',
    })

    expect(converted?.accounts[0].credentials.chatgpt_user_id).toBe('user_1')
  })

  it('honors explicit input format overrides', () => {
    const converted = convertToPayload(
      {
        credentials: {
          refresh_token: 'rt',
          session_token: 'st',
        },
        platform: 'openai',
      },
      { inputFormat: 'codex2api' }
    )

    expect(converted?.accounts[0].credentials.refresh_token).toBe('rt')
    expect(converted?.accounts[0].credentials.session_token).toBe('st')
  })
})

describe('authconv CPA xAI Grok compatibility', () => {
  it('imports a CPA xAI auth file as a native Grok OAuth account', () => {
    const converted = convertToPayload({
      type: 'xai',
      access_token: 'xai-access',
      refresh_token: 'xai-refresh',
      id_token: 'xai-id',
      token_type: 'Bearer',
      expires_in: 3600,
      expired: '2030-01-01T00:00:00.000Z',
      last_refresh: '2029-12-31T23:00:00.000Z',
      email: 'grok@example.com',
      sub: 'xai-subject-1',
      base_url: 'https://api.x.ai/v1',
      token_endpoint: 'https://auth.x.ai/oauth2/token',
      auth_kind: 'oauth',
      using_api: false,
      subscription_tier: 'SUPER_GROK',
    })

    expect(converted?.accounts).toHaveLength(1)
    expect(converted?.accounts[0]).toMatchObject({
      name: 'grok@example.com',
      platform: 'grok',
      type: 'oauth',
      concurrency: 1,
      credentials: {
        access_token: 'xai-access',
        refresh_token: 'xai-refresh',
        id_token: 'xai-id',
        token_type: 'Bearer',
        expires_in: 3600,
        email: 'grok@example.com',
        sub: 'xai-subject-1',
        base_url: 'https://api.x.ai/v1',
        token_endpoint: 'https://auth.x.ai/oauth2/token',
        auth_kind: 'oauth',
        using_api: false,
        subscription_tier: 'SUPER_GROK',
      },
    })
    expect(converted?.accounts[0].expires_at).toBe(1893456000)
  })

  it('accepts the legacy grok CPA type alias without generating an OpenAI id token', () => {
    const converted = convertToPayload({
      type: 'grok',
      access_token: 'xai-access',
      refresh_token: 'xai-refresh',
      email: 'legacy@example.com',
      sub: 'legacy-subject',
    })

    expect(converted?.accounts[0].platform).toBe('grok')
    expect(converted?.accounts[0].credentials.id_token).toBeUndefined()
    expect(converted?.accounts[0].extra?.id_token_synthetic).toBeUndefined()
  })

  it('exports a native Grok account to the CPA xAI auth-file shape', () => {
    const payload: AdminDataPayload = {
      type: 'LightBridge-data',
      version: 1,
      exported_at: '2026-01-01T00:00:00.000Z',
      proxies: [],
      accounts: [
        {
          name: 'grok@example.com',
          platform: 'grok',
          type: 'oauth',
          credentials: {
            access_token: 'xai-access',
            refresh_token: 'xai-refresh',
            id_token: 'xai-id',
            token_type: 'Bearer',
            expires_in: 3600,
            expires_at: '2030-01-01T00:00:00.000Z',
            last_refresh: '2029-12-31T23:00:00.000Z',
            email: 'grok@example.com',
            sub: 'xai-subject-1',
            base_url: 'https://cli-chat-proxy.grok.com/v1',
            token_endpoint: 'https://auth.x.ai/oauth2/token',
            auth_kind: 'oauth',
            using_api: false,
            subscription_tier: 'SUPER_GROK',
          },
          concurrency: 1,
          priority: 1,
          expires_at: 1893456000,
        },
      ],
    }

    const exported = convertFromPayload(payload, 'cpa') as any
    expect(exported).toEqual({
      type: 'xai',
      access_token: 'xai-access',
      refresh_token: 'xai-refresh',
      auth_kind: 'oauth',
      disabled: false,
      id_token: 'xai-id',
      token_type: 'Bearer',
      expires_in: 3600,
      expired: '2030-01-01T00:00:00.000Z',
      last_refresh: '2029-12-31T23:00:00.000Z',
      email: 'grok@example.com',
      sub: 'xai-subject-1',
      base_url: 'https://cli-chat-proxy.grok.com/v1',
      token_endpoint: 'https://auth.x.ai/oauth2/token',
      using_api: false,
      subscription_tier: 'SUPER_GROK',
    })
  })

  it('keeps OpenAI and Grok account types distinct in a mixed CPA export', () => {
    const payload: AdminDataPayload = {
      exported_at: '2026-01-01T00:00:00.000Z',
      proxies: [],
      accounts: [
        {
          name: 'codex@example.com',
          platform: 'openai',
          type: 'oauth',
          credentials: { access_token: 'oa', refresh_token: 'or' },
          concurrency: 10,
          priority: 1,
        },
        {
          name: 'grok@example.com',
          platform: 'grok',
          type: 'oauth',
          credentials: { access_token: 'xa', refresh_token: 'xr', email: 'grok@example.com' },
          concurrency: 1,
          priority: 1,
        },
      ],
    }

    const exported = convertFromPayload(payload, 'cpa') as any[]
    expect(exported.map((item) => item.type)).toEqual(['codex', 'xai'])
  })

  it('preserves Grok platform and single-concurrency semantics in sub2api export', () => {
    const payload: AdminDataPayload = {
      exported_at: '2026-01-01T00:00:00.000Z',
      proxies: [],
      accounts: [
        {
          name: 'grok@example.com',
          platform: 'grok',
          type: 'oauth',
          credentials: {
            access_token: 'xa',
            refresh_token: 'xr',
            email: 'grok@example.com',
            sub: 'subject-1',
            using_api: false,
          },
          concurrency: 1,
          priority: 1,
        },
      ],
    }

    const exported = convertFromPayload(payload, 'sub2api') as any
    expect(exported.accounts[0]).toMatchObject({
      platform: 'grok',
      concurrency: 1,
      credentials: {
        sub: 'subject-1',
        using_api: false,
      },
    })
  })
})
