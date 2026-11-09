import { createHmac } from 'node:crypto'

export const TOTP_STEP_SECONDS = 30

const base32Alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'

export function decodeBase32(secret: string): Buffer {
  const normalized = secret.replace(/[\s=]/g, '').toUpperCase()
  let buffer = 0
  let bits = 0
  const bytes: number[] = []
  for (const character of normalized) {
    const index = base32Alphabet.indexOf(character)
    if (index === -1) {
      throw new Error(`Invalid base32 character "${character}" in TOTP secret`)
    }
    buffer = (buffer << 5) | index
    bits += 5
    if (bits >= 8) {
      bits -= 8
      bytes.push((buffer >>> bits) & 0xff)
      buffer &= (1 << bits) - 1
    }
  }
  return Buffer.from(bytes)
}

export function totpStep(now = Date.now()): number {
  return Math.floor(now / 1000 / TOTP_STEP_SECONDS)
}

export function totpCode(secret: string, step = totpStep()): string {
  const counter = Buffer.alloc(8)
  counter.writeBigUInt64BE(BigInt(step))
  const digest = createHmac('sha1', decodeBase32(secret)).update(counter).digest()
  const offset = digest[digest.length - 1]! & 0x0f
  const value =
    ((digest[offset]! & 0x7f) << 24) |
    (digest[offset + 1]! << 16) |
    (digest[offset + 2]! << 8) |
    digest[offset + 3]!
  return String(value % 1_000_000).padStart(6, '0')
}

function stepEndsAt(step: number): number {
  return (step + 1) * TOTP_STEP_SECONDS * 1000
}

export async function waitForFreshStep(minRemainingMs = 10_000): Promise<number> {
  for (;;) {
    const step = totpStep()
    const remaining = stepEndsAt(step) - Date.now()
    if (remaining >= minRemainingMs) {
      return step
    }
    await new Promise((resolve) => setTimeout(resolve, remaining + 250))
  }
}

export async function waitForStepAfter(step: number): Promise<number> {
  for (;;) {
    const current = totpStep()
    if (current > step) {
      return current
    }
    await new Promise((resolve) => setTimeout(resolve, stepEndsAt(current) - Date.now() + 250))
  }
}
