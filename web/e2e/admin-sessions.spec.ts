import { expect, test } from '@playwright/test'

import { adminStatePath, userIdentity } from './config'
import { signInAs } from './login'

test.use({ storageState: adminStatePath })

test('admin revokes a user session', async ({ browser, page }) => {
  const userContext = await browser.newContext()
  const userPage = await userContext.newPage()
  await signInAs(userPage, userIdentity.email, userIdentity.password)
  await expect(userPage.getByRole('heading', { name: 'Your account', exact: true })).toBeVisible()

  await page.goto('/admin/sessions')
  await expect(page.getByRole('heading', { name: 'Sessions', exact: true })).toBeVisible()

  const active = page
    .getByRole('row')
    .filter({ hasText: userIdentity.email })
    .filter({ hasText: 'Active' })
    .first()
  await expect(active).toBeVisible()
  await active.getByRole('button', { name: 'Revoke', exact: true }).click()

  const dialog = page.getByRole('alertdialog')
  await dialog.getByRole('button', { name: 'Revoke', exact: true }).click()
  await expect(
    page
      .getByRole('row')
      .filter({ hasText: userIdentity.email })
      .filter({ hasText: 'Revoked' })
      .first(),
  ).toBeVisible()

  await userContext.close()
})
