import { beforeEach, describe, expect, it, vi } from 'vitest'

describe('Chinese-only UI locale', () => {
  beforeEach(() => {
    vi.resetModules()
    localStorage.clear()
  })

  it('ignores saved English UI locale and initializes as Chinese', async () => {
    localStorage.setItem('sub2api_locale', 'en')
    const { initI18n, getLocale, i18n, availableLocales } = await import('../index')

    await initI18n()

    expect(getLocale()).toBe('zh')
    expect(i18n.global.locale.value).toBe('zh')
    expect(document.documentElement.getAttribute('lang')).toBe('zh')
    expect(availableLocales).toEqual([{ code: 'zh', name: '中文', flag: '🇨🇳' }])
  })

  it('does not switch the UI locale to English', async () => {
    const { initI18n, setLocale, getLocale, i18n } = await import('../index')

    await initI18n()
    await setLocale('en')

    expect(getLocale()).toBe('zh')
    expect(i18n.global.locale.value).toBe('zh')
    expect(localStorage.getItem('sub2api_locale')).toBe('zh')
  })
})
