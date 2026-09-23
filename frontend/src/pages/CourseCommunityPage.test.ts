import { cleanup, render, screen, waitFor } from '@testing-library/vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { metadata, threads, probe } = vi.hoisted(() => ({ metadata: vi.fn(), threads: vi.fn(), probe: vi.fn() }))

vi.mock('../courses/community', async (original) => ({
  ...(await original<typeof import('../courses/community')>()),
  getCourseCommunity: metadata,
  listCommunityThreads: threads,
  getCommunityModerationProbe: probe,
}))

import Page from './CourseCommunityPage.vue'

const courseId = '10000000-0000-4000-8000-000000000001'
const secondCourseId = '10000000-0000-4000-8000-000000000002'
const threadId = '20000000-0000-4000-8000-000000000001'
const visiblePage = {
  threads: [{ threadId, title: 'Visible discussion', author: { userId: '30000000-0000-4000-8000-000000000001' }, postCount: 1, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z' }],
  total: 1,
  limit: 20,
  offset: 0,
}

async function mount() {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/courses/by-id/:courseId/community', component: Page },
    { path: '/courses/by-id/:courseId/community/moderation', component: { template: '<p>Moderator destination</p>' } },
  ] })
  await router.push(`/courses/by-id/${courseId}/community`)
  await router.isReady()
  render(Page, { global: { plugins: [router] } })
  await screen.findByRole('heading', { name: 'Course discussions' })
  return router
}

describe('course community moderation entry', () => {
  beforeEach(() => {
    metadata.mockReset()
    threads.mockReset()
    probe.mockReset()
    metadata.mockResolvedValue({ courseId, mode: 'ENABLED' })
    threads.mockResolvedValue(visiblePage)
  })
  afterEach(cleanup)

  it('shows the server-authorized moderation entry while preserving participant discussions', async () => {
    probe.mockResolvedValue({ canModerate: true })
    await mount()
    const entry = await screen.findByRole('link', { name: 'Moderation view' })
    expect(screen.getByRole('link', { name: 'Visible discussion' })).toBeTruthy()
    expect(entry.getAttribute('href')).toBe(`/courses/by-id/${courseId}/community/moderation`)
  })

  it('fails closed for a participant or an unavailable probe without breaking Community', async () => {
    probe.mockResolvedValueOnce({ canModerate: false })
    await mount()
    expect(screen.getByRole('link', { name: 'Visible discussion' })).toBeTruthy()
    expect(screen.queryByRole('link', { name: 'Moderation view' })).toBeNull()
    cleanup()

    probe.mockRejectedValueOnce(new Error('unavailable'))
    await mount()
    expect(screen.getByRole('link', { name: 'Visible discussion' })).toBeTruthy()
    await waitFor(() => expect(screen.queryByRole('link', { name: 'Moderation view' })).toBeNull())
    expect(screen.queryByText('Hidden from participants')).toBeNull()
    expect(screen.queryByRole('button', { name: /hide|unhide/i })).toBeNull()
  })

  it('keeps participant controls unavailable when Community is disabled but still exposes moderation when authorized', async () => {
    metadata.mockResolvedValue({ courseId, mode: 'DISABLED' })
    probe.mockResolvedValue({ canModerate: true })
    await mount()
    expect(screen.getByText('Community discussions are not enabled for this course.')).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Start discussion' })).toBeNull()
    expect(await screen.findByRole('link', { name: 'Moderation view' })).toBeTruthy()
  })

  it('does not let a late Course A moderation probe enable controls after navigating to Course B', async () => {
    let resolveFirstProbe!: (value: { canModerate: boolean }) => void
    probe.mockImplementationOnce(() => new Promise((resolve) => { resolveFirstProbe = resolve })).mockResolvedValueOnce({ canModerate: false })
    metadata.mockResolvedValueOnce({ courseId, mode: 'ENABLED' }).mockResolvedValueOnce({ courseId: secondCourseId, mode: 'ENABLED' })
    const router = await mount()
    await waitFor(() => expect(probe).toHaveBeenCalledWith(courseId))
    await router.push(`/courses/by-id/${secondCourseId}/community`)
    await waitFor(() => expect(probe).toHaveBeenCalledWith(secondCourseId))
    resolveFirstProbe({ canModerate: true })
    await waitFor(() => expect(screen.queryByRole('link', { name: 'Moderation view' })).toBeNull())
  })
})
