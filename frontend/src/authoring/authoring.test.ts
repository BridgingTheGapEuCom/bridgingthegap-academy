import { describe, expect, it, vi } from 'vitest'
import { addAuthoringMember, approveAuthoringReview, authoringDraftReviewPath, changeAuthoringMemberRole, createAuthoringModule, getAuthoringActiveDraftReview, getAuthoringDraft, getAuthoringDraftMembers, getAuthoringDraftReview, getAuthoringDraftReviewHistory, getAuthoringLatestDraftReview, getAuthoringLesson, getAuthoringStructure, isAuthoringDraftID, InvalidAuthoringDraftIDError, listAuthoringDrafts, publishAuthoringDraftReview, reorderAuthoringLessons, replaceAuthoringLessonContent, replaceAuthoringLessonPrerequisites, requestAuthoringReviewChanges, revokeAuthoringMember, submitAuthoringDraftReview, updateAuthoringDraft, updateAuthoringLesson } from './authoring'

describe('Authoring API service', () => {
  it('uses the authenticated server-authoritative Draft discovery boundary', async () => {
    const request = vi.fn().mockResolvedValue({ drafts: [] })
    await listAuthoringDrafts({ request })
    expect(request).toHaveBeenCalledWith('/api/authoring/drafts', { cache: 'no-store' })
  })

  it('fails closed when the private Draft discovery payload is malformed', async () => {
    const request = vi.fn().mockResolvedValue({ drafts: [{ id: 'not-a-draft' }] })
    await expect(listAuthoringDrafts({ request })).rejects.toThrow('Invalid Authoring draft list response')
  })

  it('replaces only canonical content through the scoped authenticated PUT boundary', async () => {
    const draftID = '11111111-1111-4111-8111-111111111111'
    const lessonID = '33333333-3333-4333-8333-333333333333'
    const request = vi.fn().mockResolvedValue({})
    const body = { expectedLessonRevision: 9, content: { schemaVersion: 1 as const, blocks: [] } }
    await replaceAuthoringLessonContent(draftID, lessonID, body, { request })
    expect(request).toHaveBeenCalledWith(`/api/authoring/drafts/${draftID}/lessons/${lessonID}/content`, expect.objectContaining({ method: 'PUT', body: JSON.stringify(body) }))
    await expect(replaceAuthoringLessonContent(draftID, 'invalid', body, { request })).rejects.toBeInstanceOf(InvalidAuthoringDraftIDError)
    expect(request).toHaveBeenCalledTimes(1)
  })
  it('uses the authenticated shared request boundary for a bounded Draft ID', async () => {
    const request = vi.fn().mockResolvedValue({ id: '11111111-1111-4111-8111-111111111111' })
    await getAuthoringDraft('11111111-1111-4111-8111-111111111111', { request })
    expect(request).toHaveBeenCalledWith('/api/authoring/drafts/11111111-1111-4111-8111-111111111111')
  })

  it('rejects malformed Draft IDs before constructing an API path', async () => {
    expect(isAuthoringDraftID('not-a-draft')).toBe(false)
    await expect(getAuthoringDraft('not-a-draft', { request: vi.fn() })).rejects.toBeInstanceOf(InvalidAuthoringDraftIDError)
  })

  it('uses PATCH with the shared authenticated request boundary for metadata updates', async () => {
    const request = vi.fn().mockResolvedValue({})
    await updateAuthoringDraft('11111111-1111-4111-8111-111111111111', { expectedRevision: 3, title: 'Updated' }, { request })
    expect(request).toHaveBeenCalledWith('/api/authoring/drafts/11111111-1111-4111-8111-111111111111', expect.objectContaining({ method: 'PATCH' }))
  })

  it('uses the scoped authenticated membership read and mutation boundaries with opaque IDs', async () => {
    const request = vi.fn().mockResolvedValue({})
    const draftID = '11111111-1111-4111-8111-111111111111'
    const userID = '22222222-2222-4222-8222-222222222222'
    await getAuthoringDraftMembers(draftID, { request })
    await addAuthoringMember(draftID, { expectedDraftRevision: 3, userId: userID, role: 'AUTHOR' }, { request })
    await changeAuthoringMemberRole(draftID, userID, { expectedDraftRevision: 4, role: 'MAINTAINER' }, { request })
    await revokeAuthoringMember(draftID, userID, 5, { request })
    expect(request).toHaveBeenNthCalledWith(1, `/api/authoring/drafts/${draftID}/members`)
    expect(request).toHaveBeenNthCalledWith(2, `/api/authoring/drafts/${draftID}/members`, expect.objectContaining({ method: 'POST' }))
    expect(request).toHaveBeenNthCalledWith(3, `/api/authoring/drafts/${draftID}/members/${userID}`, expect.objectContaining({ method: 'PATCH' }))
    expect(request).toHaveBeenNthCalledWith(4, `/api/authoring/drafts/${draftID}/members/${userID}`, expect.objectContaining({ method: 'DELETE', body: JSON.stringify({ expectedDraftRevision: 5 }) }))
    await expect(changeAuthoringMemberRole(draftID, 'not-a-user-id', { expectedDraftRevision: 6, role: 'AUTHOR' }, { request })).rejects.toBeInstanceOf(InvalidAuthoringDraftIDError)
  })

  it('uses the authenticated shared request boundary for Draft structure reads and mutations', async () => {
    const request = vi.fn().mockResolvedValue({})
    const draftID = '11111111-1111-4111-8111-111111111111'
    const moduleID = '22222222-2222-4222-8222-222222222222'
    const lessonID = '33333333-3333-4333-8333-333333333333'
    await getAuthoringStructure(draftID, { request })
    await createAuthoringModule(draftID, { expectedDraftRevision: 3, stableKey: 'foundations', title: 'Foundations', position: 0 }, { request })
    await reorderAuthoringLessons(draftID, 4, [{ moduleId: moduleID, lessonIds: [lessonID] }], { request })
    expect(request).toHaveBeenNthCalledWith(1, `/api/authoring/drafts/${draftID}/structure`)
    expect(request).toHaveBeenNthCalledWith(2, `/api/authoring/drafts/${draftID}/modules`, expect.objectContaining({ method: 'POST' }))
    expect(request).toHaveBeenNthCalledWith(3, `/api/authoring/drafts/${draftID}/lessons/order`, expect.objectContaining({ method: 'PUT' }))
  })

  it('reads Review metadata through exact Draft-scoped endpoints without snapshots', async () => {
    const request = vi.fn().mockResolvedValue({ reviews: [] })
    const draftID = '11111111-1111-4111-8111-111111111111'
    await getAuthoringDraftReviewHistory(draftID, { request })
    await getAuthoringActiveDraftReview(draftID, { request })
    await getAuthoringLatestDraftReview(draftID, { request })
    expect(request).toHaveBeenNthCalledWith(1, `/api/authoring/drafts/${draftID}/reviews`)
    expect(request).toHaveBeenNthCalledWith(2, `/api/authoring/drafts/${draftID}/reviews/active`)
    expect(request).toHaveBeenNthCalledWith(3, `/api/authoring/drafts/${draftID}/reviews/latest`)
  })

  it('submits only the authoritative expected Draft revision through the authenticated Review boundary', async () => {
    const request = vi.fn().mockResolvedValue({ review: {}, snapshot: {} })
    const draftID = '11111111-1111-4111-8111-111111111111'
    await submitAuthoringDraftReview(draftID, { expectedDraftRevision: 7 }, { request })
    expect(request).toHaveBeenCalledWith(
      `/api/authoring/drafts/${draftID}/reviews`,
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ expectedDraftRevision: 7 }) }),
    )
  })

  it('reads an immutable Review snapshot through both exact bounded route IDs', async () => {
    const request = vi.fn().mockResolvedValue({})
    const draftID = '11111111-1111-4111-8111-111111111111'
    const reviewID = '22222222-2222-4222-8222-222222222222'
    await getAuthoringDraftReview(draftID, reviewID, { request })
    expect(request).toHaveBeenCalledWith(`/api/authoring/drafts/${draftID}/reviews/${reviewID}`)
    expect(authoringDraftReviewPath(draftID, reviewID)).toBe(`/authoring/drafts/${draftID}/reviews/${reviewID}`)
    await expect(getAuthoringDraftReview(draftID, 'not-a-review', { request })).rejects.toBeInstanceOf(InvalidAuthoringDraftIDError)
  })

  it('sends only the authoritative Review revision through exact scoped decision endpoints', async () => {
    const request = vi.fn().mockResolvedValue({})
    const draftID = '11111111-1111-4111-8111-111111111111'
    const reviewID = '22222222-2222-4222-8222-222222222222'
    const input = { expectedReviewRevision: 6 }
    await approveAuthoringReview(draftID, reviewID, input, { request })
    await requestAuthoringReviewChanges(draftID, reviewID, input, { request })
    expect(request).toHaveBeenNthCalledWith(1, `/api/authoring/drafts/${draftID}/reviews/${reviewID}/approve`, expect.objectContaining({ method: 'POST', body: JSON.stringify(input) }))
    expect(request).toHaveBeenNthCalledWith(2, `/api/authoring/drafts/${draftID}/reviews/${reviewID}/request-changes`, expect.objectContaining({ method: 'POST', body: JSON.stringify(input) }))
    await expect(approveAuthoringReview(draftID, 'not-a-review', input, { request })).rejects.toBeInstanceOf(InvalidAuthoringDraftIDError)
  })

  it('publishes an exact Review using only its authoritative revision', async () => {
    const request = vi.fn().mockResolvedValue({})
    const draftID = '11111111-1111-4111-8111-111111111111'
    const reviewID = '22222222-2222-4222-8222-222222222222'
    await publishAuthoringDraftReview(draftID, reviewID, { expectedReviewRevision: 8 }, { request })
    expect(request).toHaveBeenCalledWith(
      `/api/authoring/drafts/${draftID}/reviews/${reviewID}/publish`,
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ expectedReviewRevision: 8 }) }),
    )
    const body = JSON.parse(request.mock.calls[0][1].body)
    expect(body).toEqual({ expectedReviewRevision: 8 })
    await expect(publishAuthoringDraftReview(draftID, 'not-a-review', { expectedReviewRevision: 8 }, { request })).rejects.toBeInstanceOf(InvalidAuthoringDraftIDError)
  })

  it('keeps Lesson reads and metadata patches scoped to both bounded IDs', async () => {
    const request = vi.fn().mockResolvedValue({})
    const draftID = '11111111-1111-4111-8111-111111111111'
    const lessonID = '33333333-3333-4333-8333-333333333333'
    await getAuthoringLesson(draftID, lessonID, { request })
    await updateAuthoringLesson(draftID, lessonID, { expectedLessonRevision: 3, title: 'Updated Lesson' }, { request })
    await replaceAuthoringLessonPrerequisites(draftID, lessonID, { expectedLessonRevision: 3, prerequisiteLessonKeys: ['intro-to-eai'] }, { request })
    expect(request).toHaveBeenNthCalledWith(1, `/api/authoring/drafts/${draftID}/lessons/${lessonID}`)
    expect(request).toHaveBeenNthCalledWith(2, `/api/authoring/drafts/${draftID}/lessons/${lessonID}`, expect.objectContaining({ method: 'PATCH' }))
    expect(request).toHaveBeenNthCalledWith(3, `/api/authoring/drafts/${draftID}/lessons/${lessonID}/prerequisites`, expect.objectContaining({ method: 'PUT' }))
    await expect(getAuthoringLesson(draftID, 'not-a-lesson', { request })).rejects.toBeInstanceOf(InvalidAuthoringDraftIDError)
  })
})
