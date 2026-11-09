import { ApiError, apiFetch, isApiError, notifySessionExpired } from './http'
import { requireRecord, requireString } from './json'

const CSRF_COOKIE = 'sso_csrf'

let cachedToken: string | null = null
let pendingToken: Promise<string> | null = null

function parseCsrfToken(value: unknown): string {
  const record = requireRecord(value, 'CSRF')
  const token = requireString(record, 'csrf_token')
  if (token === '') {
    throw new Error('SSO API returned an empty CSRF token')
  }
  return token
}

export function clearCsrfToken() {
  cachedToken = null
  pendingToken = null
}

export function readCsrfCookie(): string | null {
  for (const part of document.cookie.split(';')) {
    const entry = part.trim()
    if (!entry.startsWith(`${CSRF_COOKIE}=`)) {
      continue
    }
    const value = entry.slice(CSRF_COOKIE.length + 1)
    return value === '' ? null : decodeURIComponent(value)
  }
  return null
}

export async function getCsrfToken(options: { force?: boolean } = {}): Promise<string> {
  if (!options.force && cachedToken !== null) {
    return cachedToken
  }
  if (!options.force && pendingToken !== null) {
    return pendingToken
  }

  const request = apiFetch('/api/v1/auth/csrf').then(parseCsrfToken)
  pendingToken = request
  try {
    const token = await request
    cachedToken = token
    return token
  } finally {
    if (pendingToken === request) {
      pendingToken = null
    }
  }
}

async function sendPreSession(path: string, body: unknown): Promise<unknown> {
  const token = await getCsrfToken({ force: true })
  return apiFetch(path, { method: 'POST', body, csrfToken: token })
}

export async function preSessionFetch(path: string, body: unknown): Promise<unknown> {
  try {
    const result = await sendPreSession(path, body)
    clearCsrfToken()
    return result
  } catch (error) {
    if (isApiError(error) && error.status === 403 && error.code === 'csrf_invalid') {
      clearCsrfToken()
      const result = await sendPreSession(path, body)
      clearCsrfToken()
      return result
    }
    throw error
  }
}

interface SessionWrite {
  method: 'POST' | 'PATCH' | 'DELETE'
  body?: unknown
  headers?: Readonly<Record<string, string>>
}

export async function sessionFetch(path: string, options: SessionWrite): Promise<unknown> {
  const token = readCsrfCookie()
  if (token === null) {
    notifySessionExpired()
    throw new ApiError({
      status: 403,
      code: 'csrf_invalid',
      detail: 'csrf token is missing',
    })
  }
  try {
    return await apiFetch(path, { ...options, csrfToken: token })
  } catch (error) {
    if (isApiError(error) && error.code === 'csrf_invalid') {
      notifySessionExpired()
    }
    throw error
  }
}
