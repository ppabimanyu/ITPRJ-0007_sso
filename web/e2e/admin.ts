import { expect, type Page } from '@playwright/test'

export async function createInvitedUser(page: Page, email: string, name: string) {
  await page.goto('/admin/users')
  await expect(page.getByRole('heading', { name: 'Users', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Create user', exact: true }).click()

  const dialog = page.getByRole('dialog')
  await expect(dialog.getByRole('heading', { name: 'Create a user', exact: true })).toBeVisible()
  await dialog.getByLabel('Email', { exact: true }).fill(email)
  await dialog.getByLabel('Full name', { exact: true }).fill(name)
  await dialog.getByRole('button', { name: 'Create user', exact: true }).click()

  await expect(dialog.getByRole('heading', { name: 'User created', exact: true })).toBeVisible()
  await dialog.getByRole('button', { name: 'Done', exact: true }).click()
  await expect(dialog).toBeHidden()
}

export async function createRole(page: Page, name: string) {
  await page.goto('/admin/roles')
  await expect(page.getByRole('heading', { name: 'Roles', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Create role', exact: true }).click()

  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Name', { exact: true }).fill(name)
  await dialog.getByRole('button', { name: 'Create role', exact: true }).click()

  await expect(dialog).toBeHidden()
  await expect(page.getByRole('row').filter({ hasText: name })).toBeVisible()
}

export async function createGroup(page: Page, name: string, description: string) {
  await page.goto('/admin/groups')
  await expect(page.getByRole('heading', { name: 'Groups', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Create group', exact: true }).click()

  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Name', { exact: true }).fill(name)
  await dialog.getByLabel('Description', { exact: true }).fill(description)
  await dialog.getByRole('button', { name: 'Create group', exact: true }).click()

  await expect(dialog).toBeHidden()
  await expect(page.getByRole('row').filter({ hasText: name })).toBeVisible()
}
