import { test, expect } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

const unauthenticated = { type: 'about:blank', title: 'Unauthenticated', status: 401, instance: '/api/auth/session', request_id: 'test-request' }
const firstCourse = {
  courseId: '10000000-0000-4000-8000-000000000001', version: '1.0.0', title: 'Integration foundations', description: 'A practical published course.', sourceLanguage: 'en',
  license: { kind: 'STANDARD', identifier: 'CC-BY-4.0', displayName: 'Creative Commons Attribution 4.0', url: 'https://creativecommons.org/licenses/by/4.0/', customText: '' }, contributors: [], publishedAt: '2026-09-26T12:00:00Z',
}
const secondCourse = { ...firstCourse, courseId: '20000000-0000-4000-8000-000000000002', title: 'Event boundaries', version: '1.1.0' }
test('homepage presents a calm, accessible catalog entry point', async ({ page }) => {
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 401, contentType: 'application/problem+json', body: JSON.stringify(unauthenticated) }))
  await page.route('**/api/courses/catalog**', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [firstCourse, secondCourse], limit: 2, offset: 0, total: 3 }) }))
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: /Learn integration architecture/i })).toBeVisible()
  await expect(page.getByRole('heading', { level: 2, name: 'What you can learn' })).toBeVisible()
  await expect(page.getByRole('heading', { level: 2, name: 'Designed for focused learning' })).toBeVisible()
  await expect(page.locator('.home-page__hero-motif[aria-hidden="true"]')).toBeVisible()
  await expect(page.getByRole('link', { name: 'Integration foundations' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Event boundaries' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Messaging patterns' })).toHaveCount(0)
  expect(await page.locator('.home-page').evaluate((element) => {
    const grayscale = (value: string) => {
      const channels = value.match(/\d+/g)?.slice(0, 3).map(Number)
      return channels?.length === 3 && channels[0] === channels[1] && channels[1] === channels[2]
    }
    const primary = document.querySelector<HTMLElement>('.home-page .btg-button--primary')!
    return [getComputedStyle(element).color, getComputedStyle(element).backgroundColor, getComputedStyle(primary).backgroundColor].every(grayscale)
  })).toBe(true)
  await page.keyboard.press('Tab')
  await expect(page.getByRole('link', { name: 'Skip to content' })).toBeFocused()
  await page.getByRole('link', { name: 'Browse courses' }).first().click()
  await expect(page).toHaveURL('/courses')

  await page.goto('/')
  await page.setViewportSize({ width: 320, height: 844 })
  await page.locator('html').evaluate((element) => { element.style.fontSize = '200%' })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('homepage gives an accessible catalog path when no featured courses are published', async ({ page }) => {
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 401, contentType: 'application/problem+json', body: JSON.stringify(unauthenticated) }))
  await page.route('**/api/courses/catalog**', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ items: [], limit: 2, offset: 0, total: 0 }) }))
  await page.goto('/')

  await expect(page.getByRole('status')).toContainText('Courses are being prepared.')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.getByRole('status').getByRole('link', { name: 'Browse courses' }).click()
  await expect(page).toHaveURL('/courses')
})
