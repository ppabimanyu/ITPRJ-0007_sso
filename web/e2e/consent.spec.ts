import { expect, test } from '@playwright/test'
import { createHash, randomBytes, randomUUID } from 'node:crypto'

import { userStatePath } from './config'

test.use({ storageState: userStatePath })

test('user approves a sample-spa authorization request', async ({ page }) => {
  const state = randomUUID()
  const verifier = randomBytes(32).toString('base64url')
  const challenge = createHash('sha256').update(verifier).digest('base64url')

  let callbackURL: string | null = null
  await page.route('http://localhost:5173/**', async (route) => {
    callbackURL = route.request().url()
    await route.fulfill({
      status: 200,
      contentType: 'text/html',
      body: '<!doctype html><title>E2E callback</title><h1>E2E callback</h1>',
    })
  })

  const query = new URLSearchParams({
    response_type: 'code',
    client_id: 'sample-spa',
    redirect_uri: 'http://localhost:5173/callback',
    scope: 'openid profile email offline_access',
    state,
    nonce: randomUUID(),
    code_challenge: challenge,
    code_challenge_method: 'S256',
  })
  await page.goto(`/realms/default/authorize?${query.toString()}`)

  const consentTitle = page.getByRole('heading', { name: 'Grant access', exact: true })
  const continueButton = page.getByRole('button', { name: 'Continue', exact: true })
  await expect(consentTitle.or(continueButton)).toBeVisible()
  if (await consentTitle.isVisible()) {
    await page.getByRole('button', { name: 'Allow access', exact: true }).click()
  } else {
    await continueButton.click()
  }

  await expect.poll(() => callbackURL).not.toBeNull()
  const callback = new URL(callbackURL ?? '')
  expect(callback.origin + callback.pathname).toBe('http://localhost:5173/callback')
  expect(callback.searchParams.get('state')).toBe(state)
  expect(callback.searchParams.get('code')).toBeTruthy()

  await expect(page).toHaveURL(/^http:\/\/localhost:5173\/callback\?/)
  await expect(page.getByRole('heading', { name: 'E2E callback', exact: true })).toBeVisible()
})
