import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'

export type AppLocale = 'zh-CN' | 'en'

type I18nContextValue = {
  locale: AppLocale
  setLocale: (locale: AppLocale) => void
  tx: (zh: string, en: string) => string
  formatDateTime: (value?: string, options?: Intl.DateTimeFormatOptions) => string
  localeTag: string
  isZh: boolean
}

const STORAGE_KEY = 'vola.locale'
const MANUAL_STORAGE_KEY = 'vola.locale.manual'

const I18nContext = createContext<I18nContextValue | null>(null)

export function normalizeLocale(value?: string | null): AppLocale {
  const normalized = (value || '').trim().toLowerCase()
  if (normalized.startsWith('zh')) return 'zh-CN'
  return 'en'
}

export function getLocaleTag(locale: AppLocale) {
  return locale === 'zh-CN' ? 'zh-CN' : 'en-US'
}

function detectInitialLocale(): AppLocale {
  if (typeof window === 'undefined') return 'zh-CN'

  const manuallySelected = window.localStorage.getItem(MANUAL_STORAGE_KEY) === '1'
  const stored = manuallySelected ? window.localStorage.getItem(STORAGE_KEY) : null
  if (stored) return normalizeLocale(stored)

  return 'zh-CN'
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocale] = useState<AppLocale>(detectInitialLocale)
  const isZh = locale === 'zh-CN'
  const localeTag = getLocaleTag(locale)

  const updateLocale = useCallback((nextLocale: AppLocale) => {
    window.localStorage.setItem(MANUAL_STORAGE_KEY, '1')
    setLocale(nextLocale)
  }, [])

  useEffect(() => {
    window.localStorage.setItem(STORAGE_KEY, locale)
    document.documentElement.lang = isZh ? 'zh-CN' : 'en'
  }, [isZh, locale])

  const value: I18nContextValue = {
    locale,
    setLocale: updateLocale,
    tx: (zh, en) => (isZh ? zh : en),
    formatDateTime: (rawValue, options) => {
      if (!rawValue) return '-'

      try {
        const value = new Date(rawValue)
        if (Number.isNaN(value.getTime())) return rawValue
        return new Intl.DateTimeFormat(localeTag, options ?? {
          year: 'numeric',
          month: 'numeric',
          day: 'numeric',
          hour: 'numeric',
          minute: '2-digit',
        }).format(value)
      } catch {
        return rawValue
      }
    },
    localeTag,
    isZh,
  }

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n() {
  const context = useContext(I18nContext)
  if (!context) {
    throw new Error('useI18n must be used within I18nProvider')
  }
  return context
}
