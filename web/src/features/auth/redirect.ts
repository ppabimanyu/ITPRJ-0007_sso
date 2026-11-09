const AUTH_PATHS = new Set([
  '/login',
  '/consent',
  '/forgot-password',
  '/reset-password',
  '/accept-invite',
  '/verify-email',
  '/mfa/enroll',
])

export function isLocalRedirect(value: unknown): value is string {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//')) {
    return false
  }
  if (value.startsWith('/\\')) {
    return false
  }
  try {
    const url = new URL(value, window.location.origin)
    return url.origin === window.location.origin && !AUTH_PATHS.has(url.pathname)
  } catch {
    return false
  }
}
