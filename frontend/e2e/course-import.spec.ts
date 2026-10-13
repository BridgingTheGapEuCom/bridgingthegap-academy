import { expect, test, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { pseudoLocalize } from '../src/i18n/pseudo'

const session = { authenticated: true, user_id: '11111111-1111-4111-8111-111111111111', expires_at: '2099-09-26T12:00:00Z', csrf_token: 'test-csrf-token' }
const preview = { previewToken: 'a'.repeat(64), preview: { format: 'bridging-the-gap-course', formatVersion: 1, title: 'A very long imported course title that must remain readable in Administration', version: '1.2.3', language: 'en', license: { kind: 'STANDARD', displayName: 'Creative Commons Attribution 4.0' }, attribution: [{ displayName: 'A very long public author name that wraps safely', role: 'AUTHOR', order: 0 }], moduleCount: 12, lessonCount: 48, assessmentCount: 4, assetCount: 7, assetBytes: 1536000, translationLanguages: ['es'] } }
const imported = { importId: '11111111-1111-4111-8111-111111111111', courseId: '22222222-2222-4222-8222-222222222222', courseVersionId: '33333333-3333-4333-8333-333333333333', semVer: '1.2.3', status: 'IMPORTED', translationLanguages: ['es'] }

async function serve(page: Page) {
  await page.route('**/api/auth/session', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify(session) }))
  await page.route('**/api/me/preferences', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ applicationLocale: null, appearance: 'SYSTEM', revision: 0, updatedAt: null }) }))
  await page.route('**/api/portability/imports/preview', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify(preview) }))
  await page.route('**/api/portability/imports/*/execute', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify(imported) }))
}

test('course import uses the accessible file field and responsive shared states', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' })
  await serve(page)
  await page.goto('/admin/course-import')
  await page.getByLabel(/Course package/).setInputFiles({ name: 'course.zip', mimeType: 'application/zip', buffer: Buffer.from('zip') })
  await page.getByRole('button', { name: 'Validate package' }).click()
  await expect(page.getByRole('heading', { name: 'Review course package' })).toBeVisible()
  await page.getByRole('button', { name: 'Import course' }).click()
  await expect(page.getByRole('heading', { name: 'Course imported' })).toBeVisible()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.setViewportSize({ width: 320, height: 844 })
  await page.evaluate(() => { document.documentElement.style.fontSize = '200%' })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
})

test('course import Academy copy is pseudo-localized', async ({ page }) => {
  await serve(page)
  await page.addInitScript(() => localStorage.setItem('btg.academy.application-locale', 'en-XA'))
  await page.goto('/admin/course-import')
  await expect(page.getByRole('heading', { level: 1, name: pseudoLocalize('Import course package') })).toBeVisible()
  await expect(page.locator('html')).toHaveAttribute('lang', 'en-XA')
})
