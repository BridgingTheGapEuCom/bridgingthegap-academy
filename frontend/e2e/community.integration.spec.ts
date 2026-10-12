import { execFileSync } from 'node:child_process'
import { expect, test } from '@playwright/test'

const email = 'learner-e2e@example.com'
const password = 'learner integration password'
const title = 'Real community persistence'
const opening = 'A persisted real-stack opening post.'
const reply = 'A durable real-stack reply.'

function fixture(command: 'seed' | 'status') {
  if (command === 'seed') {
    execFileSync('go', ['run', './tools/learner-e2e-seed'], { cwd: process.cwd(), env: { ...process.env, BTG_LEARNER_E2E_ALLOW_SEED: '1' }, stdio: ['ignore', 'ignore', 'inherit'] })
  }
  const output = execFileSync('go', ['run', './tools/community-e2e-seed', command], {
    cwd: process.cwd(), encoding: 'utf8', env: { ...process.env, BTG_COMMUNITY_E2E_ALLOW_SEED: '1' }, stdio: ['ignore', 'pipe', 'inherit'],
  })
  return JSON.parse(output) as { courseId: string; threads: number; posts: number; matchingTitle: number; matchingReply: number; hiddenPosts: number }
}

test.beforeEach(() => fixture('seed'))

test('real Community lifecycle and moderation persist in PostgreSQL', async ({ page }) => {
  const { courseId } = fixture('status')
  await page.goto(`/login?returnTo=/courses/by-id/${courseId}/community`)
  await page.getByLabel(/^Email/).fill(email)
  await page.getByLabel(/^Password/).fill(password)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page).toHaveURL(`/courses/by-id/${courseId}/community`)

  await page.getByLabel(/^Title/).fill(title)
  await page.getByLabel(/^Opening message/).fill(opening)
  await page.getByRole('button', { name: 'Start discussion' }).click()
  await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible()
  await page.reload()
  await expect(page.getByText(opening)).toBeVisible()

  await page.getByLabel(/^Message/).fill(reply)
  await page.getByRole('button', { name: 'Reply' }).click()
  await expect(page.getByText(reply)).toBeVisible()
  await page.reload()
  await expect(page.getByText(reply)).toBeVisible()
  expect(fixture('status')).toMatchObject({ threads: 1, posts: 2, matchingTitle: 1, matchingReply: 1, hiddenPosts: 0 })

  await page.getByRole('link', { name: 'Back to discussions' }).click()
  await page.getByRole('link', { name: 'Moderation view' }).click()
  await page.getByRole('link', { name: title }).click()
  await page.getByRole('button', { name: 'Hide post' }).click()
  await expect(page.getByText('Post visibility updated.')).toBeVisible()
  await page.reload()
  await expect(page.getByRole('button', { name: 'Unhide post' })).toBeVisible()
  expect(fixture('status')).toMatchObject({ hiddenPosts: 1 })
})
