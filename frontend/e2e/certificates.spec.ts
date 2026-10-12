import { expect, test, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { pseudoLocalize } from '../src/i18n/pseudo'

const id = '22222222-2222-4222-8222-222222222222'
const achievement = {
  courseId: '11111111-1111-4111-8111-111111111111',
  title: 'A deliberately long authored course title for resilient certificate layouts',
  version: '1.0.0',
  language: 'en',
  criteria: 'Complete every lesson in the published course version.',
}
const publicCertificate = { certificateId: id, status: 'ACTIVE', issuedAt: '2026-01-01T00:00:00Z', revokedAt: null, achievement, issuer: { id: 'https://academy.example', name: 'Academy' } }
const learnerCertificate = { ...publicCertificate, verificationPath: `/verify/certificates/${id}` }

async function session(page: Page, authenticated: boolean) {
  await page.route('**/api/auth/session', (route) => route.fulfill(authenticated
    ? { status: 200, contentType: 'application/json', body: JSON.stringify({ authenticated: true, user_id: '33333333-3333-4333-8333-333333333333', csrf_token: 'csrf', expires_at: '2027-01-01T00:00:00Z' }) }
    : { status: 401, contentType: 'application/problem+json', body: '{}' }))
  await page.route('**/api/me/preferences', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ applicationLocale: null, appearance: 'SYSTEM', revision: 0, updatedAt: null }) }))
}

test('private certificate uses shared localized and responsive presentation', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' })
  await session(page, true)
  await page.route(`**/api/learner/certificates/${id}`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(learnerCertificate) }))
  await page.goto(`/certificates/${id}`)
  await expect(page.getByRole('heading', { level: 1, name: achievement.title })).toBeVisible()
  await expect(page.getByText('Valid', { exact: true })).toBeVisible()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  for (const width of [320, 390]) {
    await page.setViewportSize({ width, height: 844 })
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  }
  await page.evaluate(() => { document.documentElement.style.fontSize = '200%' })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
})

test('public verification is anonymous, recipient-free, and pseudo-localized', async ({ page }) => {
  await session(page, false)
  await page.route(`**/api/public/certificates/${id}`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(publicCertificate) }))
  await page.addInitScript(() => localStorage.setItem('btg.academy.application-locale', 'en-XA'))
  await page.goto(`/verify/certificates/${id}`)
  await expect(page.getByRole('heading', { level: 1, name: pseudoLocalize('Certificate verification') })).toBeVisible()
  await expect(page.getByRole('heading', { level: 2, name: achievement.title })).toBeVisible()
  await expect(page.getByText(/email|user id|learner id/i)).toHaveCount(0)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('public verification distinguishes an unknown certificate', async ({ page }) => {
  await session(page, false)
  await page.route(`**/api/public/certificates/${id}`, (route) => route.fulfill({ status: 404, contentType: 'application/problem+json', body: '{}' }))
  await page.goto(`/verify/certificates/${id}`)
  await expect(page.getByRole('heading', { level: 1, name: 'Certificate not found' })).toBeVisible()
  await expect(page.getByText('No certificate is recorded for this identifier.')).toBeVisible()
})

test('public verification keeps transport failure distinct from invalidity', async ({ page }) => {
  await session(page, false)
  await page.route(`**/api/public/certificates/${id}`, (route) => route.fulfill({ status: 503, contentType: 'application/problem+json', body: '{}' }))
  await page.goto(`/verify/certificates/${id}`)
  await expect(page.getByRole('heading', { level: 1, name: 'Verification unavailable' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Try again' })).toBeVisible()
})
