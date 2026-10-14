import { execFileSync } from 'node:child_process'
import { expect, test } from '@playwright/test'

type Fixture = { email: string; password: string; courseId: string; courseVersionId: string; version: string; sourceLanguage: string; targetLanguage: string; sourceTitle: string; expectedTranslatedTitle: string; translationId?: string; translationRevision?: number; storedTranslatedTitle?: string; sourceUnchanged?: boolean; sourceBindingValid?: boolean }
const databaseURL = process.env.BTG_LMS_DATABASE_URL
const enabled = process.env.BTG_TRANSLATION_E2E_REAL === '1' && Boolean(databaseURL)

function fixture(command: 'seed' | 'status'): Fixture {
  return JSON.parse(execFileSync('go', ['run', './tools/translation-e2e-seed', command], {
    cwd: process.cwd(), encoding: 'utf8', env: { ...process.env, BTG_TRANSLATION_E2E_ALLOW_SEED: '1', BTG_LMS_DATABASE_URL: databaseURL ?? '' },
  }).trim()) as Fixture
}

test('real author creates and persists an exact-source translation', async ({ page }) => {
  test.skip(!enabled, 'Set BTG_TRANSLATION_E2E_REAL=1 and BTG_LMS_DATABASE_URL for the real translation flow')
  const seeded = fixture('seed')

  await page.goto(`/login?returnTo=/authoring/courses/${seeded.courseId}/versions/${seeded.version}/translations`)
  await page.getByLabel(/^Email/).fill(seeded.email)
  await page.getByLabel(/^Password/).fill(seeded.password)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page).toHaveURL(`/authoring/courses/${seeded.courseId}/versions/${seeded.version}/translations`)
  await expect(page.locator('html')).toHaveAttribute('lang', 'en')
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  await expect(page.getByText(seeded.sourceTitle)).toBeVisible()
  await expect(page.getByText(/English \(en\)/)).toBeVisible()

  const picker = page.getByRole('combobox', { name: /Target language/ })
  await picker.focus()
  await picker.fill(seeded.targetLanguage)
  await picker.press('Enter')
  const creation = page.waitForResponse((response) => response.request().method() === 'POST' && response.url().endsWith(`/versions/${seeded.version}/translations`))
  const workspaceLoad = page.waitForResponse((response) => response.request().method() === 'GET' && /\/api\/translations\/[0-9a-f-]+$/.test(new URL(response.url()).pathname))
  await page.getByRole('button', { name: 'Create translation' }).click()
  expect((await creation).status()).toBe(201)
  await expect(page).toHaveURL(/\/authoring\/translations\/[0-9a-f-]+$/)
  const workspaceResponse = await workspaceLoad
  const workspaceBody = await workspaceResponse.text()
  expect(workspaceResponse.status(), workspaceBody).toBe(200)

  await expect(page.getByText(seeded.sourceTitle).first(), workspaceBody).toBeVisible()
  await page.getByRole('textbox', { name: 'Course title translation' }).fill(seeded.expectedTranslatedTitle)
  const save = page.waitForResponse((response) => response.request().method() === 'PATCH' && /\/api\/translations\/[0-9a-f-]+$/.test(new URL(response.url()).pathname))
  await page.getByRole('button', { name: 'Save changes' }).click()
  expect((await save).status()).toBe(200)
  await expect(page.getByText('Translation saved.')).toBeVisible()

  await page.reload()
  await expect(page.getByRole('textbox', { name: 'Course title translation' })).toHaveValue(seeded.expectedTranslatedTitle)
  await expect(page.getByText(seeded.sourceTitle).first()).toBeVisible()
  const status = fixture('status')
  expect(status).toMatchObject({ storedTranslatedTitle: seeded.expectedTranslatedTitle, sourceUnchanged: true, sourceBindingValid: true, sourceLanguage: 'en', targetLanguage: seeded.targetLanguage })
  expect(status.translationRevision).toBe(2)
})
