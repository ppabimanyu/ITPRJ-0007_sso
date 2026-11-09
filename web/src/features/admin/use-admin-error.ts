import { useI18n } from 'vue-i18n'

import { ApiError } from '@/lib/http'

export interface AdminProblem {
  title: string
  message: string
  detail: string
}

export function useAdminError() {
  const { t } = useI18n()

  function retryAfterText(error: ApiError): string {
    if (error.retryAfterSeconds === null) {
      return ''
    }
    const seconds = Math.max(1, error.retryAfterSeconds)
    if (seconds >= 60) {
      return ` ${t('admin.errors.retryAfterMinutes', { count: Math.ceil(seconds / 60) })}`
    }
    return ` ${t('admin.errors.retryAfterSeconds', { count: seconds })}`
  }

  function describe(error: unknown): AdminProblem {
    const title = t('admin.errors.title')
    if (!(error instanceof ApiError)) {
      return { title, message: t('admin.errors.general'), detail: '' }
    }
    const detail = error.detail
    switch (error.status) {
      case 401:
        return { title, message: t('admin.errors.unauthorized'), detail }
      case 403:
        return {
          title,
          message:
            error.code === 'mfa_required'
              ? t('admin.errors.mfaRequired')
              : t('admin.errors.forbidden'),
          detail,
        }
      case 404:
        return { title, message: t('admin.errors.notFound'), detail }
      case 409:
        return { title, message: conflictMessage(detail), detail }
      case 422:
        return { title, message: t('admin.errors.validation'), detail }
      case 429:
        return {
          title,
          message: `${t('admin.errors.rateLimited')}${retryAfterText(error)}`,
          detail,
        }
      case 503:
        return {
          title,
          message: `${t('admin.errors.unavailable')}${retryAfterText(error)}`,
          detail,
        }
      default:
        return { title, message: t('admin.errors.general'), detail }
    }
  }

  function conflictMessage(detail: string): string {
    const normalized = detail.toLowerCase()
    if (normalized.includes('last platform administrator')) {
      return t('admin.errors.lastAdmin')
    }
    if (normalized.includes('active session or consent')) {
      return t('admin.errors.clientInUse')
    }
    if (normalized.includes('already exists')) {
      return t('admin.errors.duplicate')
    }
    return t('admin.errors.conflict')
  }

  function fieldErrors(error: unknown): Readonly<Record<string, string>> {
    return error instanceof ApiError ? error.fieldErrors : {}
  }

  return { describe, fieldErrors }
}
