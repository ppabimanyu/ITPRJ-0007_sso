import { expect, test } from '@playwright/test'

import { createInvitedUser } from './admin'
import { adminStatePath, type TotpState } from './config'
import { signInAs, verifyTotpChallenge } from './login'
import { signOut, waitForLink } from './mailhog'
import { totpCode, waitForFreshStep } from './totp'

test('user accepts an invitation and enrolls an authenticator app', async ({ browser, page }) => {
  const email = `e2e-mfa-${Date.now()}@example.com`
  const password = 'Copper-Lantern-5273!'

  const adminContext = await browser.newContext({ storageState: adminStatePath })
  const adminPage = await adminContext.newPage()
  await createInvitedUser(adminPage, email, 'E2E MFA')
  const inviteLink = await waitForLink(email, '/accept-invite')
  await adminContext.close()

  await page.goto(inviteLink)
  await page.getByLabel('Password', { exact: true }).fill(password)
  await page.getByLabel('Confirm password', { exact: true }).fill(password)
  await page.getByRole('button', { name: 'Activate account', exact: true }).click()
  await expect(page).toHaveURL(/\/login/)

  await signInAs(page, email, password)
  await expect(page.getByRole('heading', { name: 'Your account', exact: true })).toBeVisible()

  await page.getByRole('tab', { name: 'MFA', exact: true }).click()
  await page.getByRole('button', { name: 'Enable two-step verification', exact: true }).click()
  const secret = (await page.locator('#mfa-enroll-secret').textContent())?.trim() ?? ''
  if (secret === '') {
    throw new Error('MFA enrollment did not show a setup key')
  }
  const step = await waitForFreshStep()
  await page.getByLabel('Authentication code').fill(totpCode(secret, step))
  await page.getByRole('button', { name: 'Confirm and enable', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Save your recovery codes', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'I saved these codes', exact: true }).click()

  await signOut(page)
  await signInAs(page, email, password)
  const state: TotpState = { secret, confirmedStep: step }
  await verifyTotpChallenge(
    page,
    state,
    page.getByRole('heading', { name: 'Your account', exact: true }),
  )
})
