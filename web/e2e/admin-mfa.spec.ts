import { expect, test } from '@playwright/test'
import { readFile, writeFile } from 'node:fs/promises'

import { adminIdentity, adminTotpPath, type TotpState } from './config'
import { signInAs, verifyTotpChallenge } from './login'

test('admin completes the MFA challenge and reaches the console', async ({ page }) => {
  const state = JSON.parse(await readFile(adminTotpPath, 'utf8')) as TotpState

  await signInAs(page, adminIdentity.email, adminIdentity.password, '/login?redirect=/admin')
  await verifyTotpChallenge(
    page,
    state,
    page.getByRole('heading', { name: 'Admin console', exact: true }),
  )

  await expect(page).toHaveURL(/\/admin$/)
  await expect(page.getByRole('heading', { name: 'Admin console', exact: true })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Users', exact: true })).toBeVisible()

  await writeFile(adminTotpPath, `${JSON.stringify(state, null, 2)}\n`, 'utf8')
})
