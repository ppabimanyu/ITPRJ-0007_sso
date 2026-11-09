import path from 'node:path'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))

export const baseURL = process.env.E2E_BASE_URL ?? 'http://localhost:8080'

export const authDir = path.join(here, '.auth')
export const userStatePath = path.join(authDir, 'user.json')
export const adminStatePath = path.join(authDir, 'admin.json')
export const adminTotpPath = path.join(authDir, 'admin-totp.json')

export const userIdentity = {
  email: 'user@example.com',
  password: process.env.E2E_USER_PASSWORD ?? 'E2E-User-Passw0rd!',
}

export const adminIdentity = {
  email: 'admin@example.com',
  password: process.env.E2E_ADMIN_PASSWORD ?? 'E2E-Admin-Passw0rd!',
}

export interface TotpState {
  secret: string
  confirmedStep: number
}
