import { test, expect } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

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
