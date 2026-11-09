import { expect, test } from '@playwright/test'

import { adminStatePath } from './config'

test.use({ storageState: adminStatePath })

test('admin creates a user from the users screen', async ({ page }) => {
  const email = `e2e-user-${Date.now()}@example.com`

  await page.goto('/admin/users')
  await expect(page.getByRole('heading', { name: 'Users', exact: true })).toBeVisible()

  await page.getByRole('button', { name: 'Create user', exact: true }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByRole('heading', { name: 'Create a user', exact: true })).toBeVisible()

  await dialog.getByLabel('Email', { exact: true }).fill(email)
  await dialog.getByLabel('Full name', { exact: true }).fill('E2E User')
  await dialog.getByRole('button', { name: 'Create user', exact: true }).click()

  await expect(dialog.getByRole('heading', { name: 'User created', exact: true })).toBeVisible()
  await dialog.getByRole('button', { name: 'Done', exact: true }).click()

  await expect(page.getByRole('row').filter({ hasText: email })).toBeVisible()
})
