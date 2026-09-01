import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import ConfirmDialog from '../ConfirmDialog.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const mountDialog = (loading: boolean) => mount(ConfirmDialog, {
  props: {
    show: true,
    title: 'Delete group',
    message: 'This cannot be undone.',
    confirmText: 'Delete',
    loading,
  },
  global: {
    stubs: {
      BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
    },
  },
})

describe('ConfirmDialog loading state', () => {
  it('disables both actions and suppresses events while loading', async () => {
    const wrapper = mountDialog(true)
    const [cancel, confirm] = wrapper.findAll('button')

    expect(cancel.attributes('disabled')).toBeDefined()
    expect(confirm.attributes('disabled')).toBeDefined()
    expect(confirm.attributes('aria-busy')).toBe('true')
    expect(confirm.text()).toBe('common.processing')

    await cancel.trigger('click')
    await confirm.trigger('click')
    expect(wrapper.emitted('cancel')).toBeUndefined()
    expect(wrapper.emitted('confirm')).toBeUndefined()
  })

  it('emits actions normally when idle', async () => {
    const wrapper = mountDialog(false)
    const [cancel, confirm] = wrapper.findAll('button')

    await cancel.trigger('click')
    await confirm.trigger('click')
    expect(wrapper.emitted('cancel')).toHaveLength(1)
    expect(wrapper.emitted('confirm')).toHaveLength(1)
  })
})
