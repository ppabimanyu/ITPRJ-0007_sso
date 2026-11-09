import { preSessionFetch, sessionFetch } from './csrf'
import { apiFetch, isApiError } from './http'
import {
  readNullableString,
  requireArray,
  requireBoolean,
  requireRecord,
  requireString,
} from './json'

export interface RoleRef {
  id: string
  name: string
}

export interface GroupRef {
  id: string
  name: string
}

export interface User {
  id: string
  email: string
  username: string | null
  name: string | null
  givenName: string | null
  familyName: string | null
  locale: string | null
  emailVerifiedAt: string | null
  disabledAt: string | null
  mfaEnrolled: boolean
  roles: RoleRef[]
  groups: GroupRef[]
  createdAt: string
  updatedAt: string
}

export interface SessionInfo {
  user: User
  expiresAt: string
  idleExpiresAt: string
  mfaEnrolled: boolean
  isAdmin: boolean
}

export interface MeSession {
  id: string
  ip: string | null
  userAgent: string | null
  createdAt: string
  lastSeenAt: string
  expiresAt: string
  current: boolean
}

export type LoginStatus = 'authenticated' | 'mfa_required' | 'mfa_enrollment_required'

export interface LoginResult {
  status: LoginStatus
  mfaToken: string | null
  sessionExpiresAt: string | null
  user: User | null
  redirectTo: string | null
}

export interface LoginInput {
  email: string
  password: string
  rememberMe: boolean
}

export interface ProfileInput {
  name: string
  givenName: string
  familyName: string
  locale: 'en' | 'id'
}

export interface PasswordInput {
  currentPassword: string
  newPassword: string
}

function parseRoleRef(value: unknown): RoleRef {
  const record = requireRecord(value, 'role')
  return {
    id: requireString(record, 'id'),
    name: requireString(record, 'name'),
  }
}

function parseGroupRef(value: unknown): GroupRef {
  const record = requireRecord(value, 'group')
  return {
    id: requireString(record, 'id'),
    name: requireString(record, 'name'),
  }
}

export function parseUser(value: unknown): User {
  const record = requireRecord(value, 'user')
  return {
    id: requireString(record, 'id'),
    email: requireString(record, 'email'),
    username: readNullableString(record, 'username'),
    name: readNullableString(record, 'name'),
    givenName: readNullableString(record, 'given_name'),
    familyName: readNullableString(record, 'family_name'),
    locale: readNullableString(record, 'locale'),
    emailVerifiedAt: readNullableString(record, 'email_verified_at'),
    disabledAt: readNullableString(record, 'disabled_at'),
    mfaEnrolled: requireBoolean(record, 'mfa_enrolled'),
    roles: requireArray(record, 'roles').map(parseRoleRef),
    groups: requireArray(record, 'groups').map(parseGroupRef),
    createdAt: requireString(record, 'created_at'),
    updatedAt: requireString(record, 'updated_at'),
  }
}

export function parseSessionInfo(value: unknown): SessionInfo {
  const record = requireRecord(value, 'session')
  return {
    user: parseUser(record.user),
    expiresAt: requireString(record, 'expires_at'),
    idleExpiresAt: requireString(record, 'idle_expires_at'),
    mfaEnrolled: requireBoolean(record, 'mfa_enrolled'),
    isAdmin: requireBoolean(record, 'is_admin'),
  }
}

export function parseLoginResult(value: unknown): LoginResult {
  const record = requireRecord(value, 'login')
  const status = requireString(record, 'status')
  if (status !== 'authenticated' && status !== 'mfa_required' && status !== 'mfa_enrollment_required') {
    throw new Error(`SSO API returned an unknown login status "${status}"`)
  }
  const user = record.user === null || record.user === undefined ? null : parseUser(record.user)
  if (status === 'authenticated' && user === null) {
    throw new Error('SSO API authenticated login without a user')
  }
  return {
    status,
    mfaToken: readNullableString(record, 'mfa_token'),
    sessionExpiresAt: readNullableString(record, 'session_expires_at'),
    user,
    redirectTo: readNullableString(record, 'redirect_to'),
  }
}

export function parseSessionList(value: unknown): MeSession[] {
  const record = requireRecord(value, 'session list')
  return requireArray(record, 'items').map((item) => {
    const session = requireRecord(item, 'session')
    return {
      id: requireString(session, 'id'),
      ip: readNullableString(session, 'ip'),
      userAgent: readNullableString(session, 'user_agent'),
      createdAt: requireString(session, 'created_at'),
      lastSeenAt: requireString(session, 'last_seen_at'),
      expiresAt: requireString(session, 'expires_at'),
      current: requireBoolean(session, 'current'),
    }
  })
}

export async function fetchSession(): Promise<SessionInfo | null> {
  try {
    const payload = await apiFetch('/api/v1/auth/session', {
      redirectOnSessionExpired: false,
    })
    return parseSessionInfo(payload)
  } catch (error) {
    if (isApiError(error) && error.status === 401) {
      return null
    }
    throw error
  }
}

export async function login(input: LoginInput): Promise<LoginResult> {
  const payload = await preSessionFetch('/api/v1/auth/login', {
    email: input.email,
    password: input.password,
    remember_me: input.rememberMe,
  })
  return parseLoginResult(payload)
}

export async function logout(): Promise<void> {
  await sessionFetch('/api/v1/auth/logout', { method: 'POST' })
}

export async function forgotPassword(email: string): Promise<void> {
  await preSessionFetch('/api/v1/auth/forgot-password', { email })
}

export async function resetPassword(token: string, newPassword: string): Promise<void> {
  await preSessionFetch('/api/v1/auth/reset-password', {
    token,
    new_password: newPassword,
  })
}

export async function verifyEmail(token: string): Promise<void> {
  await preSessionFetch('/api/v1/auth/verify-email', { token })
}

export async function acceptInvite(token: string, password: string): Promise<void> {
  await preSessionFetch('/api/v1/auth/accept-invite', { token, password })
}

export async function updateProfile(input: ProfileInput): Promise<User> {
  const payload = await sessionFetch('/api/v1/me', {
    method: 'PATCH',
    body: {
      name: input.name,
      given_name: input.givenName,
      family_name: input.familyName,
      locale: input.locale,
    },
  })
  return parseUser(payload)
}

export async function changePassword(input: PasswordInput): Promise<void> {
  await sessionFetch('/api/v1/me/password', {
    method: 'POST',
    body: {
      current_password: input.currentPassword,
      new_password: input.newPassword,
    },
  })
}

export async function listSessions(): Promise<MeSession[]> {
  const payload = await apiFetch('/api/v1/me/sessions')
  return parseSessionList(payload)
}

export async function revokeSession(id: string): Promise<void> {
  await sessionFetch(`/api/v1/me/sessions/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}
