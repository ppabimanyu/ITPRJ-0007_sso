import { sessionFetch } from './csrf'
import { apiFetch } from './http'
import {
  readNullableString,
  readString,
  requireArray,
  requireBoolean,
  requireRecord,
  requireString,
} from './json'

const REQUEST_ID_MAX_LENGTH = 128

export type AuthorizeRequestId = string & { readonly __brand: 'AuthorizeRequestId' }

export function parseAuthorizeRequestId(value: unknown): AuthorizeRequestId | null {
  if (typeof value !== 'string') {
    return null
  }
  const trimmed = value.trim()
  if (trimmed === '' || trimmed.length > REQUEST_ID_MAX_LENGTH) {
    return null
  }
  return trimmed as AuthorizeRequestId
}

export interface AuthorizeRequestInfo {
  requestId: AuthorizeRequestId
  clientId: string
  clientName: string
  scopes: string[]
  prompt: string | null
  loginHint: string | null
  loginRequired: boolean
  consentRequired: boolean
}

function parseScopes(record: Record<string, unknown>): string[] {
  return requireArray(record, 'scopes').map((scope) => {
    if (typeof scope !== 'string' || scope === '') {
      throw new Error('SSO API returned an invalid scope')
    }
    return scope
  })
}

function parseOptionalString(record: Record<string, unknown>, key: string): string | null {
  const value = readNullableString(record, key)
  return value === null || value === '' ? null : value
}

export function parseAuthorizeRequestInfo(value: unknown): AuthorizeRequestInfo {
  const record = requireRecord(value, 'authorize request')
  const requestId = parseAuthorizeRequestId(requireString(record, 'request_id'))
  if (requestId === null) {
    throw new Error('SSO API returned an invalid request id')
  }
  return {
    requestId,
    clientId: readString(record, 'client_id'),
    clientName: requireString(record, 'client_name'),
    scopes: parseScopes(record),
    prompt: readNullableString(record, 'prompt'),
    loginHint: parseOptionalString(record, 'login_hint'),
    loginRequired: requireBoolean(record, 'login_required'),
    consentRequired: requireBoolean(record, 'consent_required'),
  }
}

function parseRedirectTo(value: unknown): string {
  const record = requireRecord(value, 'authorize completion')
  const raw = requireString(record, 'redirect_to')
  let url: URL
  try {
    url = new URL(raw)
  } catch {
    throw new Error('SSO API returned an invalid redirect URI')
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    throw new Error('SSO API returned an unsupported redirect URI')
  }
  return raw
}

export async function fetchAuthorizeRequest(
  requestId: AuthorizeRequestId,
): Promise<AuthorizeRequestInfo> {
  const query = encodeURIComponent(requestId)
  const payload = await apiFetch(`/api/v1/auth/authorize/request?request_id=${query}`, {
    redirectOnSessionExpired: false,
  })
  return parseAuthorizeRequestInfo(payload)
}

export async function completeAuthorize(requestId: AuthorizeRequestId): Promise<string> {
  const payload = await sessionFetch('/api/v1/auth/authorize/complete', {
    method: 'POST',
    body: { request_id: requestId },
  })
  return parseRedirectTo(payload)
}

export interface ConsentInput {
  requestId: AuthorizeRequestId
  scopes: string[]
}

export async function grantConsent(input: ConsentInput): Promise<string> {
  const payload = await sessionFetch('/api/v1/auth/consent', {
    method: 'POST',
    body: {
      request_id: input.requestId,
      scopes: input.scopes,
    },
  })
  return parseRedirectTo(payload)
}
