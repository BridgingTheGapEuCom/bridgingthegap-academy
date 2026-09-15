import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

const draftID = '11111111-1111-4111-8111-111111111111'
const session = { user_id: '22222222-2222-4222-8222-222222222222', expires_at: '2026-09-15T16:00:00Z', csrf_token: 'test-csrf-token' }
const draft = {
  id: draftID,
  course_id: '33333333-3333-4333-8333-333333333333',
  intended_version: '1.0.0',
  source_language: 'en',
  title: 'Integration foundations',
  description: 'A mutable working version.',
  objectives: ['Explain ownership'],
  changelog: 'Initial Draft.',
  license: {
    kind: 'STANDARD',
    identifier: 'CC-BY-4.0',
    display_name: 'Creative Commons Attribution 4.0',
    url: 'https://creativecommons.org/licenses/by/4.0/',
    custom_text: '',
  },
  status: 'ACTIVE',
  revision: 3,
  created_at: '2026-09-15T10:00:00Z',
  updated_at: '2026-09-15T11:00:00Z',
}

async function serveDraft(page: Page) {
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session) }))
  await page.route(`**/api/authoring/drafts/${draftID}`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(draft) }))
}

test('Authoring Draft shell is private, accessible, and responsive', async ({ page }) => {
  await serveDraft(page)
  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.goto(`/authoring/drafts/${draftID}`)
  await expect(page.getByRole('heading', { level: 1, name: 'Integration foundations' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Overview' })).toHaveAttribute('aria-current', 'page')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.getByRole('link', { name: 'Structure', exact: true }).focus()
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(`/authoring/drafts/${draftID}/structure`)
  await expect(page.getByRole('heading', { level: 2, name: 'Structure' })).toBeVisible()

  await page.setViewportSize({ width: 390, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
})

test('Authoring metadata editor saves a changed field with its current revision', async ({ page }) => {
  let patchBody: unknown
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session) }))
  await page.route(`**/api/authoring/drafts/${draftID}`, (route) => {
    if (route.request().method() === 'PATCH') {
      patchBody = route.request().postDataJSON()
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ...draft, title: 'Updated foundations', revision: 4, updated_at: '2026-09-15T12:00:00Z' }),
      })
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(draft) })
  })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(`/authoring/drafts/${draftID}`)
  const title = page.getByRole('textbox', { name: /^Title\b/ })
  await title.fill('Updated foundations')
  await page.getByRole('button', { name: 'Save changes' }).click()

  await expect(page.getByRole('heading', { level: 1, name: 'Updated foundations' })).toBeVisible()
  expect(patchBody).toEqual({ expectedRevision: 3, title: 'Updated foundations' })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})
