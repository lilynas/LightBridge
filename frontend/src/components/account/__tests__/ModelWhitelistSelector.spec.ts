import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { syncUpstreamModelsMock } = vi.hoisted(() => ({
  syncUpstreamModelsMock: vi.fn()
}))

vi.mock('@/api/admin/accounts', () => ({
  accountsAPI: {
    syncUpstreamModels: syncUpstreamModelsMock
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showInfo: vi.fn(),
    showSuccess: vi.fn()
  })
}))

vi.mock('@/composables/useModelWhitelist', () => ({
  allModels: [],
  getModelsByPlatform: () => []
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

import ModelWhitelistSelector from '../ModelWhitelistSelector.vue'

describe('ModelWhitelistSelector', () => {
  it('uses transient Custom discovery before an account ID exists', async () => {
    const discoverModels = vi.fn().mockResolvedValue(['model-b', 'model-a'])
    const wrapper = mount(ModelWhitelistSelector, {
      props: {
        modelValue: [],
        platform: 'custom',
        discoverModels
      },
      global: {
        stubs: {
          ModelIcon: true,
          Icon: true
        }
      }
    })

    const pullButton = wrapper.findAll('button').find((button) =>
      button.text().includes('admin.accounts.syncUpstreamModels')
    )
    expect(pullButton).toBeDefined()

    await pullButton!.trigger('click')
    await flushPromises()

    expect(discoverModels).toHaveBeenCalledOnce()
    expect(syncUpstreamModelsMock).not.toHaveBeenCalled()
    expect(wrapper.emitted('update:modelValue')).toEqual([[['model-b', 'model-a']]])
  })
})
