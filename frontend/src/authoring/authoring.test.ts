import { describe, expect, it, vi } from 'vitest'
import { createAuthoringModule, getAuthoringDraft, getAuthoringLesson, getAuthoringStructure, isAuthoringDraftID, InvalidAuthoringDraftIDError, reorderAuthoringLessons, replaceAuthoringLessonPrerequisites, updateAuthoringDraft, updateAuthoringLesson } from './authoring'

describe('Authoring API service', () => {
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
