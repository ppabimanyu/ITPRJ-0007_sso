import { expect, test } from '@playwright/test'

import { userIdentity } from './config'
import { signInAs } from './login'

test('user signs in with a password and keeps the session', async ({ page }) => {
  await signInAs(page, userIdentity.email, userIdentity.password)

  await expect(page).toHaveURL(/\/account$/)
  await expect(page.getByRole('heading', { name: 'Your account', exact: true })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Account', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Sign out', exact: true }).first()).toBeVisible()

  await page.reload()

  await expect(page.getByRole('heading', { name: 'Your account', exact: true })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Account', exact: true })).toBeVisible()
})
