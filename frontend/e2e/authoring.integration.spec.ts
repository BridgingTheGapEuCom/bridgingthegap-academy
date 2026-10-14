import { execFileSync } from 'node:child_process'
import { expect, test } from '@playwright/test'

type Fixture = { email: string; password: string; title: string; sourceLanguage: string }
type Status = Fixture & { draftExists: boolean; draftId: string; courseId: string; membershipRole: string; creatorOwnsDraft: boolean; accessibleDrafts: number }

function fixture(command: 'seed' | 'status'): Fixture & Status {
  const output = execFileSync('go', ['run', './tools/authoring-e2e-seed', command], {
    cwd: process.cwd(), encoding: 'utf8', env: { ...process.env, BTG_AUTHORING_E2E_ALLOW_SEED: '1' }, stdio: ['ignore', 'pipe', 'inherit'],
  })
  return JSON.parse(output) as Fixture & Status
}

test('real author creates and rediscovers a persisted Draft', async ({ page }) => {
  const seeded = fixture('seed')
  await page.goto('/login?returnTo=/authoring')
  await page.getByLabel(/^Email/).fill(seeded.email)
  await page.getByLabel(/^Password/).fill(seeded.password)
  await page.getByRole('button', { name: 'Sign in' }).click()

  await expect(page).toHaveURL('/authoring')
  await expect(page.getByRole('heading', { level: 3, name: 'No course Drafts yet' })).toBeVisible()
  await page.getByRole('button', { name: 'Create Draft' }).click()
  await expect(page).toHaveURL('/authoring/new')

  await page.getByRole('textbox', { name: /^Title required$/ }).fill(seeded.title)
  const language = page.getByRole('combobox', { name: /^Source language required$/ })
  await language.click()
  await language.fill(seeded.sourceLanguage)
  await page.getByRole('option', { name: new RegExp(`\\(${seeded.sourceLanguage}\\)$`) }).click()
  await page.getByRole('textbox', { name: /^Description required$/ }).fill('A real-stack Draft created through the Academy authoring entry flow.')
  await page.getByRole('textbox', { name: 'Learning objective 1' }).fill('Verify authoring persistence')
  await page.getByRole('button', { name: 'Create Draft' }).click()

  await expect(page).toHaveURL(/\/authoring\/drafts\/[0-9a-f-]+\/overview$/)
  await expect(page.getByRole('heading', { level: 1, name: seeded.title })).toBeVisible()
  await page.reload()
  await expect(page.getByRole('heading', { level: 1, name: seeded.title })).toBeVisible()

  await page.goto('/authoring')
  await expect(page.getByRole('link', { name: `Open Draft: ${seeded.title}` })).toBeVisible()
  const status = fixture('status')
  expect(status).toMatchObject({ draftExists: true, sourceLanguage: seeded.sourceLanguage, membershipRole: 'MAINTAINER', creatorOwnsDraft: true, accessibleDrafts: 1 })
  expect(status.draftId).toBeTruthy()
  expect(status.courseId).toBeTruthy()
})
