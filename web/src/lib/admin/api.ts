import { sessionFetch } from '../csrf'
import { apiFetch } from '../http'
import {
  parseAdminClient,
  parseAdminClientList,
  parseAdminClientWithSecret,
  parseAdminGroup,
  parseAdminGroupList,
  parseAdminRole,
  parseAdminRoleList,
  parseAdminSessionList,
  parseAdminUser,
  parseAdminUserList,
  parseAuditList,
  parseClientSecretResult,
  parseGroupMemberList,
  parseRealmSettings,
  type AdminClient,
  type AdminClientList,
  type AdminClientWithSecret,
  type AdminGroup,
  type AdminRole,
  type AdminSessionList,
  type AdminUser,
  type AdminUserList,
  type AuditList,
  type ClientSecretResult,
  type ClientType,
  type GrantType,
  type GroupMember,
  type RealmSettings,
} from './types'

type QueryValue = string | number | null | undefined

function query(params: ReadonlyArray<readonly [string, QueryValue]>): string {
  const search = new URLSearchParams()
  for (const [key, value] of params) {
    if (value === null || value === undefined || value === '') {
      continue
    }
    search.set(key, String(value))
  }
  const text = search.toString()
  return text === '' ? '' : `?${text}`
}

export interface ListUsersParams {
  q?: string
  status?: string
  after?: string
  limit?: number
}

export async function listUsers(params: ListUsersParams = {}): Promise<AdminUserList> {
  const payload = await apiFetch(
    `/api/v1/users${query([
      ['q', params.q],
      ['status', params.status],
      ['after', params.after],
      ['limit', params.limit],
    ])}`,
  )
  return parseAdminUserList(payload)
}

export async function getUser(id: string): Promise<AdminUser> {
  return parseAdminUser(await apiFetch(`/api/v1/users/${encodeURIComponent(id)}`))
}

export interface CreateUserInput {
  email: string
  name: string
  username: string | null
  sendInvite: boolean
  idempotencyKey: string
}

export async function createUser(input: CreateUserInput): Promise<AdminUser> {
  const payload = await sessionFetch('/api/v1/users', {
    method: 'POST',
    headers: { 'Idempotency-Key': input.idempotencyKey },
    body: {
      email: input.email,
      name: input.name,
      send_invite: input.sendInvite,
      ...(input.username === null || input.username === '' ? {} : { username: input.username }),
    },
  })
  return parseAdminUser(payload)
}

export interface UpdateUserInput {
  name?: string
  locale?: 'en' | 'id'
  disabled?: boolean
}

export async function updateUser(id: string, input: UpdateUserInput): Promise<AdminUser> {
  const body: Record<string, unknown> = {}
  if (input.name !== undefined) {
    body.name = input.name
  }
  if (input.locale !== undefined) {
    body.locale = input.locale
  }
  if (input.disabled !== undefined) {
    body.disabled = input.disabled
  }
  const payload = await sessionFetch(`/api/v1/users/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body,
  })
  return parseAdminUser(payload)
}

export async function resetUserPassword(id: string): Promise<void> {
  await sessionFetch(`/api/v1/users/${encodeURIComponent(id)}/reset-password`, {
    method: 'POST',
  })
}

export async function resendUserInvite(id: string): Promise<void> {
  await sessionFetch(`/api/v1/users/${encodeURIComponent(id)}/invite`, {
    method: 'POST',
  })
}

export async function assignUserRoles(id: string, roleIds: readonly string[]): Promise<void> {
  await sessionFetch(`/api/v1/users/${encodeURIComponent(id)}/roles`, {
    method: 'POST',
    body: { role_ids: roleIds },
  })
}

export interface ListClientsParams {
  after?: string
  limit?: number
}

export async function listClients(params: ListClientsParams = {}): Promise<AdminClientList> {
  const payload = await apiFetch(
    `/api/v1/clients${query([
      ['after', params.after],
      ['limit', params.limit],
    ])}`,
  )
  return parseAdminClientList(payload)
}

export async function getClient(id: string): Promise<AdminClient> {
  return parseAdminClient(await apiFetch(`/api/v1/clients/${encodeURIComponent(id)}`))
}

export interface ClientWriteInput {
  name: string
  redirectUris: string[]
  postLogoutRedirectUris: string[]
  backchannelLogoutUri: string | null
  corsOrigins: string[]
  grantTypes: GrantType[]
  allowedScopes: string[]
  thirdParty: boolean
  accessTokenTtlSeconds: number | null
  idTokenTtlSeconds: number | null
}

export interface CreateClientInput extends ClientWriteInput {
  type: ClientType
  idempotencyKey: string
}

export async function createClient(input: CreateClientInput): Promise<AdminClientWithSecret> {
  const payload = await sessionFetch('/api/v1/clients', {
    method: 'POST',
    headers: { 'Idempotency-Key': input.idempotencyKey },
    body: {
      name: input.name,
      type: input.type,
      redirect_uris: input.redirectUris,
      post_logout_redirect_uris: input.postLogoutRedirectUris,
      backchannel_logout_uri: input.backchannelLogoutUri,
      cors_origins: input.corsOrigins,
      grant_types: input.grantTypes,
      allowed_scopes: input.allowedScopes,
      third_party: input.thirdParty,
      access_token_ttl_seconds: input.accessTokenTtlSeconds,
      id_token_ttl_seconds: input.idTokenTtlSeconds,
    },
  })
  return parseAdminClientWithSecret(payload)
}

export interface UpdateClientInput extends ClientWriteInput {
  disabled: boolean
}

export async function updateClient(id: string, input: UpdateClientInput): Promise<AdminClient> {
  const payload = await sessionFetch(`/api/v1/clients/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: {
      name: input.name,
      redirect_uris: input.redirectUris,
      post_logout_redirect_uris: input.postLogoutRedirectUris,
      backchannel_logout_uri: input.backchannelLogoutUri ?? '',
      cors_origins: input.corsOrigins,
      grant_types: input.grantTypes,
      allowed_scopes: input.allowedScopes,
      third_party: input.thirdParty,
      access_token_ttl_seconds: input.accessTokenTtlSeconds,
      id_token_ttl_seconds: input.idTokenTtlSeconds,
      disabled: input.disabled,
    },
  })
  return parseAdminClient(payload)
}

export async function deleteClient(id: string): Promise<void> {
  await sessionFetch(`/api/v1/clients/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export async function rotateClientSecret(id: string): Promise<ClientSecretResult> {
  const payload = await sessionFetch(`/api/v1/clients/${encodeURIComponent(id)}/rotate-secret`, {
    method: 'POST',
  })
  return parseClientSecretResult(payload)
}

export async function listRoles(): Promise<AdminRole[]> {
  return parseAdminRoleList(await apiFetch('/api/v1/roles'))
}

export interface CreateRoleInput {
  name: string
  description: string | null
  idempotencyKey: string
}

export async function createRole(input: CreateRoleInput): Promise<AdminRole> {
  const payload = await sessionFetch('/api/v1/roles', {
    method: 'POST',
    headers: { 'Idempotency-Key': input.idempotencyKey },
    body: { name: input.name, description: input.description },
  })
  return parseAdminRole(payload)
}

export async function listGroups(): Promise<AdminGroup[]> {
  return parseAdminGroupList(await apiFetch('/api/v1/groups'))
}

export async function getGroup(id: string): Promise<AdminGroup> {
  return parseAdminGroup(await apiFetch(`/api/v1/groups/${encodeURIComponent(id)}`))
}

export interface GroupWriteInput {
  name: string
  description: string | null
}

export interface CreateGroupInput extends GroupWriteInput {
  idempotencyKey: string
}

export async function createGroup(input: CreateGroupInput): Promise<AdminGroup> {
  const payload = await sessionFetch('/api/v1/groups', {
    method: 'POST',
    headers: { 'Idempotency-Key': input.idempotencyKey },
    body: { name: input.name, description: input.description },
  })
  return parseAdminGroup(payload)
}

export async function updateGroup(id: string, input: GroupWriteInput): Promise<AdminGroup> {
  const payload = await sessionFetch(`/api/v1/groups/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: { name: input.name, description: input.description ?? '' },
  })
  return parseAdminGroup(payload)
}

export async function deleteGroup(id: string): Promise<void> {
  await sessionFetch(`/api/v1/groups/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export async function listGroupMembers(id: string): Promise<GroupMember[]> {
  return parseGroupMemberList(await apiFetch(`/api/v1/groups/${encodeURIComponent(id)}/members`))
}

export async function addGroupMembers(id: string, userIds: readonly string[]): Promise<GroupMember[]> {
  const payload = await sessionFetch(`/api/v1/groups/${encodeURIComponent(id)}/members`, {
    method: 'POST',
    body: { user_ids: userIds },
  })
  return parseGroupMemberList(payload)
}

export async function removeGroupMember(id: string, userId: string): Promise<void> {
  await sessionFetch(
    `/api/v1/groups/${encodeURIComponent(id)}/members/${encodeURIComponent(userId)}`,
    { method: 'DELETE' },
  )
}

export interface ListSessionsParams {
  userId?: string
  status?: string
  after?: string
  limit?: number
}

export async function listAdminSessions(
  params: ListSessionsParams = {},
): Promise<AdminSessionList> {
  const payload = await apiFetch(
    `/api/v1/sessions${query([
      ['user_id', params.userId],
      ['status', params.status],
      ['after', params.after],
      ['limit', params.limit],
    ])}`,
  )
  return parseAdminSessionList(payload)
}

export async function revokeAdminSession(id: string): Promise<void> {
  await sessionFetch(`/api/v1/sessions/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export interface ListAuditParams {
  from?: string
  to?: string
  type?: string
  actorId?: string
  after?: string
  limit?: number
}

export async function listAuditEvents(params: ListAuditParams = {}): Promise<AuditList> {
  const payload = await apiFetch(
    `/api/v1/audit${query([
      ['from', params.from],
      ['to', params.to],
      ['type', params.type],
      ['actor_id', params.actorId],
      ['after', params.after],
      ['limit', params.limit],
    ])}`,
  )
  return parseAuditList(payload)
}

export async function getRealmSettings(): Promise<RealmSettings> {
  return parseRealmSettings(await apiFetch('/api/v1/realm/settings'))
}

export interface UpdateRealmSettingsInput {
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
    password?: string
  }
}

export async function updateRealmSettings(
  input: UpdateRealmSettingsInput,
): Promise<RealmSettings> {
  const smtp: Record<string, unknown> = {
    host: input.smtp.host,
    port: input.smtp.port,
    username: input.smtp.username,
    from_address: input.smtp.fromAddress,
    use_tls: input.smtp.useTls,
  }
  if (input.smtp.password !== undefined && input.smtp.password !== '') {
    smtp.password = input.smtp.password
  }
  const payload = await sessionFetch('/api/v1/realm/settings', {
    method: 'PATCH',
    body: {
      password_policy: { min_length: input.passwordPolicy.minLength },
      session_policy: {
        idle_minutes: input.sessionPolicy.idleMinutes,
        absolute_hours: input.sessionPolicy.absoluteHours,
        admin_idle_minutes: input.sessionPolicy.adminIdleMinutes,
        remember_me_days: input.sessionPolicy.rememberMeDays,
      },
      smtp,
    },
  })
  return parseRealmSettings(payload)
}
