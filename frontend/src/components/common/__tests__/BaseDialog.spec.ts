import { defineComponent, nextTick, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'

import BaseDialog from '../BaseDialog.vue'

afterEach(() => {
  document.body.innerHTML = ''
  document.body.classList.remove('modal-open')
})

describe('BaseDialog focus management', () => {
  it('keeps Escape scoped to the topmost nested dialog', async () => {
    const Host = defineComponent({
      components: { BaseDialog },
      setup() {
        const outer = ref(true)
        const inner = ref(true)
        return { outer, inner }
      },
      template: `
        <BaseDialog :show="outer" title="Outer" @close="outer = false">
          <button id="outer-button">Outer</button>
          <BaseDialog :show="inner" title="Inner" :z-index="60" @close="inner = false">
            <button id="inner-button">Inner</button>
          </BaseDialog>
        </BaseDialog>
      `,
    })
    const wrapper = mount(Host, { attachTo: document.body, global: { stubs: { Icon: true } } })
    await nextTick()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()

    expect((wrapper.vm as unknown as { inner: boolean }).inner).toBe(false)
    expect((wrapper.vm as unknown as { outer: boolean }).outer).toBe(true)
    expect(document.body.classList.contains('modal-open')).toBe(true)
    wrapper.unmount()
  })

  it('wraps Tab focus inside the active dialog', async () => {
    const wrapper = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: true, title: 'Dialog', showCloseButton: false },
      slots: { default: '<button id="first">First</button><button id="last">Last</button>' },
      global: { stubs: { Icon: true } },
    })
    await nextTick()
    const first = document.getElementById('first') as HTMLButtonElement
    const last = document.getElementById('last') as HTMLButtonElement
    last.focus()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }))
    expect(document.activeElement).toBe(first)
    wrapper.unmount()
  })

  it('does not initially focus aria-hidden or tabindex=-1 inputs', async () => {
    const wrapper = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: true, title: 'Dialog', showCloseButton: false },
      slots: {
        default: '<input id="otp-autofill" aria-hidden="true" tabindex="-1"><input id="visible-code" aria-label="Code">',
      },
      global: { stubs: { Icon: true } },
    })
    await nextTick()

    expect(document.activeElement).toBe(document.getElementById('visible-code'))
    wrapper.unmount()
  })
})
