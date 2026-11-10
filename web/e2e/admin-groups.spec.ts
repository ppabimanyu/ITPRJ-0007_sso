import { expect, test } from '@playwright/test'

import { createGroup } from './admin'
import { adminStatePath } from './config'

test.use({ storageState: adminStatePath })

test('admin creates a group', async ({ page }) => {
  const name = `e2e-group-${Date.now()}`
  await createGroup(page, name, 'Created by the e2e suite')
  await expect(page.getByRole('row').filter({ hasText: 'Created by the e2e suite' })).toBeVisible()
})
