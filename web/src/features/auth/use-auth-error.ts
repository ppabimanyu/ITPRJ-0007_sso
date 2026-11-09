import { useI18n } from 'vue-i18n'

import { ApiError } from '@/lib/http'

interface MessageOverrides {
  unauthorized?: string
  badRequest?: string
}

export function useAuthError() {
  const { t } = useI18n()

  function retryAfterText(error: ApiError): string {
    if (error.retryAfterSeconds === null) {
      return ''
    }
    const seconds = Math.max(1, error.retryAfterSeconds)
    if (seconds >= 60) {
      return ` ${t('errors.retryAfterMinutes', { count: Math.ceil(seconds / 60) })}`
    }
    return ` ${t('errors.retryAfterSeconds', { count: seconds })}`
  }

  function generalMessage(error: unknown, overrides: MessageOverrides = {}): string {
    if (!(error instanceof ApiError)) {
      return t('errors.general')
    }
    switch (error.status) {
      case 400:
        return t(overrides.badRequest ?? 'errors.badRequest')
      case 401:
        return t(overrides.unauthorized ?? 'errors.unauthorized')
      case 403:
        return error.code === 'csrf_invalid' ? t('errors.csrf') : t('errors.forbidden')
      case 404:
        return t('errors.notFound')
      case 422:
        return t('errors.validation')
      case 423:
        return `${t('errors.locked')}${retryAfterText(error)}`
      case 429:
        return `${t('errors.rateLimited')}${retryAfterText(error)}`
      case 503:
        return t('errors.unavailable')
      default:
        return t('errors.general')
    }
  }

  function fieldErrors(error: unknown): Readonly<Record<string, string>> {
    return error instanceof ApiError ? error.fieldErrors : {}
  }

  return { generalMessage, fieldErrors }
}
