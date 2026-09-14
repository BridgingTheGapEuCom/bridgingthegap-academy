import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

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
