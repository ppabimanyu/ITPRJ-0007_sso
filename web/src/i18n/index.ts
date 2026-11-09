import { createI18n } from 'vue-i18n'

import en from './messages/en'
import id from './messages/id'

export type AppLocale = 'en' | 'id'

function detectLocale(): AppLocale {
  const languages = navigator.languages.length > 0 ? navigator.languages : [navigator.language]
  return languages.some((language) => language.toLowerCase().startsWith('id')) ? 'id' : 'en'
}

export const i18n = createI18n({
  legacy: false,
  locale: detectLocale(),
  fallbackLocale: 'en',
  messages: { en, id },
})
