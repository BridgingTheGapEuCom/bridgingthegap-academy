import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { list } = vi.hoisted(() => ({ list: vi.fn() }))
vi.mock('../courses/community', async (original) => ({ ...(await original<typeof import('../courses/community')>()), listCommunityThreadsForModeration: list }))
import Page from './CourseCommunityModerationPage.vue'

const courseA = '10000000-0000-4000-8000-000000000001'
const courseB = '10000000-0000-4000-8000-000000000002'
const summary = (threadId: string, title: string, state: 'VISIBLE' | 'HIDDEN') => ({ threadId, title, author: { userId: '30000000-0000-4000-8000-000000000001' }, state, postCount: 1, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z' })
const first = summary('20000000-0000-4000-8000-000000000001', 'Visible first', 'VISIBLE')
const hidden = summary('20000000-0000-4000-8000-000000000002', 'Hidden second', 'HIDDEN')

async function mount(course = courseA) {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/courses/by-id/:courseId/community/moderation', component: Page }] })
  await router.push(`/courses/by-id/${course}/community/moderation`)
  await router.isReady()
  render(Page, { global: { plugins: [router] } })
  return router
}

describe('community moderation list', () => {
  beforeEach(() => list.mockReset())
  afterEach(cleanup)

  it('keeps server ordering, labels hidden Threads, and paginates', async () => {
    list.mockResolvedValueOnce({ threads: [first, hidden], total: 21, limit: 20, offset: 0 }).mockResolvedValueOnce({ threads: [hidden], total: 21, limit: 20, offset: 20 })
    await mount()
    await screen.findByRole('link', { name: 'Visible first' })
    expect(screen.getAllByRole('link').map((link) => link.textContent)).toContain('Visible first')
    expect(screen.getByText(/Hidden from participants/)).toBeTruthy()
    await fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await waitFor(() => expect(list).toHaveBeenLastCalledWith(courseA, 20))
    expect(await screen.findByRole('link', { name: 'Hidden second' })).toBeTruthy()
  })

  it('handles an empty moderator list and discards a late Course A response after navigation', async () => {
    let resolveA!: (value: unknown) => void
    list.mockImplementationOnce(() => new Promise((resolve) => { resolveA = resolve })).mockResolvedValueOnce({ threads: [], total: 0, limit: 20, offset: 0 })
    const router = await mount()
    await router.push(`/courses/by-id/${courseB}/community/moderation`)
    await waitFor(() => expect(list).toHaveBeenLastCalledWith(courseB, 0))
    resolveA({ threads: [first], total: 1, limit: 20, offset: 0 })
    expect(await screen.findByText('No discussions to moderate.')).toBeTruthy()
    expect(screen.queryByRole('link', { name: 'Visible first' })).toBeNull()
  })
})
