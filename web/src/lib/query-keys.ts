export const authKeys = {
  session: ['auth', 'session'] as const,
  sessions: ['auth', 'sessions'] as const,
}

export const oidcKeys = {
  authorizeRequest: (requestId: string) =>
    ['oidc', 'authorize-request', requestId] as const,
}
