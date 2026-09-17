import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

const unauthenticated = {
  type: 'about:blank',
  title: 'Unauthenticated',
  status: 401,
  instance: '/api/auth/session',
  request_id: 'test-request',
}

const authenticatedSession = {
  authenticated: true,
  user_id: '3c5e54ca-37fb-4af0-938d-694a1815d1ed',
  expires_at: '2026-09-15T12:00:00Z',
  csrf_token: 'test-csrf-token',
}

async function serveUnauthenticatedSession(page: Page) {
  await page.route('**/api/auth/session', (route) => route.fulfill({
    status: 401,
    contentType: 'application/problem+json',
    body: JSON.stringify(unauthenticated),
  }))
}

test('administrator journey uses backend access confirmation and signs out', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 })
  let signedIn = false
  await page.route('**/api/auth/session', (route) => route.fulfill(signedIn
    ? { status: 200, contentType: 'application/json', body: JSON.stringify(authenticatedSession) }
    : { status: 401, contentType: 'application/problem+json', body: JSON.stringify(unauthenticated) },
  ))
  await page.route('**/api/auth/login', (route) => {
    signedIn = true
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(authenticatedSession) })
  })
  await page.route('**/api/admin/status', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ status: 'ok' }),
  }))
  await page.route('**/api/auth/logout', (route) => {
    signedIn = false
    return route.fulfill({ status: 204 })
  })

  await page.goto('/admin')
  await expect(page).toHaveURL(/\/login(?:\?returnTo=\/admin)?$/)
  await page.getByLabel(/^Email/).fill('admin@example.com')
  await page.getByLabel(/^Password/).fill('test-only-password')
  await page.getByLabel(/^Password/).press('Enter')
  await expect(page).toHaveURL(/\/admin$/)
  await expect(page.locator('main')).toBeFocused()

  await page.goto('/admin')
  await expect(page.getByRole('heading', { level: 1, name: 'Administration' })).toBeVisible()
  await expect(page.getByText('System status: OK')).toBeVisible()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  for (const width of [768, 390, 320]) {
    await page.setViewportSize({ width, height: 844 })
    await expect(page.getByRole('button', { name: 'Sign out' })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  }

  await page.getByRole('banner').getByRole('button', { name: 'Sign out' }).focus()
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(/\/login$/)
  await expect(page.locator('main')).toBeFocused()

  await page.goBack()
  await expect(page.getByRole('heading', { level: 1, name: 'Administration' })).toHaveCount(0)

  await page.goto('/admin')
  await expect(page).toHaveURL(/\/login(?:\?returnTo=\/admin)?$/)
})

test('an authenticated non-administrator sees access denied and remains signed in', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.route('**/api/auth/session', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(authenticatedSession),
  }))
  await page.route('**/api/admin/status', (route) => route.fulfill({
    status: 403,
    contentType: 'application/problem+json',
    body: JSON.stringify({ ...unauthenticated, status: 403, title: 'Forbidden', instance: '/api/admin/status' }),
  }))

  await page.goto('/admin')
  await expect(page.getByRole('heading', { level: 1, name: 'Access denied' })).toBeVisible()
  await expect(page).toHaveURL(/\/admin$/)
  await expect(page.getByText('Your account does not have permission to access Administration.')).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await page.setViewportSize({ width: 320, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.getByRole('button', { name: 'Return home' }).focus()
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(/\/$/)
})

test('returning to Administration rechecks backend authorization', async ({ page }) => {
  await page.route('**/api/auth/session', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(authenticatedSession),
  }))
  let allowed = true
  let checks = 0
  await page.route('**/api/admin/status', (route) => {
    checks += 1
    return route.fulfill(allowed
      ? { status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'ok' }) }
      : { status: 403, contentType: 'application/problem+json', body: JSON.stringify({ ...unauthenticated, status: 403, title: 'Forbidden' }) },
    )
  })

  await page.goto('/admin')
  await expect(page.getByRole('heading', { level: 1, name: 'Administration' })).toBeVisible()
  await page.getByRole('button', { name: 'Return home' }).click()
  await expect(page).toHaveURL(/\/$/)

  allowed = false
  await page.goBack()
  await expect(page.getByRole('heading', { level: 1, name: 'Access denied' })).toBeVisible()
  expect(checks).toBe(2)
})

test('admin and logout operational failures keep the session and provide a retry', async ({ page }) => {
  await page.route('**/api/auth/session', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(authenticatedSession),
  }))
  let accessChecks = 0
  await page.route('**/api/admin/status', (route) => {
    accessChecks += 1
    return route.fulfill(accessChecks === 1
      ? { status: 500, contentType: 'application/problem+json', body: JSON.stringify({ ...unauthenticated, status: 500, title: 'Unavailable' }) }
      : { status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'ok' }) },
    )
  })
  let logoutAttempts = 0
  await page.route('**/api/auth/logout', (route) => {
    logoutAttempts += 1
    return route.fulfill(logoutAttempts === 1
      ? { status: 500, contentType: 'application/problem+json', body: JSON.stringify({ ...unauthenticated, status: 500, title: 'Unavailable' }) }
      : { status: 204 },
    )
  })

  await page.goto('/admin')
  await expect(page.getByRole('heading', { level: 1, name: 'Administration unavailable' })).toBeVisible()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.getByRole('button', { name: 'Try again' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'Administration' })).toBeVisible()

  await page.getByRole('banner').getByRole('button', { name: 'Sign out' }).click()
  const logoutError = page.getByText('We couldn’t sign you out right now. Please try again.')
  await expect(logoutError).toBeFocused()
  await expect(page.getByRole('heading', { level: 1, name: 'Administration' })).toBeVisible()
  await expect(page).toHaveURL(/\/admin$/)
  await page.getByRole('banner').getByRole('button', { name: 'Sign out' }).click()
  await expect(page).toHaveURL(/\/login$/)
})
