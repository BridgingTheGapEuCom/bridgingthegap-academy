import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { pseudoLocalize } from '../src/i18n/pseudo'

const unauthenticatedSession = {
  type: 'about:blank',
  title: 'Unauthenticated',
  status: 401,
  instance: '/api/auth/session',
  request_id: 'test-request',
}

async function serveUnauthenticatedSession(page: Page) {
  await page.route('**/api/auth/session', (route) => route.fulfill({
    status: 401,
    contentType: 'application/problem+json',
    body: JSON.stringify(unauthenticatedSession),
  }))
}

test('login is labelled, keyboard-operable, and reports invalid credentials generically', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 })
  await serveUnauthenticatedSession(page)
  await page.route('**/api/auth/login', (route) => route.fulfill({
    status: 401,
    contentType: 'application/problem+json',
    body: JSON.stringify({ ...unauthenticatedSession, instance: '/api/auth/login' }),
  }))
  await page.goto('/login')

  await expect(page.getByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible()
  await expect(page.getByLabel(/^Email/)).toBeVisible()
  await expect(page.getByLabel(/^Password/)).toBeVisible()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.getByLabel(/^Email/).focus()
  await page.keyboard.press('Tab')
  await expect(page.getByLabel(/^Password/)).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(page.getByRole('button', { name: 'Sign in' })).toBeFocused()

  await page.getByLabel(/^Email/).fill('learner@example.com')
  await page.getByLabel(/^Password/).fill('private password')
  await page.getByLabel(/^Password/).press('Enter')

  const error = page.getByText('The email or password is incorrect.')
  await expect(error).toHaveText('The email or password is incorrect.')
  await expect(error).toBeFocused()
  await expect(page.getByLabel(/^Email/)).toHaveValue('learner@example.com')
  await expect(page.getByLabel(/^Password/)).toHaveValue('')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('login reflows from tablet width down to a 320px CSS viewport', async ({ page }) => {
  await page.setViewportSize({ width: 768, height: 844 })
  await serveUnauthenticatedSession(page)
  await page.goto('/login')

  for (const width of [768, 390, 320]) {
    await page.setViewportSize({ width, height: 844 })
    await expect(page.getByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible()
    await expect(page.getByLabel(/^Email/)).toBeVisible()
    await expect(page.getByLabel(/^Password/)).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  }
})

test('login localizes rate limits and remains usable in dark mode at 200% text', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' })
  await page.addInitScript(() => localStorage.setItem('btg.academy.application-locale', 'en-XA'))
  await serveUnauthenticatedSession(page)
  await page.route('**/api/auth/login', (route) => route.fulfill({
    status: 429,
    headers: { 'Retry-After': '20' },
    contentType: 'application/problem+json',
    body: JSON.stringify({ ...unauthenticatedSession, status: 429, title: 'Too Many Requests', instance: '/api/auth/login' }),
  }))
  await page.goto('/login')

  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect(page.getByRole('heading', { level: 1, name: pseudoLocalize('Sign in') })).toBeVisible()
  await page.getByLabel(pseudoLocalize('Email'), { exact: false }).fill('learner@example.com')
  await page.getByLabel(pseudoLocalize('Password'), { exact: false }).fill('private password')
  await page.getByRole('button', { name: pseudoLocalize('Sign in') }).click()
  await expect(page.getByRole('alert')).toContainText('20')
  await expect(page.getByRole('alert')).not.toContainText('Too many sign-in attempts.')

  await page.setViewportSize({ width: 320, height: 844 })
  await page.locator('html').evaluate((element) => { element.style.fontSize = '200%' })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})
