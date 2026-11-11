import { expect, test } from '@playwright/test'

import { createRole } from './admin'
import { adminStatePath } from './config'

test.use({ storageState: adminStatePath })

test('admin creates a role', async ({ page }) => {
  const name = `e2e-role-${Date.now()}`
  await createRole(page, name)
  await expect(
    page.getByRole('row').filter({ hasText: name }).getByText('Custom', { exact: true }),
  ).toBeVisible()
})
