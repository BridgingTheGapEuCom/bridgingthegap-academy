import { test, expect, type Page, type Route } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

const courseId = '10000000-0000-4000-8000-000000000001'
const threadId = '20000000-0000-4000-8000-000000000001'
const openingId = '40000000-0000-4000-8000-000000000001'
const replyId = '50000000-0000-4000-8000-000000000001'
const authorId = '30000000-0000-4000-8000-000000000001'
const session = { authenticated: true, user_id: authorId, csrf_token: 'csrf', expires_at: '2027-01-01T00:00:00Z' }

async function serveCommunity(page: Page, mode: 'ENABLED' | 'DISABLED' = 'ENABLED') {
  let moderator = true
  let threadHidden = false
  let replyHidden = false
  const timestamp = '2026-01-01T00:00:00Z'
  const summary = () => ({ threadId, title: 'Moderated discussion', author: { userId: authorId }, postCount: replyHidden ? 1 : 2, createdAt: timestamp, updatedAt: timestamp })
  const detailBase = () => ({ threadId, title: 'Moderated discussion', author: { userId: authorId }, createdAt: timestamp, updatedAt: timestamp })
  const participantDetail = () => ({ ...detailBase(), posts: [
    { postId: openingId, author: { userId: authorId }, body: 'Opening post', createdAt: timestamp, updatedAt: timestamp },
    ...(!replyHidden ? [{ postId: replyId, author: { userId: authorId }, body: 'Reply to moderate', createdAt: '2026-01-01T00:01:00Z', updatedAt: '2026-01-01T00:01:00Z' }] : []),
  ], postTotal: replyHidden ? 1 : 2 })
  const moderatorDetail = () => ({ ...detailBase(), state: threadHidden ? 'HIDDEN' : 'VISIBLE', posts: [
    { postId: openingId, author: { userId: authorId }, body: 'Opening post', state: 'VISIBLE', isOpeningPost: true, createdAt: timestamp, updatedAt: timestamp },
    { postId: replyId, author: { userId: authorId }, body: 'Reply to moderate', state: replyHidden ? 'HIDDEN' : 'VISIBLE', isOpeningPost: false, createdAt: '2026-01-01T00:01:00Z', updatedAt: '2026-01-01T00:01:00Z' },
  ], postTotal: 2 })
  const json = (route: Route, body: unknown) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })

  await page.route('**/api/auth/session', (route) => json(route, session))
  await page.route(`**/api/courses/by-id/${courseId}/community**`, (route) => {
    const path = new URL(route.request().url()).pathname
    if (path.endsWith('/community')) return json(route, { courseId, mode })
    if (path.endsWith('/community/moderation')) return json(route, { canModerate: moderator })
    if (path.endsWith('/community/moderation/threads')) return json(route, { threads: [{ ...summary(), state: threadHidden ? 'HIDDEN' : 'VISIBLE' }], total: 1, limit: 20, offset: 0 })
    if (path.endsWith(`/community/moderation/threads/${threadId}`)) return json(route, moderatorDetail())
    if (path.endsWith(`/threads/${threadId}/hide`)) { threadHidden = true; return json(route, {}) }
    if (path.endsWith(`/threads/${threadId}/unhide`)) { threadHidden = false; return json(route, {}) }
    if (path.endsWith(`/posts/${replyId}/hide`)) { replyHidden = true; return json(route, {}) }
    if (path.endsWith(`/posts/${replyId}/unhide`)) { replyHidden = false; return json(route, {}) }
    if (path.endsWith(`/threads/${threadId}`)) {
      if (threadHidden) return route.fulfill({ status: 404, contentType: 'application/problem+json', body: JSON.stringify({ type: 'about:blank', title: 'Not found', status: 404, instance: '', request_id: 'test' }) })
      return json(route, participantDetail())
    }
    return json(route, { threads: threadHidden ? [] : [summary()], total: threadHidden ? 0 : 1, limit: 20, offset: 0 })
  })
  return { asParticipant: () => { moderator = false }, asModerator: () => { moderator = true } }
}

test('moderator can hide and unhide a reply without exposing it to participants', async ({ page }) => {
  const actors = await serveCommunity(page)
  await page.goto(`/courses/by-id/${courseId}/community`)
  await page.getByRole('link', { name: 'Moderation view' }).focus()
  await page.keyboard.press('Enter')
  await page.getByRole('link', { name: 'Moderated discussion' }).click()
  await expect(page.getByText('Reply to moderate')).toBeVisible()
  await page.getByRole('button', { name: 'Hide post' }).focus()
  await page.keyboard.press('Enter')
  await expect(page.getByText('Hidden from participants')).toBeVisible()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  actors.asParticipant()
  await page.goto(`/courses/by-id/${courseId}/community/threads/${threadId}`)
  await expect(page.getByText('Opening post')).toBeVisible()
  await expect(page.getByText('Reply to moderate')).toHaveCount(0)

  actors.asModerator()
  await page.goto(`/courses/by-id/${courseId}/community/moderation/threads/${threadId}`)
  await page.getByRole('button', { name: 'Unhide post' }).click()
  actors.asParticipant()
  await page.goto(`/courses/by-id/${courseId}/community/threads/${threadId}`)
  await expect(page.getByText('Reply to moderate')).toBeVisible()
})

test('hidden Thread survives moderator reload and remains rediscoverable', async ({ page }) => {
  const actors = await serveCommunity(page)
  await page.goto(`/courses/by-id/${courseId}/community/moderation/threads/${threadId}`)
  await page.getByRole('button', { name: 'Hide thread' }).click()
  await expect(page.getByText('Hidden from participants')).toBeVisible()
  await page.reload()
  await expect(page.getByRole('button', { name: 'Unhide thread' })).toBeVisible()
  await page.getByRole('link', { name: 'Back to moderation' }).click()
  await expect(page.getByRole('link', { name: 'Moderated discussion' })).toBeVisible()
  await expect(page.getByText(/Hidden from participants/)).toBeVisible()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.setViewportSize({ width: 320, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await page.evaluate(() => { document.documentElement.style.fontSize = '200%' })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await page.evaluate(() => { document.documentElement.style.fontSize = '' })

  actors.asParticipant()
  await page.goto(`/courses/by-id/${courseId}/community`)
  await expect(page.getByRole('link', { name: 'Moderated discussion' })).toHaveCount(0)
  actors.asModerator()
  await page.goto(`/courses/by-id/${courseId}/community/moderation/threads/${threadId}`)
  await page.getByRole('button', { name: 'Unhide thread' }).click()
  actors.asParticipant()
  await page.goto(`/courses/by-id/${courseId}/community`)
  await expect(page.getByRole('link', { name: 'Moderated discussion' })).toBeVisible()
})

test('participant has no moderation UI and disabled Community remains moderator-readable', async ({ page }) => {
  const actors = await serveCommunity(page, 'DISABLED')
  actors.asParticipant()
  await page.goto(`/courses/by-id/${courseId}/community`)
  await expect(page.getByText('Community discussions are not enabled for this course.')).toBeVisible()
  await expect(page.getByRole('link', { name: 'Moderation view' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: /hide|unhide/i })).toHaveCount(0)

  actors.asModerator()
  await page.reload()
  await expect(page.getByRole('link', { name: 'Moderation view' })).toBeVisible()
  await page.getByRole('link', { name: 'Moderation view' }).click()
  await expect(page.getByRole('link', { name: 'Moderated discussion' })).toBeVisible()
  await page.getByRole('link', { name: 'Moderated discussion' }).click()
  await expect(page.getByRole('button', { name: 'Hide thread' })).toBeVisible()
})
