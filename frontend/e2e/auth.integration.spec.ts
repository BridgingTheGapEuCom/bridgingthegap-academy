import { execFileSync } from 'node:child_process'
import { expect, test } from '@playwright/test'

const email = 'auth-e2e@example.com'
const password = 'auth integration password'

function fixture(command: 'seed' | 'expire' | 'status') {
  const output = execFileSync('go', ['run', './tools/auth-e2e-seed', command], {
    cwd: process.cwd(),
    encoding: 'utf8',
    env: { ...process.env, BTG_AUTH_E2E_ALLOW_SEED: '1' },
    stdio: command === 'status' ? ['ignore', 'pipe', 'inherit'] : ['ignore', 'ignore', 'inherit'],
  })
  return command === 'status' ? JSON.parse(output) as { active: number; expired: number; revoked: number } : undefined
}

test.beforeEach(() => fixture('seed'))

test('real login persists across reload and real logout revokes the session', async ({ page }) => {
  await page.goto('/login?returnTo=/settings/account')
  await page.getByLabel(/^Email/).fill(email)
  await page.getByLabel(/^Password/).fill(password)
  await page.getByRole('button', { name: 'Sign in' }).click()

  await expect(page).toHaveURL('/settings/account')
  await expect(page.getByRole('button', { name: 'Sign out' })).toBeVisible()
  expect(fixture('status')?.active).toBe(1)
  await page.reload()
  await expect(page.getByRole('heading', { level: 1, name: 'Account' })).toBeVisible()

  await page.getByRole('button', { name: 'Sign out' }).click()
  await expect(page.getByRole('link', { name: 'Sign in' })).toBeVisible()
  expect(fixture('status')?.revoked).toBe(1)
  await page.goto('/settings/account')
  await expect(page).toHaveURL(/\/login\?returnTo=\/settings\/account$/)
})

test('a backend-expired session returns safely to localized login', async ({ page }) => {
  await page.goto('/login?returnTo=/settings/preferences')
  await page.getByLabel(/^Email/).fill(email)
  await page.getByLabel(/^Password/).fill(password)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page).toHaveURL('/settings/preferences')

  fixture('expire')
  expect(fixture('status')?.expired).toBe(1)
  await page.getByRole('radio', { name: 'Dark' }).check()
  await page.getByRole('button', { name: 'Save' }).click()

  await expect(page).toHaveURL(/\/login\?returnTo=\/settings\/preferences$/)
  await expect(page.getByRole('heading', { level: 2, name: 'Your session expired' })).toBeVisible()
  const sessionResponse = await page.request.get('/api/auth/session')
  expect(sessionResponse.status()).toBe(401)
})
