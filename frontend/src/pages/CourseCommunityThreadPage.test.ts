import { cleanup, fireEvent, screen, waitFor } from '@testing-library/vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithI18n } from '../test/i18n'

const { read, create } = vi.hoisted(() => ({ read: vi.fn(), create: vi.fn() }))
vi.mock('../courses/community', async (original) => ({ ...(await original<typeof import('../courses/community')>()), getCommunityThread: read, createCommunityPost: create }))
import Page from './CourseCommunityThreadPage.vue'

const courseId = '10000000-0000-4000-8000-000000000001'
const threadId = '20000000-0000-4000-8000-000000000001'
const detail = { threadId, title: 'A user-authored title', author: { userId: '30000000-0000-4000-8000-000000000001' }, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z', posts: [{ postId: '40000000-0000-4000-8000-000000000001', author: { userId: '30000000-0000-4000-8000-000000000001' }, body: 'User-authored opening', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z' }], postTotal: 21 }

async function mount(locale: 'en' | 'en-XA' = 'en') {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/courses/by-id/:courseId/community/threads/:threadId', component: Page }] })
  await router.push(`/courses/by-id/${courseId}/community/threads/${threadId}`)
  await router.isReady()
  return renderWithI18n(Page, { global: { plugins: [router] } }, locale)
}

describe('participant Community thread', () => {
  beforeEach(() => { read.mockReset(); create.mockReset(); read.mockResolvedValue(detail) })
  afterEach(cleanup)
  it('keeps authored content unchanged, replies through authenticated transport, and paginates', async () => {
    create.mockResolvedValue({ postId: '50000000-0000-4000-8000-000000000001', author: detail.author, body: 'Durable reply', createdAt: '2026-01-01T00:01:00Z', updatedAt: '2026-01-01T00:01:00Z' })
    await mount()
    expect(await screen.findByRole('heading', { name: detail.title })).toBeTruthy()
    await fireEvent.update(screen.getByLabelText(/^Message/), 'Durable reply')
    await fireEvent.click(screen.getByRole('button', { name: 'Reply' }))
    await waitFor(() => expect(create).toHaveBeenCalledWith(courseId, threadId, 'Durable reply', expect.anything()))
    expect(screen.getByText('Reply posted.')).toBeTruthy()
    await fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await waitFor(() => expect(read).toHaveBeenLastCalledWith(courseId, threadId, 20, expect.anything()))
  })
  it('pseudo-localizes chrome but not authored content', async () => {
    await mount('en-XA')
    expect(await screen.findByRole('heading', { name: detail.title })).toBeTruthy()
    expect(screen.getByText('User-authored opening')).toBeTruthy()
    expect(screen.queryByText('Reply', { selector: 'h2' })).toBeNull()
  })
})
