export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function readString(record: Record<string, unknown>, key: string): string {
  const value = record[key]
  return typeof value === 'string' ? value : ''
}

export function requireString(record: Record<string, unknown>, key: string): string {
  const value = record[key]
  if (typeof value !== 'string') {
    throw new Error(`SSO API field "${key}" is missing or invalid`)
  }
  return value
}

export function readNullableString(
  record: Record<string, unknown>,
  key: string,
): string | null {
  const value = record[key]
  if (value === null || value === undefined) {
    return null
  }
  if (typeof value !== 'string') {
    throw new Error(`SSO API field "${key}" is invalid`)
  }
  return value
}

export function requireBoolean(record: Record<string, unknown>, key: string): boolean {
  const value = record[key]
  if (typeof value !== 'boolean') {
    throw new Error(`SSO API field "${key}" is missing or invalid`)
  }
  return value
}

export function requireArray(record: Record<string, unknown>, key: string): unknown[] {
  const value = record[key]
  if (!Array.isArray(value)) {
    throw new Error(`SSO API field "${key}" is missing or invalid`)
  }
  return value
}

export function requireRecord(value: unknown, label: string): Record<string, unknown> {
  if (!isRecord(value)) {
    throw new Error(`SSO API returned an unexpected ${label} payload`)
  }
  return value
}
