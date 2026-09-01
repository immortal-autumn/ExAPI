import { createI18n } from 'vue-i18n'

export type LocaleCode = 'en' | 'zh-CN'

type LocaleMessages = Record<string, any>

const LOCALE_KEY = 'sub2api_locale'
const DEFAULT_LOCALE: LocaleCode = 'en'

const localeLoaders: Record<LocaleCode, () => Promise<{ default: LocaleMessages }>> = {
  en: () => import('./locales/en'),
  'zh-CN': () => import('./locales/zh'),
}

function isLocaleCode(value: unknown): value is LocaleCode {
  return value === 'en' || value === 'zh-CN'
}

function getDefaultLocale(): LocaleCode {
  const savedLocale = localStorage.getItem(LOCALE_KEY)
  if (isLocaleCode(savedLocale)) {
    return savedLocale
  }
  return DEFAULT_LOCALE
}

export const i18n = createI18n({
  legacy: false,
  locale: getDefaultLocale(),
  fallbackLocale: DEFAULT_LOCALE,
  messages: {},
  // Keep HTML message warnings disabled for trusted admin help content.
  // 这些内容是内部定义的，不存在 XSS 风险
  warnHtmlMessage: false
})

const loadedLocales = new Set<LocaleCode>()

export async function loadLocaleMessages(locale: LocaleCode): Promise<void> {
  if (loadedLocales.has(locale)) {
    return
  }

  const loader = localeLoaders[locale]
  const module = await loader()
  i18n.global.setLocaleMessage(locale, module.default)
  loadedLocales.add(locale)
}

export async function initI18n(): Promise<void> {
  const current = getLocale()
  await loadLocaleMessages(current)
  localStorage.setItem(LOCALE_KEY, current)
  document.documentElement.setAttribute('lang', current)
}

export async function setLocale(locale: string): Promise<void> {
  const nextLocale = isLocaleCode(locale) ? locale : DEFAULT_LOCALE
  await loadLocaleMessages(nextLocale)
  i18n.global.locale.value = nextLocale
  localStorage.setItem(LOCALE_KEY, nextLocale)
  document.documentElement.setAttribute('lang', nextLocale)
}

export function getLocale(): LocaleCode {
  const current = i18n.global.locale.value
  return isLocaleCode(current) ? current : DEFAULT_LOCALE
}

export const availableLocales = [
  { code: 'en', name: 'English', flag: '🇬🇧' },
  { code: 'zh-CN', name: '中文', flag: '🇨🇳' },
] as const

export default i18n
