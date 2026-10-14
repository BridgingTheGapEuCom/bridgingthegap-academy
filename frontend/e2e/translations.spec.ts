import { expect, test, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { pseudoLocalize } from '../src/i18n/pseudo'

const courseId = '10000000-0000-4000-8000-000000000001'
const translationId = '20000000-0000-4000-8000-000000000001'
const source = { courseId, version: '1.0.0', title: 'A deliberately long immutable source course title that remains fully readable', description: 'Source course description', objectives: ['Source objective'], sourceLanguage: 'en', changelog: 'Source release', license: { kind: 'STANDARD', identifier: '', displayName: 'License', url: '', customText: '' }, contributors: [], publishedAt: '2026-10-01T00:00:00Z', modules: [], assessments: [] }
const text = (value: string) => ({ source: value, translated: null, state: 'UNTRANSLATED' })
const workspace = { translationId, revision: 2, lifecycle: 'DRAFT', source: { courseId, courseVersionId: '30000000-0000-4000-8000-000000000001', version: '1.0.0', language: 'en' }, targetLanguage: 'es', course: { title: text(source.title), description: text('A long source description that must remain visible beside its editable translation.'), learningObjectives: [text('Explain exact source coupling')] }, modules: [{ sourceStableKey: 'module-one', position: 0, title: text('A long immutable module title for responsive source comparison'), description: text('Module description'), completeness: { totalTranslatableFields: 5, translatedFields: 0, untranslatedFields: 5, complete: false }, lessons: [{ sourceStableKey: 'lesson-one', position: 0, prerequisiteStableKeys: [], title: text('A long immutable lesson title for responsive source comparison'), description: text('Lesson description'), learningObjectives: [text('Lesson objective')], blocks: [{ sourceBlockKey: 'text-one', type: 'TEXT', position: 0, fields: { text: text('User-authored source content is not application-localized.') }, assetKeys: [], code: '', tableHeaders: [], tableRows: [], assessmentKey: '' }], completeness: { totalTranslatableFields: 4, translatedFields: 0, untranslatedFields: 4, complete: false } }] }], assessments: [], completeness: { totalTranslatableFields: 10, translatedFields: 0, untranslatedFields: 10, complete: false } }

async function session(page: Page) {
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ authenticated: true, user_id: '40000000-0000-4000-8000-000000000001', csrf_token: 'csrf', expires_at: '2027-01-01T00:00:00Z' }) }))
  await page.route('**/api/me/preferences', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ applicationLocale: null, appearance: 'SYSTEM', revision: 0, updatedAt: null }) }))
}

test('translation discovery and creation use shared responsive accessible patterns', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' })
  await session(page)
  await page.route(`**/api/courses/by-id/${courseId}/versions/1.0.0**`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(route.request().url().endsWith('/translations') ? { translations: [{ translationId, targetLanguage: 'es', lifecycle: 'DRAFT', revision: 2, completeness: workspace.completeness }] } : source) }))
  await page.goto(`/authoring/courses/${courseId}/versions/1.0.0/translations`)
  await expect(page.getByRole('heading', { level: 1, name: 'Translations' })).toBeVisible()
  await expect(page.getByText(source.title)).toBeVisible()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.setViewportSize({ width: 320, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await page.evaluate(() => { document.documentElement.style.fontSize = '200%' })
  const overflowing = await page.evaluate(() => [...document.querySelectorAll<HTMLElement>('body *')]
    .filter((element) => element.getBoundingClientRect().right > window.innerWidth + 1)
    .slice(0, 12)
    .map((element) => ({ element: `${element.tagName.toLowerCase()}.${element.className}`, left: element.getBoundingClientRect().left, right: element.getBoundingClientRect().right, width: element.getBoundingClientRect().width })))
  expect(overflowing).toEqual([])
})

test('translation workspace stacks source and target, preserves authored content, and localizes conflict handling', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' })
  await session(page)
  await page.addInitScript(() => localStorage.setItem('btg.academy.application-locale', 'en-XA'))
  await page.route(`**/api/translations/${translationId}`, (route) => route.request().method() === 'PATCH'
    ? route.fulfill({ status: 409, contentType: 'application/problem+json', body: JSON.stringify({ type: 'about:blank', title: 'Conflict', status: 409, code: 'translation_revision_conflict' }) })
    : route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(workspace) }))
  await page.goto(`/authoring/translations/${translationId}`)
  await expect(page.getByText('User-authored source content is not application-localized.')).toBeVisible()
  await expect(page.getByText(pseudoLocalize('Course metadata'))).toBeVisible()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  const titleInput = page.getByRole('group', { name: pseudoLocalize('Course title') }).getByRole('textbox')
  await titleInput.fill('Título local')
  await page.getByRole('button', { name: pseudoLocalize('Save changes') }).click()
  await expect(page.getByRole('alert')).toContainText(pseudoLocalize('This translation changed elsewhere. Your unsaved edits are still on this page. Reload the latest translation before saving.'))
  await expect(titleInput).toHaveValue('Título local')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.setViewportSize({ width: 320, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await page.evaluate(() => { document.documentElement.style.fontSize = '200%' })
  const overflowing = await page.evaluate(() => [...document.querySelectorAll<HTMLElement>('body *')]
    .filter((element) => element.getBoundingClientRect().right > window.innerWidth + 1)
    .slice(0, 12)
    .map((element) => ({ element: `${element.tagName.toLowerCase()}.${element.className}`, left: element.getBoundingClientRect().left, right: element.getBoundingClientRect().right, width: element.getBoundingClientRect().width })))
  expect(overflowing).toEqual([])
})
