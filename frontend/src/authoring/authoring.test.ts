import { describe, expect, it, vi } from 'vitest'
import { addAuthoringMember, approveAuthoringReview, authoringDraftReviewPath, changeAuthoringMemberRole, createAuthoringAssessment, createAuthoringDraft, createAuthoringModule, getAuthoringActiveDraftReview, getAuthoringAssessment, getAuthoringDraft, getAuthoringDraftMembers, getAuthoringDraftReview, getAuthoringDraftReviewHistory, getAuthoringLatestDraftReview, getAuthoringLesson, getAuthoringStructure, isAuthoringDraftID, InvalidAuthoringAssessmentResponseError, InvalidAuthoringAssetResponseError, InvalidAuthoringDraftIDError, InvalidAuthoringDraftResponseError, InvalidAuthoringPublicationResponseError, InvalidAuthoringReviewResponseError, listAuthoringDraftAssessments, listAuthoringDraftAssets, listAuthoringDrafts, publishAuthoringDraftReview, reorderAuthoringLessons, replaceAuthoringAssessment, replaceAuthoringLessonContent, replaceAuthoringLessonPrerequisites, requestAuthoringReviewChanges, revokeAuthoringMember, submitAuthoringDraftReview, updateAuthoringDraft, updateAuthoringLesson, uploadAuthoringAsset } from './authoring'

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

  it('creates a Draft with metadata only and rejects a malformed authoritative response', async () => {
    const input = {
      title: 'First Draft', intendedVersion: '0.1.0', sourceLanguage: 'en', description: 'A complete description.', objectives: ['Explain the topic'], changelog: 'Initial Draft.',
    }
    const created = {
      id: '11111111-1111-4111-8111-111111111111', course_id: '22222222-2222-4222-8222-222222222222', intended_version: '0.1.0', source_language: 'en', title: input.title, description: input.description, objectives: input.objectives, changelog: input.changelog,
      license: { kind: 'ALL_RIGHTS_RESERVED', identifier: '', display_name: 'All Rights Reserved', url: '', custom_text: '' }, status: 'ACTIVE', revision: 1, created_at: '2026-09-17T10:00:00Z', updated_at: '2026-09-17T10:00:00Z',
    }
    const request = vi.fn().mockResolvedValue(created)
    await expect(createAuthoringDraft(input, { request })).resolves.toEqual(created)
    expect(request).toHaveBeenCalledWith('/api/authoring/drafts', expect.objectContaining({ method: 'POST', body: JSON.stringify(input), cache: 'no-store' }))
    expect(JSON.parse(request.mock.calls[0][1].body)).not.toHaveProperty('userId')
    expect(JSON.parse(request.mock.calls[0][1].body)).not.toHaveProperty('publishedBy')

    await expect(createAuthoringDraft(input, { request: vi.fn().mockResolvedValue({ id: 'not-a-draft' }) })).rejects.toBeInstanceOf(InvalidAuthoringDraftResponseError)
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

  it('uploads exactly one file to the scoped Asset boundary and validates the authoritative response', async () => {
    const draftID = '11111111-1111-4111-8111-111111111111'
    const asset = { assetKey: '22222222-2222-4222-8222-222222222222', filename: 'diagram.png', mediaType: 'image/png', byteSize: 123, status: 'AVAILABLE' }
    const request = vi.fn().mockResolvedValue(asset)
    const file = new File(['untrusted browser bytes'], 'diagram.png', { type: 'image/png' })
    await expect(uploadAuthoringAsset(draftID, file, { request })).resolves.toEqual(asset)
    expect(request).toHaveBeenCalledWith(`/api/authoring/drafts/${draftID}/assets`, expect.objectContaining({ method: 'POST', cache: 'no-store' }))
    const options = request.mock.calls[0]![1] as { body: FormData; headers?: HeadersInit }
    expect(options.body).toBeInstanceOf(FormData)
    expect(options.body.getAll('file')).toEqual([file])
    const formEntries: string[] = []
    options.body.forEach((_value, key) => formEntries.push(key))
    expect(formEntries).toEqual(['file'])
    expect(options.headers).toBeUndefined()

    await expect(uploadAuthoringAsset(draftID, file, { request: vi.fn().mockResolvedValue({ ...asset, status: 'PENDING' }) })).rejects.toBeInstanceOf(InvalidAuthoringAssetResponseError)
    await expect(uploadAuthoringAsset(draftID, file, { request: vi.fn().mockResolvedValue({ ...asset, assetKey: 'not-an-asset' }) })).rejects.toBeInstanceOf(InvalidAuthoringAssetResponseError)
    await expect(uploadAuthoringAsset(draftID, file, { request: vi.fn().mockResolvedValue({ ...asset, filename: '../path' }) })).rejects.toBeInstanceOf(InvalidAuthoringAssetResponseError)
  })

  it('does not retry an Asset upload mutation', async () => {
    const request = vi.fn().mockRejectedValue(new Error('network unavailable'))
    await expect(uploadAuthoringAsset('11111111-1111-4111-8111-111111111111', new File(['x'], 'notes.txt'), { request })).rejects.toThrow('network unavailable')
    expect(request).toHaveBeenCalledTimes(1)
  })
  it('lists only runtime-validated Asset summaries through the exact Draft boundary', async () => {
    const page = { items: [{ assetKey: '22222222-2222-4222-8222-222222222222', filename: 'diagram.png', mediaType: 'image/png', byteSize: 123, createdAt: '2026-09-17T10:00:00Z' }], limit: 20, offset: 0, total: 1 }
    const request = vi.fn().mockResolvedValue(page)
    await expect(listAuthoringDraftAssets('11111111-1111-4111-8111-111111111111', 20, 0, { request })).resolves.toEqual(page)
    expect(request).toHaveBeenCalledWith('/api/authoring/drafts/11111111-1111-4111-8111-111111111111/assets?limit=20&offset=0', { cache: 'no-store' })
    await expect(listAuthoringDraftAssets('11111111-1111-4111-8111-111111111111', 20, 0, { request: vi.fn().mockResolvedValue({ ...page, items: [{ ...page.items[0], createdAt: 'invalid' }] }) })).rejects.toBeInstanceOf(InvalidAuthoringAssetResponseError)
  })
  it('uses Draft-scoped Authoring-only Assessment DTOs and fails closed on malformed answer definitions', async () => {
    const draftID = '11111111-1111-4111-8111-111111111111'
    const assessmentID = '22222222-2222-4222-8222-222222222222'
    const question = { stableKey: 'question-one', type: 'SINGLE_CHOICE' as const, prompt: 'Choose one', position: 0, options: [{ stableKey: 'option-one', text: 'One', position: 0 }, { stableKey: 'option-two', text: 'Two', position: 1 }], correctOptionKeys: ['option-one'], leftItems: [], rightItems: [], correctPairs: [] }
    const detail = { assessmentKey: assessmentID, title: 'Knowledge check', revision: 2, questions: [question], createdAt: '2026-09-17T10:00:00Z', updatedAt: '2026-09-17T10:00:00Z' }
    const page = { items: [{ assessmentKey: assessmentID, title: 'Knowledge check', questionCount: 1, revision: 2, updatedAt: '2026-09-17T10:00:00Z' }], limit: 20, offset: 0, total: 1 }
    const request = vi.fn().mockResolvedValueOnce(page).mockResolvedValueOnce(detail).mockResolvedValueOnce({ ...detail, revision: 3 }).mockResolvedValueOnce(detail)
    await expect(listAuthoringDraftAssessments(draftID, 20, 0, { request })).resolves.toEqual(page)
    await expect(createAuthoringAssessment(draftID, { title: detail.title, questions: [] }, { request })).resolves.toEqual(detail)
    await expect(replaceAuthoringAssessment(draftID, assessmentID, { expectedRevision: 2, title: detail.title, questions: [question] }, { request })).resolves.toMatchObject({ revision: 3 })
    await expect(getAuthoringAssessment(draftID, assessmentID, { request })).resolves.toEqual(detail)
    expect(request).toHaveBeenNthCalledWith(1, `/api/authoring/drafts/${draftID}/assessments?limit=20&offset=0`, { cache: 'no-store' })
    expect(request).toHaveBeenNthCalledWith(2, `/api/authoring/drafts/${draftID}/assessments`, expect.objectContaining({ method: 'POST', cache: 'no-store', body: JSON.stringify({ title: detail.title, questions: [] }) }))
    expect(request).toHaveBeenNthCalledWith(3, `/api/authoring/drafts/${draftID}/assessments/${assessmentID}`, expect.objectContaining({ method: 'PUT', cache: 'no-store' }))
    expect(JSON.parse(request.mock.calls[2]![1].body)).toEqual({ expectedRevision: 2, title: detail.title, questions: [question] })
    await expect(getAuthoringAssessment(draftID, assessmentID, { request: vi.fn().mockResolvedValue({ ...detail, questions: [{ ...question, correctOptionKeys: ['missing'] }] }) })).rejects.toBeInstanceOf(InvalidAuthoringAssessmentResponseError)
    await expect(getAuthoringAssessment(draftID, assessmentID, { request: vi.fn().mockResolvedValue({ ...detail, questions: [{ ...question, position: 1 }] }) })).rejects.toBeInstanceOf(InvalidAuthoringAssessmentResponseError)
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
    const draftID = '11111111-1111-4111-8111-111111111111'
    const reviewID = '22222222-2222-4222-8222-222222222222'
    const request = vi.fn().mockResolvedValue({
      review: { id: reviewID, draftId: draftID, status: 'APPROVED', reviewRevision: 2 },
      snapshot: { schemaVersion: 1, modules: [] },
      publication: { canPublish: true, publishable: true, issues: [], published: null },
    })
    await getAuthoringDraftReview(draftID, reviewID, { request })
    expect(request).toHaveBeenCalledWith(`/api/authoring/drafts/${draftID}/reviews/${reviewID}`, { cache: 'no-store' })
    expect(authoringDraftReviewPath(draftID, reviewID)).toBe(`/authoring/drafts/${draftID}/reviews/${reviewID}`)
    await expect(getAuthoringDraftReview(draftID, 'not-a-review', { request })).rejects.toBeInstanceOf(InvalidAuthoringDraftIDError)
    await expect(getAuthoringDraftReview(draftID, reviewID, { request: vi.fn().mockResolvedValue({ review: {}, snapshot: {} }) })).rejects.toBeInstanceOf(InvalidAuthoringReviewResponseError)
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
    const request = vi.fn().mockResolvedValue({
      reviewId: '22222222-2222-4222-8222-222222222222', reviewRevision: 8,
      courseId: '33333333-3333-4333-8333-333333333333', courseVersion: '1.0.0',
      courseVersionId: '44444444-4444-4444-8444-444444444444', publishedAt: '2026-09-16T14:30:00Z',
    })
    const draftID = '11111111-1111-4111-8111-111111111111'
    const reviewID = '22222222-2222-4222-8222-222222222222'
    await publishAuthoringDraftReview(draftID, reviewID, { expectedReviewRevision: 8 }, { request })
    expect(request).toHaveBeenCalledWith(
      `/api/authoring/drafts/${draftID}/reviews/${reviewID}/publish`,
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ expectedReviewRevision: 8 }), cache: 'no-store' }),
    )
    const body = JSON.parse(request.mock.calls[0][1].body)
    expect(body).toEqual({ expectedReviewRevision: 8 })
    expect(body).not.toHaveProperty('userId')
    expect(body).not.toHaveProperty('publishedBy')
    await expect(publishAuthoringDraftReview(draftID, 'not-a-review', { expectedReviewRevision: 8 }, { request })).rejects.toBeInstanceOf(InvalidAuthoringDraftIDError)
  })

  it('fails closed when a publication response is malformed or belongs to another Review', async () => {
    const draftID = '11111111-1111-4111-8111-111111111111'
    const reviewID = '22222222-2222-4222-8222-222222222222'
    const input = { expectedReviewRevision: 8 }
    const valid = {
      reviewId: reviewID, reviewRevision: 8,
      courseId: '33333333-3333-4333-8333-333333333333', courseVersion: '1.0.0',
      courseVersionId: '44444444-4444-4444-8444-444444444444', publishedAt: '2026-09-16T14:30:00Z',
    }
    await expect(publishAuthoringDraftReview(draftID, reviewID, input, { request: vi.fn().mockResolvedValue({ ...valid, reviewId: draftID }) })).rejects.toBeInstanceOf(InvalidAuthoringPublicationResponseError)
    await expect(publishAuthoringDraftReview(draftID, reviewID, input, { request: vi.fn().mockResolvedValue({ ...valid, reviewRevision: 9 }) })).rejects.toBeInstanceOf(InvalidAuthoringPublicationResponseError)
    await expect(publishAuthoringDraftReview(draftID, reviewID, input, { request: vi.fn().mockResolvedValue({ ...valid, courseVersion: 'not-a-version' }) })).rejects.toBeInstanceOf(InvalidAuthoringPublicationResponseError)
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
