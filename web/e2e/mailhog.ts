import type { Page } from '@playwright/test'

const mailhogURL = process.env.E2E_MAILHOG_URL ?? 'http://localhost:8025'

interface MailHogMessage {
  To?: Array<{ Mailbox: string; Domain: string }>
  Content?: { Body?: string }
}

function decodeQuotedPrintable(body: string): string {
  return body.replace(/=\r?\n/g, '').replace(/=3D/g, '=')
}

export async function waitForLink(
  email: string,
  pathFragment: string,
  timeoutMs = 30_000,
): Promise<string> {
  const deadline = Date.now() + timeoutMs
  const wanted = email.toLowerCase()
  while (Date.now() < deadline) {
    const response = await fetch(`${mailhogURL}/api/v2/messages?limit=100`)
    if (response.ok) {
      const payload = (await response.json()) as { items?: MailHogMessage[] }
      for (const item of payload.items ?? []) {
        const recipient = item.To?.[0]
        if (!recipient || `${recipient.Mailbox}@${recipient.Domain}`.toLowerCase() !== wanted) {
          continue
        }
        const body = decodeQuotedPrintable(item.Content?.Body ?? '')
        const match = body.match(new RegExp(`https?://[^\\s"<>]+${pathFragment}[^\\s"<>]*`))
        if (match) {
          return match[0]
        }
      }
    }
    await new Promise((resolve) => setTimeout(resolve, 400))
  }
  throw new Error(`no email for ${email} containing ${pathFragment}`)
}

export async function signOut(page: Page) {
  await page.getByRole('button', { name: 'Sign out', exact: true }).first().click()
}
