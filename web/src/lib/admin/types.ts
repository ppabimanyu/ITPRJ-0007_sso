import { parseUser, type User } from '../auth-api'
import {
  readNullableString,
  readString,
  requireArray,
  requireBoolean,
  requireRecord,
  requireString,
} from '../json'

export interface AdminUser extends User {
  sessionCount: number
}

export interface AdminUserList {
  items: AdminUser[]
  nextAfter: string | null
}

export type ClientType = 'public' | 'confidential'

export type GrantType = 'authorization_code' | 'refresh_token' | 'client_credentials'

export interface AdminClient {
  id: string
  clientId: string
  name: string
  type: ClientType
  redirectUris: string[]
  postLogoutRedirectUris: string[]
  backchannelLogoutUri: string | null
  corsOrigins: string[]
  grantTypes: GrantType[]
  allowedScopes: string[]
  thirdParty: boolean
  accessTokenTtlSeconds: number | null
  idTokenTtlSeconds: number | null
  disabled: boolean
  secretRotatedAt: string | null
  createdAt: string
}

export interface AdminClientWithSecret extends AdminClient {
  clientSecret: string | null
}

export interface AdminClientList {
  items: AdminClient[]
  nextAfter: string | null
}

export interface ClientSecretResult {
  clientId: string
  clientSecret: string
  rotatedAt: string
}

export interface AdminRole {
  id: string
  name: string
  description: string | null
  isSystem: boolean
}

export interface AdminGroup {
  id: string
  name: string
  description: string | null
  memberCount: number
}

export interface GroupMember {
  userId: string
  email: string
  name: string | null
}

export interface AdminSession {
  id: string
  userId: string
  userEmail: string
  ip: string | null
  userAgent: string | null
  amr: string[]
  acr: string
  authTime: string
  createdAt: string
  lastSeenAt: string
  expiresAt: string
  revokedAt: string | null
}

export interface AdminSessionList {
  items: AdminSession[]
  nextAfter: string | null
}

export interface AuditEvent {
  id: string
  type: string
  actorType: string
  actorId: string | null
  metadata: unknown
  createdAt: string
}

export interface AuditList {
  items: AuditEvent[]
  nextAfter: string | null
}

export interface RealmSettings {
  realm: string
  passwordPolicy: {
    minLength: number
  }
  sessionPolicy: {
    idleMinutes: number
    absoluteHours: number
    adminIdleMinutes: number
    rememberMeDays: number
  }
  smtp: {
    host: string
    port: number
    username: string | null
    fromAddress: string
    useTls: boolean
    passwordSet: boolean
  }
}

function requireNumber(record: Record<string, unknown>, key: string): number {
  const value = record[key]
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    throw new Error(`SSO API field "${key}" is missing or invalid`)
  }
  return value
}

function readNullableNumber(record: Record<string, unknown>, key: string): number | null {
  const value = record[key]
  if (value === null || value === undefined) {
    return null
  }
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    throw new Error(`SSO API field "${key}" is invalid`)
  }
  return value
}

function readStringArray(record: Record<string, unknown>, key: string): string[] {
  const value = record[key]
  if (value === null || value === undefined) {
    return []
  }
  if (!Array.isArray(value)) {
    throw new Error(`SSO API field "${key}" is invalid`)
  }
  const result: string[] = []
  for (const item of value) {
    if (typeof item !== 'string') {
      throw new Error(`SSO API field "${key}" is invalid`)
    }
    result.push(item)
  }
  return result
}

function readNextAfter(record: Record<string, unknown>): string | null {
  return readNullableString(record, 'next_after')
}

function readClientType(record: Record<string, unknown>): ClientType {
  const value = requireString(record, 'type')
  if (value === 'public' || value === 'confidential') {
    return value
  }
  throw new Error(`SSO API returned an unknown client type "${value}"`)
}

function readGrantTypes(record: Record<string, unknown>): GrantType[] {
  const result: GrantType[] = []
  for (const value of readStringArray(record, 'grant_types')) {
    if (value === 'authorization_code' || value === 'refresh_token' || value === 'client_credentials') {
      result.push(value)
      continue
    }
    throw new Error(`SSO API returned an unknown grant type "${value}"`)
  }
  return result
}

export function parseAdminUser(value: unknown): AdminUser {
  const record = requireRecord(value, 'admin user')
  return {
    ...parseUser(value),
    sessionCount: requireNumber(record, 'session_count'),
  }
}

export function parseAdminUserList(value: unknown): AdminUserList {
  const record = requireRecord(value, 'admin user list')
  return {
    items: requireArray(record, 'items').map(parseAdminUser),
    nextAfter: readNextAfter(record),
  }
}

export function parseAdminClient(value: unknown): AdminClient {
  const record = requireRecord(value, 'client')
  return {
    id: requireString(record, 'id'),
    clientId: requireString(record, 'client_id'),
    name: requireString(record, 'name'),
    type: readClientType(record),
    redirectUris: readStringArray(record, 'redirect_uris'),
    postLogoutRedirectUris: readStringArray(record, 'post_logout_redirect_uris'),
    backchannelLogoutUri: readNullableString(record, 'backchannel_logout_uri'),
    corsOrigins: readStringArray(record, 'cors_origins'),
    grantTypes: readGrantTypes(record),
    allowedScopes: readStringArray(record, 'allowed_scopes'),
    thirdParty: requireBoolean(record, 'third_party'),
    accessTokenTtlSeconds: readNullableNumber(record, 'access_token_ttl_seconds'),
    idTokenTtlSeconds: readNullableNumber(record, 'id_token_ttl_seconds'),
    disabled: requireBoolean(record, 'disabled'),
    secretRotatedAt: readNullableString(record, 'secret_rotated_at'),
    createdAt: requireString(record, 'created_at'),
  }
}

export function parseAdminClientList(value: unknown): AdminClientList {
  const record = requireRecord(value, 'client list')
  return {
    items: requireArray(record, 'items').map(parseAdminClient),
    nextAfter: readNextAfter(record),
  }
}

export function parseAdminClientWithSecret(value: unknown): AdminClientWithSecret {
  const record = requireRecord(value, 'client secret response')
  return {
    ...parseAdminClient(value),
    clientSecret: readNullableString(record, 'client_secret'),
  }
}

export function parseClientSecretResult(value: unknown): ClientSecretResult {
  const record = requireRecord(value, 'client secret')
  return {
    clientId: requireString(record, 'client_id'),
    clientSecret: requireString(record, 'client_secret'),
    rotatedAt: requireString(record, 'rotated_at'),
  }
}

export function parseAdminRole(value: unknown): AdminRole {
  const record = requireRecord(value, 'role')
  return {
    id: requireString(record, 'id'),
    name: requireString(record, 'name'),
    description: readNullableString(record, 'description'),
    isSystem: requireBoolean(record, 'is_system'),
  }
}

export function parseAdminRoleList(value: unknown): AdminRole[] {
  const record = requireRecord(value, 'role list')
  return requireArray(record, 'items').map(parseAdminRole)
}

export function parseAdminGroup(value: unknown): AdminGroup {
  const record = requireRecord(value, 'group')
  return {
    id: requireString(record, 'id'),
    name: requireString(record, 'name'),
    description: readNullableString(record, 'description'),
    memberCount: requireNumber(record, 'member_count'),
  }
}

export function parseAdminGroupList(value: unknown): AdminGroup[] {
  const record = requireRecord(value, 'group list')
  return requireArray(record, 'items').map(parseAdminGroup)
}

export function parseGroupMember(value: unknown): GroupMember {
  const record = requireRecord(value, 'group member')
  return {
    userId: requireString(record, 'user_id'),
    email: requireString(record, 'email'),
    name: readNullableString(record, 'name'),
  }
}

export function parseGroupMemberList(value: unknown): GroupMember[] {
  const record = requireRecord(value, 'group member list')
  return requireArray(record, 'items').map(parseGroupMember)
}

export function parseAdminSession(value: unknown): AdminSession {
  const record = requireRecord(value, 'session')
  return {
    id: requireString(record, 'id'),
    userId: requireString(record, 'user_id'),
    userEmail: requireString(record, 'user_email'),
    ip: readNullableString(record, 'ip'),
    userAgent: readNullableString(record, 'user_agent'),
    amr: readStringArray(record, 'amr'),
    acr: readString(record, 'acr'),
    authTime: requireString(record, 'auth_time'),
    createdAt: requireString(record, 'created_at'),
    lastSeenAt: requireString(record, 'last_seen_at'),
    expiresAt: requireString(record, 'expires_at'),
    revokedAt: readNullableString(record, 'revoked_at'),
  }
}

export function parseAdminSessionList(value: unknown): AdminSessionList {
  const record = requireRecord(value, 'session list')
  return {
    items: requireArray(record, 'items').map(parseAdminSession),
    nextAfter: readNextAfter(record),
  }
}

export function parseAuditEvent(value: unknown): AuditEvent {
  const record = requireRecord(value, 'audit event')
  return {
    id: requireString(record, 'id'),
    type: requireString(record, 'type'),
    actorType: requireString(record, 'actor_type'),
    actorId: readNullableString(record, 'actor_id'),
    metadata: record.metadata ?? null,
    createdAt: requireString(record, 'created_at'),
  }
}

export function parseAuditList(value: unknown): AuditList {
  const record = requireRecord(value, 'audit list')
  return {
    items: requireArray(record, 'items').map(parseAuditEvent),
    nextAfter: readNextAfter(record),
  }
}

export function parseRealmSettings(value: unknown): RealmSettings {
  const record = requireRecord(value, 'realm settings')
  const passwordPolicy = requireRecord(record.password_policy, 'password policy')
  const sessionPolicy = requireRecord(record.session_policy, 'session policy')
  const smtp = requireRecord(record.smtp, 'smtp settings')
  return {
    realm: requireString(record, 'realm'),
    passwordPolicy: {
      minLength: requireNumber(passwordPolicy, 'min_length'),
    },
    sessionPolicy: {
      idleMinutes: requireNumber(sessionPolicy, 'idle_minutes'),
      absoluteHours: requireNumber(sessionPolicy, 'absolute_hours'),
      adminIdleMinutes: requireNumber(sessionPolicy, 'admin_idle_minutes'),
      rememberMeDays: requireNumber(sessionPolicy, 'remember_me_days'),
    },
    smtp: {
      host: requireString(smtp, 'host'),
      port: requireNumber(smtp, 'port'),
      username: readNullableString(smtp, 'username'),
      fromAddress: requireString(smtp, 'from_address'),
      useTls: requireBoolean(smtp, 'use_tls'),
      passwordSet: requireBoolean(smtp, 'password_set'),
    },
  }
}
