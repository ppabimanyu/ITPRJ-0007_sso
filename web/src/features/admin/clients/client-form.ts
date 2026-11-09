import type { AdminClient, ClientType, GrantType } from '@/lib/admin/types'

export interface ClientFormValues {
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
}

export const grantTypeOptions: readonly GrantType[] = [
  'authorization_code',
  'refresh_token',
  'client_credentials',
]

export function splitLines(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter((item) => item !== '')
}

export function splitScopes(value: string): string[] {
  return value
    .split(/[\s,]+/)
    .map((item) => item.trim())
    .filter((item) => item !== '')
}

export function joinLines(values: readonly string[]): string {
  return values.join('\n')
}

export function parseTtl(value: string): number | null {
  const trimmed = value.trim()
  if (trimmed === '') {
    return null
  }
  const parsed = Number(trimmed)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return null
  }
  return Math.trunc(parsed)
}

export function formatTtl(value: number | null): string {
  return value === null ? '' : String(value)
}

export function clientToForm(client: AdminClient | null): {
  name: string
  type: ClientType
  redirectUris: string
  postLogoutRedirectUris: string
  backchannelLogoutUri: string
  corsOrigins: string
  grantTypes: GrantType[]
  allowedScopes: string
  thirdParty: boolean
  accessTokenTtl: string
  idTokenTtl: string
  disabled: boolean
} {
  return {
    name: client?.name ?? '',
    type: client?.type ?? 'public',
    redirectUris: joinLines(client?.redirectUris ?? []),
    postLogoutRedirectUris: joinLines(client?.postLogoutRedirectUris ?? []),
    backchannelLogoutUri: client?.backchannelLogoutUri ?? '',
    corsOrigins: joinLines(client?.corsOrigins ?? []),
    grantTypes: client?.grantTypes ?? ['authorization_code'],
    allowedScopes: joinLines(client?.allowedScopes ?? ['openid']),
    thirdParty: client?.thirdParty ?? false,
    accessTokenTtl: formatTtl(client?.accessTokenTtlSeconds ?? null),
    idTokenTtl: formatTtl(client?.idTokenTtlSeconds ?? null),
    disabled: client?.disabled ?? false,
  }
}
