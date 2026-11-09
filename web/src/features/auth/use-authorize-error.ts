import { useI18n } from 'vue-i18n'

import { ApiError } from '@/lib/http'

export function useAuthorizeError() {
  const { t } = useI18n()

  function message(error: unknown): string {
    if (!(error instanceof ApiError)) {
      return t('errors.general')
    }
    switch (error.status) {
      case 400:
        return t('auth.authorize.invalid')
      case 401:
        return t('auth.authorize.sessionExpired')
      case 404:
        return t('auth.authorize.expired')
      case 409:
        return t('auth.authorize.conflict')
      case 503:
        return t('errors.unavailable')
      default:
        return t('errors.general')
    }
  }

  return { message }
}
