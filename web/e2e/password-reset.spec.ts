import { expect, test } from '@playwright/test'

import { createInvitedUser } from './admin'
import { adminStatePath } from './config'
import { signInAs } from './login'
import { signOut, waitForLink } from './mailhog'

test('user accepts an invitation and resets a password', async ({ browser, page }) => {
  const email = `e2e-reset-${Date.now()}@example.com`
  const invitePassword = 'Calico-Harbor-4821!'
  const resetPassword = 'Meridian-Stone-9137!'

  const adminContext = await browser.newContext({ storageState: adminStatePath })
  const adminPage = await adminContext.newPage()
  await createInvitedUser(adminPage, email, 'E2E Reset')
  const inviteLink = await waitForLink(email, '/accept-invite')
  await adminContext.close()

  await page.goto(inviteLink)
  await expect(page.getByRole('heading', { name: 'Accept invitation', exact: true })).toBeVisible()
  await page.getByLabel('Password', { exact: true }).fill(invitePassword)
  await page.getByLabel('Confirm password', { exact: true }).fill(invitePassword)
  await page.getByRole('button', { name: 'Activate account', exact: true }).click()
  await expect(page).toHaveURL(/\/login/)

  await signInAs(page, email, invitePassword)
  await expect(page.getByRole('heading', { name: 'Your account', exact: true })).toBeVisible()
  await signOut(page)

  await page.goto('/forgot-password')
  await page.getByLabel('Email', { exact: true }).fill(email)
  await page.getByRole('button', { name: 'Send reset link', exact: true }).click()
  await expect(page.getByText('Check your email', { exact: true })).toBeVisible()

  const resetLink = await waitForLink(email, '/reset-password')
  await page.goto(resetLink)
  await expect(page.getByRole('heading', { name: 'Reset password', exact: true })).toBeVisible()
  await page.getByLabel('New password', { exact: true }).fill(resetPassword)
  await page.getByLabel('Confirm new password', { exact: true }).fill(resetPassword)
  await page.getByRole('button', { name: 'Save password', exact: true }).click()
  await expect(page.getByText('Password updated', { exact: true })).toBeVisible()

  await signInAs(page, email, resetPassword)
  await expect(page.getByRole('heading', { name: 'Your account', exact: true })).toBeVisible()
})
