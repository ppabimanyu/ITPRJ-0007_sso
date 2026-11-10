import { expect, test } from '@playwright/test'

import { adminStatePath } from './config'

test.use({ storageState: adminStatePath })

test('admin updates the realm password policy', async ({ page }) => {
  await page.goto('/admin/settings')
  await expect(page.getByRole('heading', { name: 'Realm settings', exact: true })).toBeVisible()

  const minLength = page.getByLabel('Minimum length', { exact: true })
  await minLength.fill('13')
  await page.getByRole('button', { name: 'Save settings', exact: true }).click()
  await expect(page.getByText('Realm settings saved.', { exact: true })).toBeVisible()

  await page.reload()
  await expect(page.getByLabel('Minimum length', { exact: true })).toHaveValue('13')

  await page.getByLabel('Minimum length', { exact: true }).fill('12')
  await page.getByRole('button', { name: 'Save settings', exact: true }).click()
  await expect(page.getByText('Realm settings saved.', { exact: true })).toBeVisible()
})
