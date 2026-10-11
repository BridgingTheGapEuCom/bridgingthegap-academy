import { expect, test } from '@playwright/test'

const email = process.env.BTG_SETTINGS_E2E_EMAIL
const password = process.env.BTG_SETTINGS_E2E_PASSWORD

test('real authenticated Settings preferences persist and Account shows real session data', async ({ page }) => {
  test.skip(!email || !password, 'Set BTG_SETTINGS_E2E_EMAIL and BTG_SETTINGS_E2E_PASSWORD for a fresh real-stack test user')

  await page.goto('/login?returnTo=/settings/preferences')
  await page.getByLabel(/^Email/).fill(email!)
  await page.getByLabel(/^Password/).fill(password!)
  await page.getByRole('button', { name: 'Sign in' }).click()

  await expect(page).toHaveURL('/settings/preferences')
  await expect(page.getByRole('heading', { level: 1, name: 'Preferences' })).toBeVisible()
  await expect(page.getByText('Preferences unavailable')).toHaveCount(0)
  await expect(page.getByRole('combobox', { name: 'Language' })).toHaveValue('')
  await expect(page.getByRole('radio', { name: 'System' })).toBeChecked()

  await page.getByRole('radio', { name: 'Dark' }).check()
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('status')).toContainText('Preferences saved.')
  await page.reload()
  await expect(page.getByRole('radio', { name: 'Dark' })).toBeChecked()

  await page.getByRole('radio', { name: 'Light' }).check()
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('status')).toContainText('Preferences saved.')
  await page.reload()
  await expect(page.getByRole('radio', { name: 'Light' })).toBeChecked()

  await page.getByRole('link', { name: 'Account' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'Account' })).toBeVisible()
  const expiry = page.locator('.btg-metadata-strip__item').filter({ hasText: 'Session expires' }).locator('dd')
  await expect(expiry).not.toHaveText('')
  await expect(expiry).not.toHaveText('Not available')
})
