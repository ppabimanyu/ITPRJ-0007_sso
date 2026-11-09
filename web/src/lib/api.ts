export interface VersionInfo {
  version: string
  commit: string
  buildDate: string
  goVersion: string
}

export interface ApiHealth {
  endpoint: '/api/v1/version' | '/healthz'
  version: VersionInfo | null
}

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL?.replace(/\/$/, '') ?? ''

function endpointFor(path: string) {
  return `${apiBaseUrl}${path}`
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function readString(record: Record<string, unknown>, key: string) {
  const value = record[key]
  return typeof value === 'string' ? value : ''
}

function parseVersion(value: unknown): VersionInfo {
  if (!isRecord(value) || typeof value.version !== 'string') {
    throw new Error('/api/v1/version returned an unexpected payload')
  }
  return {
    version: value.version,
    commit: readString(value, 'commit'),
    buildDate: readString(value, 'build_date'),
    goVersion: readString(value, 'go_version'),
  }
}

async function getJSON(path: string): Promise<unknown> {
  const response = await fetch(endpointFor(path), {
    headers: { Accept: 'application/json' },
  })
  if (!response.ok) {
    throw new Error(`${path} responded with ${response.status}`)
  }
  return response.json() as Promise<unknown>
}

export async function fetchApiHealth(): Promise<ApiHealth> {
  try {
    const payload = await getJSON('/api/v1/version')
    return { endpoint: '/api/v1/version', version: parseVersion(payload) }
  } catch {
    const payload = await getJSON('/healthz')
    if (!isRecord(payload) || payload.status !== 'ok') {
      throw new Error('/healthz did not report ok')
    }
    return { endpoint: '/healthz', version: null }
  }
}
