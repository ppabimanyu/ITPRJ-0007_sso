import { expect, test } from '@playwright/test'

import { userIdentity, userStatePath } from './config'
import { signInAs } from './login'

test('wrong password returns a generic error', async ({ page }) => {
  await signInAs(page, userIdentity.email, 'definitely-the-wrong-password')
  await expect(page.getByText('Email or password is incorrect.', { exact: true })).toBeVisible()
  await expect(page).toHaveURL(/\/login/)
})

test.describe('non-admin', () => {
  test.use({ storageState: userStatePath })

  test('is redirected away from the admin console', async ({ page }) => {
    await page.goto('/admin')
    await expect(page).toHaveURL(/\/account/)
    await expect(page.getByRole('heading', { name: 'Admin console', exact: true })).toBeHidden()
  })
})
