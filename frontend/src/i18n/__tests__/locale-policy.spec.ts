import { beforeEach, describe, expect, it, vi } from 'vitest'

describe('UI locale policy', () => {
  beforeEach(() => {
    vi.resetModules()
    localStorage.clear()
  })

  it.each([null, 'fr', 'zh'])('defaults a missing or invalid saved locale (%s) to English', async (saved) => {
    if (saved !== null) localStorage.setItem('sub2api_locale', saved)
    const { initI18n, getLocale, i18n, availableLocales } = await import('../index')

    await initI18n()

    expect(getLocale()).toBe('en')
    expect(i18n.global.locale.value).toBe('en')
    expect(localStorage.getItem('sub2api_locale')).toBe('en')
    expect(document.documentElement.getAttribute('lang')).toBe('en')
    expect(availableLocales).toEqual([
      { code: 'en', name: 'English', flag: '🇬🇧' },
      { code: 'zh-CN', name: '中文', flag: '🇨🇳' },
    ])
  })

  it('preserves an explicitly saved Chinese locale', async () => {
    localStorage.setItem('sub2api_locale', 'zh-CN')
    const { initI18n, getLocale, i18n } = await import('../index')

    await initI18n()

    expect(getLocale()).toBe('zh-CN')
    expect(i18n.global.locale.value).toBe('zh-CN')
    expect(document.documentElement.getAttribute('lang')).toBe('zh-CN')
  })

  it('switches between lazily loaded English and Chinese catalogs', async () => {
    const { initI18n, setLocale, getLocale, i18n } = await import('../index')

    await initI18n()
    expect((i18n.global.getLocaleMessage('en').common as Record<string, string>).save).toBe('Save')

    await setLocale('zh-CN')
    expect(getLocale()).toBe('zh-CN')
    expect((i18n.global.getLocaleMessage('zh-CN').common as Record<string, string>).save).toBe('保存')
    expect(localStorage.getItem('sub2api_locale')).toBe('zh-CN')

    await setLocale('not-supported')
    expect(getLocale()).toBe('en')
    expect((i18n.global.getLocaleMessage('en').common as Record<string, string>).save).toBe('Save')
    expect(localStorage.getItem('sub2api_locale')).toBe('en')
  })
})
