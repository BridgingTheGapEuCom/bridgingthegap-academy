import { describe, expect, it, vi } from 'vitest'
import { APIProblemError, type APIClient } from '../api/client'
import { CommunityDisabledError, createCommunityPost, createCommunityThread, getCommunityModerationProbe, getCommunityThreadForModeration, hideCommunityPost, hideCommunityThread, listCommunityThreads, listCommunityThreadsForModeration, unhideCommunityPost, unhideCommunityThread, getCourseCommunity } from './community'

const courseId = '10000000-0000-4000-8000-000000000001'
const threadId = '20000000-0000-4000-8000-000000000001'
const client = (value: unknown): APIClient => ({ request: vi.fn().mockResolvedValue(value) })
const metadata = { courseId, mode: 'ENABLED' }
const page = { threads: [{ threadId, title: 'Question', author: { userId: '30000000-0000-4000-8000-000000000001' }, postCount: 1, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z' }], total: 1, limit: 20, offset: 0 }
const detail = { threadId, title: 'Question', author: { userId: '30000000-0000-4000-8000-000000000001' }, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z', posts: [{ postId: '40000000-0000-4000-8000-000000000001', author: { userId: '30000000-0000-4000-8000-000000000001' }, body: 'Plain\ntext', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z' }], postTotal: 1 }

describe('community API helpers', () => {
  it('uses Course-scoped no-store reads and validates metadata/pages', async () => {
    const api = client(metadata); await expect(getCourseCommunity(courseId, api)).resolves.toEqual(metadata)
    expect(api.request).toHaveBeenCalledWith(`/api/courses/by-id/${courseId}/community`, { cache: 'no-store' })
    const list = client(page); await expect(listCommunityThreads(courseId, 0, list)).resolves.toEqual(page)
    expect(list.request).toHaveBeenCalledWith(`/api/courses/by-id/${courseId}/community/threads?limit=20&offset=0`, { cache: 'no-store' })
  })
  it('sends only title/body or body for mutations', async () => {
    const thread = client(detail); await createCommunityThread(courseId, 'Title', 'Body', thread)
    expect(thread.request).toHaveBeenCalledWith(`/api/courses/by-id/${courseId}/community/threads`, expect.objectContaining({ body: JSON.stringify({ title: 'Title', body: 'Body' }) }))
    const post = client(detail.posts[0]); await createCommunityPost(courseId, threadId, 'Reply', post)
    expect(post.request).toHaveBeenCalledWith(`/api/courses/by-id/${courseId}/community/threads/${threadId}/posts`, expect.objectContaining({ body: JSON.stringify({ body: 'Reply' }) }))
  })
  it('rejects malformed mode and maps community_disabled distinctly', async () => {
    await expect(getCourseCommunity(courseId, client({ ...metadata, mode: 'HIDDEN' }))).rejects.toThrow('Invalid published course response')
    const disabled: APIClient = { request: vi.fn().mockRejectedValue(new APIProblemError(409, { type: '', title: '', status: 409, instance: '', request_id: '', code: 'community_disabled' }, undefined)) }
    await expect(createCommunityThread(courseId, 'Title', 'Body', disabled)).rejects.toBeInstanceOf(CommunityDisabledError)
  })
  it('validates the server-authoritative moderator surface and sends bodyless actions', async () => {
    await expect(getCommunityModerationProbe(courseId, client({ canModerate: true }))).resolves.toEqual({ canModerate: true })
    await expect(getCommunityModerationProbe(courseId, client({ canModerate: 'AUTHOR' }))).rejects.toThrow('Invalid published course response')
    const moderator = { threadId, title: 'Question', author: { userId: '30000000-0000-4000-8000-000000000001' }, state: 'HIDDEN', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z', posts: [{ postId: '40000000-0000-4000-8000-000000000001', author: { userId: '30000000-0000-4000-8000-000000000001' }, body: 'Reply', state: 'HIDDEN', isOpeningPost: false, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z' }], postTotal: 1 }
    await expect(getCommunityThreadForModeration(courseId, threadId, client(moderator))).resolves.toEqual(moderator)
    await expect(listCommunityThreadsForModeration(courseId, 0, client({ threads: [{ ...moderator, postCount: 1, posts: undefined, postTotal: undefined }], total: 1, limit: 20, offset: 0 }))).resolves.toBeTruthy()
    const api = client({}); await hideCommunityThread(courseId, threadId, api); await unhideCommunityThread(courseId, threadId, api); await hideCommunityPost(courseId, threadId, moderator.posts[0].postId, api); await unhideCommunityPost(courseId, threadId, moderator.posts[0].postId, api)
    for (const [, options] of (api.request as ReturnType<typeof vi.fn>).mock.calls) expect(options).toMatchObject({ method: 'POST', cache: 'no-store' })
    expect(JSON.stringify((api.request as ReturnType<typeof vi.fn>).mock.calls)).not.toContain('state')
  })
})
