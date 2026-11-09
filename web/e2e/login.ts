import { expect, type Locator, type Page } from '@playwright/test'

import type { TotpState } from './config'
import { totpCode, waitForFreshStep, waitForStepAfter } from './totp'

export async function signInAs(
  page: Page,
  email: string,
  password: string,
  path = '/login',
) {
  await page.goto(path)
  await page.getByLabel('Email').fill(email)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
}

export async function verifyTotpChallenge(
  page: Page,
  state: TotpState,
  success: Locator,
) {
  await expect(
    page.getByRole('heading', { name: 'Two-step verification', exact: true }),
  ).toBeVisible()

  for (let attempt = 0; attempt < 3; attempt += 1) {
    const step =
      state.confirmedStep > 0 ? await waitForStepAfter(state.confirmedStep) : await waitForFreshStep()
    await page.getByLabel('Authentication code').fill(totpCode(state.secret, step))
    await page.getByRole('button', { name: 'Verify and continue' }).click()
    try {
      await success.waitFor({ state: 'visible', timeout: 8_000 })
      state.confirmedStep = step
      return
    } catch {
      // A previous attempt may have consumed the step without saving state.
      state.confirmedStep = step
    }
  }
  throw new Error('TOTP challenge did not reach the expected page after three attempts')
}
