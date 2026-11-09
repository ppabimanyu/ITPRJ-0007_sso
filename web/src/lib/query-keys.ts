export const authKeys = {
  session: ['auth', 'session'] as const,
  sessions: ['auth', 'sessions'] as const,
}

export const oidcKeys = {
  authorizeRequest: (requestId: string) =>
    ['oidc', 'authorize-request', requestId] as const,
}

export const adminKeys = {
  users: ['admin', 'users'] as const,
  userList: (params: object) => ['admin', 'users', 'list', params] as const,
  user: (id: string) => ['admin', 'users', 'detail', id] as const,
  clients: ['admin', 'clients'] as const,
  clientList: (params: object) => ['admin', 'clients', 'list', params] as const,
  client: (id: string) => ['admin', 'clients', 'detail', id] as const,
  roles: ['admin', 'roles'] as const,
  groups: ['admin', 'groups'] as const,
  group: (id: string) => ['admin', 'groups', 'detail', id] as const,
  groupMembers: (id: string) => ['admin', 'groups', 'members', id] as const,
  sessions: ['admin', 'sessions'] as const,
  sessionList: (params: object) => ['admin', 'sessions', 'list', params] as const,
  audit: ['admin', 'audit'] as const,
  auditList: (params: object) => ['admin', 'audit', 'list', params] as const,
  settings: ['admin', 'settings'] as const,
}
