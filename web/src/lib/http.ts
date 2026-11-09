import { isRecord, readString } from './json'

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL?.replace(/\/$/, '') ?? ''

export interface ProblemFieldError {
  field: string
  message: string
}

interface ApiErrorInput {
  status: number
  code?: string
  detail?: string
  fields?: readonly ProblemFieldError[]
  retryAfterSeconds?: number | null
}

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly detail: string
  readonly fieldErrors: Readonly<Record<string, string>>
  readonly retryAfterSeconds: number | null

  constructor(input: ApiErrorInput) {
    super(input.detail || input.code || `HTTP ${input.status}`)
    this.name = 'ApiError'
    this.status = input.status
    this.code = input.code ?? ''
    this.detail = input.detail ?? ''
    this.retryAfterSeconds = input.retryAfterSeconds ?? null
    this.fieldErrors = Object.freeze(
      Object.fromEntries((input.fields ?? []).map((field) => [field.field, field.message])),
    )
  }
}

export function isApiError(value: unknown): value is ApiError {
  return value instanceof ApiError
}

type SessionExpiredHandler = () => void

let sessionExpiredHandler: SessionExpiredHandler | null = null

export function setSessionExpiredHandler(handler: SessionExpiredHandler | null) {
  sessionExpiredHandler = handler
}

export function notifySessionExpired() {
  sessionExpiredHandler?.()
}

export type ApiMethod = 'GET' | 'POST' | 'PATCH' | 'DELETE'

export interface ApiRequest {
  method?: ApiMethod
  body?: unknown
  csrfToken?: string | null
  redirectOnSessionExpired?: boolean
}

function parseFieldErrors(value: unknown): ProblemFieldError[] {
  if (!Array.isArray(value)) {
    return []
  }
  const fields: ProblemFieldError[] = []
  for (const item of value) {
    if (!isRecord(item)) {
      continue
    }
    const field = readString(item, 'field')
    const message = readString(item, 'message')
    if (field !== '' && message !== '') {
      fields.push({ field, message })
    }
  }
  return fields
}

function parseRetryAfter(response: Response): number | null {
  const raw = response.headers.get('Retry-After')
  if (raw === null || raw.trim() === '') {
    return null
  }
  const seconds = Number(raw)
  if (Number.isFinite(seconds) && seconds >= 0) {
    return Math.ceil(seconds)
  }
  const date = Date.parse(raw)
  if (Number.isNaN(date)) {
    return null
  }
  return Math.max(0, Math.ceil((date - Date.now()) / 1000))
}

async function readProblem(response: Response): Promise<ApiError> {
  let payload: unknown = null
  try {
    payload = await response.json()
  } catch {
    payload = null
  }
  const problem = isRecord(payload) ? payload : {}
  return new ApiError({
    status: response.status,
    code: readString(problem, 'code'),
    detail: readString(problem, 'detail'),
    fields: parseFieldErrors(problem.errors),
    retryAfterSeconds: parseRetryAfter(response),
  })
}

export async function apiFetch(path: string, options: ApiRequest = {}): Promise<unknown> {
  const method = options.method ?? 'GET'
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (options.body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }
  if (options.csrfToken) {
    headers['X-CSRF-Token'] = options.csrfToken
  }

  const response = await fetch(`${apiBaseUrl}${path}`, {
    method,
    credentials: 'include',
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  })

  if (!response.ok) {
    const error = await readProblem(response)
    if (
      error.status === 401 &&
      error.code === 'session_expired' &&
      options.redirectOnSessionExpired !== false
    ) {
      notifySessionExpired()
    }
    throw error
  }

  if (response.status === 204 || response.status === 202) {
    return undefined
  }
  const contentType = response.headers.get('Content-Type') ?? ''
  if (!contentType.includes('json')) {
    return undefined
  }
  const text = await response.text()
  if (text === '') {
    return undefined
  }
  const payload: unknown = JSON.parse(text)
  return payload
}
