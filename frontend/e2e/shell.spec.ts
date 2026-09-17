import { test, expect } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

const unauthenticated = { type: 'about:blank', title: 'Unauthenticated', status: 401, instance: '/api/auth/session', request_id: 'test-request' }
const authenticated = { authenticated: true, user_id: '22222222-2222-4222-8222-222222222222', expires_at: '2026-09-17T12:00:00Z', csrf_token: 'test-csrf-token' }

test('application shell is keyboard accessible', async ({ page }) => {
  await page.route('**/api/auth/session', (route) => route.fulfill({
    status: 401,
    contentType: 'application/problem+json',
    body: JSON.stringify({ type: 'about:blank', title: 'Unauthenticated', status: 401, instance: '/api/auth/session', request_id: 'test-request' }),
  }))
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
  await page.keyboard.press('Tab')
  await expect(page.getByRole('link', { name: 'Skip to content' })).toBeFocused()
  await page.keyboard.press('Enter')
  await expect(page.locator('main')).toBeFocused()
  const result = await new AxeBuilder({ page }).analyze()
  expect(result.violations).toEqual([])
})

test('protected Authoring entry, expiry, and public Courses navigation remain predictable', async ({ page }) => {
  let signedIn = false
  await page.route('**/api/auth/session', (route) => route.fulfill(signedIn
    ? { status: 200, contentType: 'application/json', body: JSON.stringify(authenticated) }
    : { status: 401, contentType: 'application/problem+json', body: JSON.stringify(unauthenticated) },
  ))
  await page.route('**/api/auth/login', (route) => {
    signedIn = true
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(authenticated) })
  })
  await page.route('**/api/authoring/drafts', (route) => route.fulfill(signedIn
    ? { status: 200, contentType: 'application/json', body: JSON.stringify({ drafts: [] }) }
    : { status: 401, contentType: 'application/problem+json', body: JSON.stringify({ ...unauthenticated, instance: '/api/authoring/drafts' }) },
  ))
  await page.route('**/api/courses/catalog**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [], limit: 20, offset: 0, total: 0 }) }))

  await page.goto('/authoring')
  await expect(page).toHaveURL(/\/login\?returnTo=\/authoring$/)
  await expect(page.getByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Courses' })).toBeVisible()
  await page.getByLabel(/^Email/).fill('learner@example.com')
  await page.getByLabel(/^Password/).fill('test-only-password')
  await page.getByRole('button', { name: 'Sign in' }).press('Enter')
  await expect(page).toHaveURL('/authoring')
  await expect(page.getByLabel('Authentication status')).toHaveText('Signed in')
  await expect(page.getByRole('link', { name: 'Authoring' })).toBeVisible()

  signedIn = false
  await page.getByRole('button', { name: 'Create Draft' }).click()
  await expect(page).toHaveURL('/authoring/new')
  await page.getByRole('button', { name: 'Cancel' }).click()
  await expect(page).toHaveURL(/\/login\?returnTo=\/authoring$/)
  await expect(page.getByRole('link', { name: 'Authoring' })).toHaveCount(0)
  await page.getByRole('link', { name: 'Courses' }).click()
  await expect(page).toHaveURL('/courses')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})
