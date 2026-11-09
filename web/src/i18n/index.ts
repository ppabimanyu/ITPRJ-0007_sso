import { createI18n } from 'vue-i18n'

import en from './messages/en'
import id from './messages/id'

export type AppLocale = 'en' | 'id'

const LOCALE_STORAGE_KEY = 'sso.locale'

export function isAppLocale(value: unknown): value is AppLocale {
  return value === 'en' || value === 'id'
}

function storedLocale(): AppLocale | null {
  try {
    const value = window.localStorage.getItem(LOCALE_STORAGE_KEY)
    return isAppLocale(value) ? value : null
  } catch {
    return null
  }
}

function detectLocale(): AppLocale {
  const stored = storedLocale()
  if (stored !== null) {
    return stored
  }
  const languages = navigator.languages.length > 0 ? navigator.languages : [navigator.language]
  return languages.some((language) => language.toLowerCase().startsWith('id')) ? 'id' : 'en'
}

const initialLocale = detectLocale()

export const i18n = createI18n({
  legacy: false,
  locale: initialLocale,
  fallbackLocale: 'en',
  messages: { en, id },
})

export function setLocale(locale: AppLocale) {
  i18n.global.locale.value = locale
  document.documentElement.lang = locale
  try {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, locale)
  } catch {
    // A blocked storage API only loses the preference across reloads.
  }
}

document.documentElement.lang = initialLocale
