import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import PlatformTypeBadge from '../PlatformTypeBadge.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

describe('PlatformTypeBadge', () => {
  it('does not mislabel an unknown platform as Gemini', () => {
    const wrapper = mount(PlatformTypeBadge, {
      props: {
        platform: 'module' as any,
        type: 'oauth'
      },
      global: {
        stubs: {
          PlatformIcon: true,
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('module')
    expect(wrapper.text()).not.toContain('Gemini')
  })

  it('keeps the Gemini label for the Gemini platform', () => {
    const wrapper = mount(PlatformTypeBadge, {
      props: {
        platform: 'gemini',
        type: 'oauth'
      },
      global: {
        stubs: {
          PlatformIcon: true,
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('Gemini')
  })
})
