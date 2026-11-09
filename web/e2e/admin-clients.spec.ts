import { expect, test } from '@playwright/test'

import { adminStatePath } from './config'

test.use({ storageState: adminStatePath })

test('admin creates a public client from the clients screen', async ({ page }) => {
  const suffix = Date.now()
  const name = `E2E Client ${suffix}`
  const redirectURI = `https://e2e-${suffix}.example.com/callback`

  await page.goto('/admin/clients')
  await expect(page.getByRole('heading', { name: 'Clients', exact: true })).toBeVisible()

  await page.getByRole('button', { name: 'Create client', exact: true }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByRole('heading', { name: 'Create a client', exact: true })).toBeVisible()

  await dialog.getByLabel('Name', { exact: true }).fill(name)
  await dialog.getByLabel('Redirect URIs', { exact: true }).fill(redirectURI)
  await dialog.getByRole('button', { name: 'Create client', exact: true }).click()

  await expect(dialog).toBeHidden()
  await expect(page.getByRole('row').filter({ hasText: name })).toBeVisible()
})
