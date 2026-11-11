import { expect, test as setup, type Page } from '@playwright/test'
import { mkdir, readFile, writeFile } from 'node:fs/promises'

import {
  adminIdentity,
  adminStatePath,
  adminTotpPath,
  authDir,
  userIdentity,
  userStatePath,
  type TotpState,
} from './config'
import { signInAs, verifyTotpChallenge } from './login'
import { totpCode, waitForFreshStep } from './totp'

async function readTotpState(): Promise<TotpState | null> {
  try {
    return JSON.parse(await readFile(adminTotpPath, 'utf8')) as TotpState
  } catch {
    return null
  }
}

async function saveTotpState(state: TotpState) {
  await writeFile(adminTotpPath, `${JSON.stringify(state, null, 2)}\n`, 'utf8')
}

async function reauthenticateWithTotp(page: Page, state: TotpState) {
  await page.context().clearCookies()
  await signInAs(page, adminIdentity.email, adminIdentity.password)
  await verifyTotpChallenge(
    page,
    state,
    page.getByRole('heading', { name: 'Your account', exact: true }),
  )
  await saveTotpState(state)
}

async function enrollAdmin(page: Page, state: TotpState | null) {
  const secretLocator = page.locator('#mfa-enroll-secret')
  const alreadyTitle = page.getByText('Two-step verification is already on', { exact: true })
  await expect(secretLocator.or(alreadyTitle)).toBeVisible({ timeout: 20_000 })

  if (await alreadyTitle.isVisible()) {
    if (state === null) {
      throw new Error(
        'Admin is already enrolled but e2e/.auth/admin-totp.json is missing; delete the e2e database and rerun.',
      )
    }
    await reauthenticateWithTotp(page, state)
    return
  }

  await expect(
    page.getByRole('img', { name: 'QR code for authenticator setup', exact: true }),
  ).toBeVisible()

  const secret = (await secretLocator.textContent())?.trim() ?? ''
  if (secret === '') {
    throw new Error('MFA enrollment did not display a setup key')
  }
  state = { secret, confirmedStep: 0 }
  await saveTotpState(state)

  const step = await waitForFreshStep()
  await page.getByLabel('Authentication code').fill(totpCode(secret, step))
  await page.getByRole('button', { name: 'Confirm and enable' }).click()

  const recoveryTitle = page.getByText('Save your recovery codes', { exact: true })
  await expect(recoveryTitle.or(alreadyTitle)).toBeVisible({ timeout: 20_000 })
  if (await alreadyTitle.isVisible()) {
    await reauthenticateWithTotp(page, state)
    return
  }

  state.confirmedStep = step
  await saveTotpState(state)
  await page.getByRole('button', { name: 'I saved these codes' }).click()
  await expect(page.getByRole('heading', { name: 'Your account', exact: true })).toBeVisible()
}

setup('authenticate as the sample user', async ({ page }) => {
  await signInAs(page, userIdentity.email, userIdentity.password)
  await expect(page).toHaveURL(/\/account$/)
  await expect(page.getByRole('heading', { name: 'Your account', exact: true })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Account', exact: true })).toBeVisible()

  await mkdir(authDir, { recursive: true })
  await page.context().storageState({ path: userStatePath })
})

setup('authenticate as the platform admin', async ({ page }) => {
  await mkdir(authDir, { recursive: true })
  const state = await readTotpState()

  await signInAs(page, adminIdentity.email, adminIdentity.password)

  const challengeTitle = page.getByRole('heading', {
    name: 'Two-step verification',
    exact: true,
  })
  const enrollTitle = page.getByRole('heading', {
    name: 'Set up two-step verification',
    exact: true,
  })
  await expect(challengeTitle.or(enrollTitle)).toBeVisible()

  if (await enrollTitle.isVisible()) {
    await enrollAdmin(page, state)
  } else {
    if (state === null) {
      throw new Error(
        'Admin MFA is enrolled but e2e/.auth/admin-totp.json is missing; delete the e2e database and rerun.',
      )
    }
    await verifyTotpChallenge(
      page,
      state,
      page.getByRole('heading', { name: 'Your account', exact: true }),
    )
    await saveTotpState(state)
  }

  await expect(page.getByRole('link', { name: 'Admin', exact: true })).toBeVisible()
  await page.context().storageState({ path: adminStatePath })
})
