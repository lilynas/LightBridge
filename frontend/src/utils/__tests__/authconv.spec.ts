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
