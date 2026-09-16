import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { nextTick, ref } from 'vue'
import { APIProblemError } from '../api/client'
import { authoringDraftContextKey } from '../authoring/draftContext'

const getAuthoringLessonMock = vi.hoisted(() => vi.fn())
const updateAuthoringLessonMock = vi.hoisted(() => vi.fn())
const getAuthoringStructureMock = vi.hoisted(() => vi.fn())
const replaceAuthoringLessonPrerequisitesMock = vi.hoisted(() => vi.fn())
const replaceAuthoringLessonContentMock = vi.hoisted(() => vi.fn())

vi.mock('../authoring/authoring', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../authoring/authoring')>()),
  getAuthoringLesson: getAuthoringLessonMock,
  updateAuthoringLesson: updateAuthoringLessonMock,
  getAuthoringStructure: getAuthoringStructureMock,
  replaceAuthoringLessonPrerequisites: replaceAuthoringLessonPrerequisitesMock,
  replaceAuthoringLessonContent: replaceAuthoringLessonContentMock,
}))

import AuthoringDraftLessonPage from './AuthoringDraftLessonPage.vue'

const draftID = '11111111-1111-4111-8111-111111111111'
const moduleID = '22222222-2222-4222-8222-222222222222'
const lessonID = '33333333-3333-4333-8333-333333333333'
const secondLessonID = '55555555-5555-4555-8555-555555555555'
const prerequisiteLessonID = '66666666-6666-4666-8666-666666666666'
const draft = () => ({
  id: draftID, course_id: '44444444-4444-4444-8444-444444444444', intended_version: '1.0.0', source_language: 'en', title: 'Integration foundations', description: 'Draft.', objectives: ['Explain ownership'], changelog: 'Initial Draft.', license: { kind: 'STANDARD' as const, identifier: '', display_name: 'All Rights Reserved', url: '', custom_text: '' }, status: 'ACTIVE' as const, revision: 3, created_at: '2026-09-15T10:00:00Z', updated_at: '2026-09-15T11:00:00Z',
})
const lesson = (overrides = {}) => ({
  id: lessonID, draft_id: draftID, module_id: moduleID, stable_key: 'what-is-eai', title: 'What is EAI?', description: 'Start here.', objectives: ['Explain EAI', 'Recognise integration boundaries'], estimated_duration_minutes: 15, position: 0, revision: 3, recommended_prerequisite_keys: [], content: { schemaVersion: 1, blocks: [] }, created_at: '2026-09-15T10:00:00Z', updated_at: '2026-09-15T11:00:00Z', ...overrides,
})
const structure = () => ({ modules: [{ id: moduleID, stable_key: 'foundations', title: 'Foundations', description: '', position: 0, revision: 2, lessons: [
  { id: lessonID, stable_key: 'what-is-eai', title: 'What is EAI?', description: 'Start here.', objectives: ['Explain EAI'], estimated_duration_minutes: 15, position: 0, revision: 3, recommended_prerequisite_keys: [] },
  { id: prerequisiteLessonID, stable_key: 'intro-to-eai', title: 'Introduction to EAI', description: 'Begin here.', objectives: ['Recognise EAI'], estimated_duration_minutes: 10, position: 1, revision: 2, recommended_prerequisite_keys: [] },
  { id: secondLessonID, stable_key: 'routing', title: 'Message routing', description: 'Route safely.', objectives: ['Route messages'], estimated_duration_minutes: 20, position: 2, revision: 2, recommended_prerequisite_keys: [] },
] }] })

function mutation(overrides = {}) {
  const { content: _content, recommended_prerequisite_keys: _keys, created_at: _created, updated_at: _updated, ...metadata } = lesson()
  return { ...metadata, draftRevision: 4, ...overrides }
}

async function renderPage() {
  const currentDraft = ref(draft())
  const replaceDraft = vi.fn((nextDraft) => { currentDraft.value = nextDraft })
  const markDraftUnavailable = vi.fn()
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/authoring/drafts/:draftId/lessons/:lessonId', component: AuthoringDraftLessonPage }, { path: '/authoring/drafts/:draftId/structure', component: { template: '<p>Structure</p>' } }] })
  await router.push(`/authoring/drafts/${draftID}/lessons/${lessonID}`)
  await router.isReady()
  return {
    ...render(AuthoringDraftLessonPage, { global: { plugins: [router], provide: { [authoringDraftContextKey as symbol]: { draft: currentDraft, replaceDraft, markDraftUnavailable } } } }),
    currentDraft, replaceDraft, markDraftUnavailable, router,
  }
}

describe('AuthoringDraftLessonPage', () => {
  beforeEach(() => {
    getAuthoringLessonMock.mockReset()
    updateAuthoringLessonMock.mockReset()
    getAuthoringStructureMock.mockReset()
    replaceAuthoringLessonPrerequisitesMock.mockReset()
    replaceAuthoringLessonContentMock.mockReset()
    getAuthoringLessonMock.mockResolvedValue(lesson())
    getAuthoringStructureMock.mockResolvedValue(structure())
  })
  afterEach(cleanup)

  it('shares authoritative revisions across metadata, prerequisites, and content without submitting other unsaved sections', async () => {
    updateAuthoringLessonMock.mockResolvedValueOnce(mutation({ title: 'Saved metadata', revision: 4, draftRevision: 10 }))
      .mockResolvedValueOnce(mutation({ title: 'Second metadata save', revision: 7, draftRevision: 13 }))
    replaceAuthoringLessonPrerequisitesMock.mockResolvedValue(mutation({ title: 'Saved metadata', revision: 5, draftRevision: 11 }))
    replaceAuthoringLessonContentMock.mockImplementation(async (_draft, _lesson, request) => ({
      lesson: mutation({ title: 'Saved metadata', revision: 6, draftRevision: 12 }), content: request.content,
    }))
    const { currentDraft } = await renderPage()
    const title = await screen.findByRole('textbox', { name: /^Title\b/ })
    await fireEvent.click(screen.getByRole('button', { name: 'Add block' }))
    await fireEvent.update(title, 'Saved metadata')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await screen.findByText('Lesson metadata saved.')
    await fireEvent.update(await screen.findByRole('combobox', { name: 'Available Lessons' }), 'intro-to-eai')
    await fireEvent.click(screen.getByRole('button', { name: 'Add recommended prerequisite' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Save recommended prerequisites' }))
    await screen.findByText('Recommended prerequisites saved.')
    expect(replaceAuthoringLessonPrerequisitesMock.mock.calls[0]![2]).toEqual({ expectedLessonRevision: 4, prerequisiteLessonKeys: ['intro-to-eai'] })
    await fireEvent.update(title, 'Second metadata save')
    await fireEvent.click(screen.getByRole('button', { name: 'Save Lesson content' }))
    await screen.findByText('Lesson content saved.')
    expect(replaceAuthoringLessonContentMock.mock.calls[0]![2]).toMatchObject({ expectedLessonRevision: 5, content: { schemaVersion: 1, blocks: [{ type: 'TEXT' }] } })
    expect((title as HTMLInputElement).value).toBe('Second metadata save')
    expect(currentDraft.value.revision).toBe(12)
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(updateAuthoringLessonMock.mock.calls[1]![2]).toEqual({ expectedLessonRevision: 6, title: 'Second metadata save' }))
    expect(currentDraft.value.revision).toBe(13)
    expect(screen.getByRole('list', { name: 'Ordered recommended prerequisites' }).textContent).toContain('Introduction to EAI')
  })

  it('uses a content save revision for the next prerequisite save and keeps unsaved prerequisites independent', async () => {
    replaceAuthoringLessonContentMock.mockImplementation(async (_draft, _lesson, request) => ({ lesson: mutation({ revision: 4, draftRevision: 8 }), content: request.content }))
    replaceAuthoringLessonPrerequisitesMock.mockResolvedValue(mutation({ revision: 5, draftRevision: 9 }))
    await renderPage()
    await fireEvent.update(await screen.findByRole('combobox', { name: 'Available Lessons' }), 'intro-to-eai')
    await fireEvent.click(screen.getByRole('button', { name: 'Add recommended prerequisite' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Add block' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Save Lesson content' }))
    await screen.findByText('Lesson content saved.')
    expect(replaceAuthoringLessonPrerequisitesMock).not.toHaveBeenCalled()
    await fireEvent.click(screen.getByRole('button', { name: 'Save recommended prerequisites' }))
    await waitFor(() => expect(replaceAuthoringLessonPrerequisitesMock.mock.calls[0]![2].expectedLessonRevision).toBe(4))
  })

  it('loads editable Lesson metadata without rendering content or structure controls', async () => {
    await renderPage()
    expect(await screen.findByRole('heading', { level: 2, name: 'Lesson metadata' })).toBeTruthy()
    expect((screen.getByRole('textbox', { name: /Title/ }) as HTMLInputElement).value).toBe('What is EAI?')
    expect((screen.getByRole('textbox', { name: /Description/ }) as HTMLTextAreaElement).value).toBe('Start here.')
    expect(screen.getByText('what-is-eai').tagName).toBe('CODE')
    expect(screen.queryByText('LessonContent')).toBeNull()
    expect(screen.getByRole('heading', { level: 3, name: 'Recommended prerequisites' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: /Move|Delete lesson|Edit content/ })).toBeNull()
    expect(getAuthoringLessonMock).toHaveBeenCalledWith(draftID, lessonID)
  })

  it('shows ordered advisory prerequisites and excludes the current and selected Lessons from candidates', async () => {
    getAuthoringLessonMock.mockResolvedValue(lesson({ recommended_prerequisite_keys: ['routing'] }))
    await renderPage()
    expect(await screen.findByRole('list', { name: 'Ordered recommended prerequisites' })).toBeTruthy()
    expect(screen.getByRole('list', { name: 'Ordered recommended prerequisites' }).textContent).toContain('Message routing')
    const select = await screen.findByRole('combobox', { name: 'Available Lessons' }) as HTMLSelectElement
    expect(Array.from(select.options).map((option) => option.value)).toEqual(['', 'intro-to-eai'])
    expect(screen.getByText(/do not restrict access/)).toBeTruthy()
  })

  it('adds, removes, reorders, and saves the complete ordered prerequisite list', async () => {
    getAuthoringLessonMock.mockResolvedValue(lesson({ recommended_prerequisite_keys: ['routing'] }))
    replaceAuthoringLessonPrerequisitesMock.mockResolvedValue({ ...lesson(), revision: 4, draftRevision: 4 })
    const { currentDraft } = await renderPage()
    await screen.findByRole('heading', { level: 3, name: 'Recommended prerequisites' })
    await fireEvent.update(await screen.findByRole('combobox', { name: 'Available Lessons' }), 'intro-to-eai')
    await fireEvent.click(screen.getByRole('button', { name: 'Add recommended prerequisite' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Move Introduction to EAI up' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Save recommended prerequisites' }))
    await waitFor(() => expect(replaceAuthoringLessonPrerequisitesMock).toHaveBeenCalledWith(draftID, lessonID, {
      expectedLessonRevision: 3,
      prerequisiteLessonKeys: ['intro-to-eai', 'routing'],
    }))
    expect(currentDraft.value.revision).toBe(4)
    expect(screen.getByText('Recommended prerequisites saved.')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Save recommended prerequisites' }).hasAttribute('disabled')).toBe(true)
  })

  it('keeps prerequisite changes independent from metadata changes and clears dirty state when reverted', async () => {
    await renderPage()
    await screen.findByRole('heading', { level: 3, name: 'Recommended prerequisites' })
    await fireEvent.update(await screen.findByRole('combobox', { name: 'Available Lessons' }), 'intro-to-eai')
    await fireEvent.click(screen.getByRole('button', { name: 'Add recommended prerequisite' }))
    expect(screen.getByRole('button', { name: 'Save recommended prerequisites' }).hasAttribute('disabled')).toBe(false)
    await fireEvent.click(screen.getByRole('button', { name: 'Remove Introduction to EAI' }))
    expect(screen.getByRole('button', { name: 'Save recommended prerequisites' }).hasAttribute('disabled')).toBe(true)
    expect(updateAuthoringLessonMock).not.toHaveBeenCalled()
  })

  it('preserves a local prerequisite order after conflict and reloads only when requested', async () => {
    replaceAuthoringLessonPrerequisitesMock.mockRejectedValueOnce(new APIProblemError(409, undefined, undefined))
    getAuthoringLessonMock.mockResolvedValueOnce(lesson()).mockResolvedValueOnce(lesson({ recommended_prerequisite_keys: ['routing'], revision: 4 }))
    await renderPage()
    await screen.findByRole('heading', { level: 3, name: 'Recommended prerequisites' })
    await fireEvent.update(screen.getByRole('textbox', { name: /Title/ }), 'My metadata title')
    await fireEvent.update(await screen.findByRole('combobox', { name: 'Available Lessons' }), 'intro-to-eai')
    await fireEvent.click(screen.getByRole('button', { name: 'Add recommended prerequisite' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Save recommended prerequisites' }))
    expect(await screen.findByText(/recommended prerequisites are still here/)).toBeTruthy()
    expect(screen.getByText('Introduction to EAI')).toBeTruthy()
    await fireEvent.click(screen.getByRole('button', { name: 'Reload latest Lesson' }))
    await waitFor(() => expect(screen.getByRole('list', { name: 'Ordered recommended prerequisites' }).textContent).not.toContain('Introduction to EAI'))
    expect(screen.getByRole('list', { name: 'Ordered recommended prerequisites' }).textContent).toContain('Message routing')
    expect((screen.getByRole('textbox', { name: /Title/ }) as HTMLInputElement).value).toBe('My metadata title')
  })

  it('keeps prerequisite validation failures recoverable and hides unavailable Draft structure', async () => {
    replaceAuthoringLessonPrerequisitesMock.mockRejectedValueOnce(new APIProblemError(400, undefined, undefined))
    await renderPage()
    await fireEvent.update(await screen.findByRole('combobox', { name: 'Available Lessons' }), 'intro-to-eai')
    await fireEvent.click(screen.getByRole('button', { name: 'Add recommended prerequisite' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Save recommended prerequisites' }))
    expect(await screen.findByText(/couldn’t save these recommendations/)).toBeTruthy()

    cleanup()
    getAuthoringStructureMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    const { markDraftUnavailable } = await renderPage()
    await waitFor(() => expect(markDraftUnavailable).toHaveBeenCalledTimes(1))
  })

  it('sends only changed metadata using the authoritative Lesson revision and updates the Draft revision', async () => {
    updateAuthoringLessonMock.mockResolvedValue({ ...lesson(), revision: 4, draftRevision: 4 })
    const { currentDraft } = await renderPage()
    await screen.findByRole('heading', { level: 2, name: 'Lesson metadata' })
    await fireEvent.update(screen.getByRole('textbox', { name: /Title/ }), 'Updated Lesson')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(updateAuthoringLessonMock).toHaveBeenCalledWith(draftID, lessonID, { expectedLessonRevision: 3, title: 'Updated Lesson' }))
    expect(currentDraft.value.revision).toBe(4)
    expect(screen.getByRole('button', { name: 'Save changes' }).hasAttribute('disabled')).toBe(true)
  })

  it('preserves ordered objectives and supports adding and removing objective fields', async () => {
    updateAuthoringLessonMock.mockResolvedValue({ ...lesson({ objectives: ['Explain EAI', 'Apply it safely'] }), revision: 4, draftRevision: 4 })
    await renderPage()
    await screen.findByRole('heading', { level: 2, name: 'Lesson metadata' })
    await fireEvent.update(screen.getByRole('textbox', { name: 'Learning objective 2' }), 'Apply it safely')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(updateAuthoringLessonMock).toHaveBeenCalledWith(draftID, lessonID, { expectedLessonRevision: 3, objectives: ['Explain EAI', 'Apply it safely'] }))
    await fireEvent.click(screen.getByRole('button', { name: 'Add learning objective' }))
    expect(screen.getByRole('textbox', { name: 'Learning objective 3' })).toBeTruthy()
    await fireEvent.click(screen.getByRole('button', { name: 'Remove learning objective 3' }))
    expect(screen.queryByRole('textbox', { name: 'Learning objective 3' })).toBeNull()
  })

  it('allows an optional duration to be cleared without sending unrelated fields', async () => {
    updateAuthoringLessonMock.mockResolvedValue({ ...lesson({ estimated_duration_minutes: null }), revision: 4, draftRevision: 4 })
    await renderPage()
    const duration = await screen.findByRole('spinbutton', { name: /Estimated duration/ })
    await fireEvent.update(duration, '')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(updateAuthoringLessonMock).toHaveBeenCalledWith(draftID, lessonID, { expectedLessonRevision: 3, estimatedDurationMinutes: null }))
  })

  it('keeps Save disabled for unchanged and reverted values and reports local duration validation', async () => {
    await renderPage()
    const title = await screen.findByRole('textbox', { name: /Title/ })
    const save = screen.getByRole('button', { name: 'Save changes' })
    expect(save.hasAttribute('disabled')).toBe(true)
    await fireEvent.update(title, 'Temporary')
    await fireEvent.update(title, 'What is EAI?')
    expect(save.hasAttribute('disabled')).toBe(true)
    const duration = screen.getByRole('spinbutton', { name: /Estimated duration/ })
    await fireEvent.update(duration, '0')
    await fireEvent.click(save)
    expect(await screen.findByText(/Enter a whole number from 1 to 1440/)).toBeTruthy()
    expect(updateAuthoringLessonMock).not.toHaveBeenCalled()
  })

  it('preserves local values on conflict and reloads only on explicit request', async () => {
    updateAuthoringLessonMock.mockRejectedValueOnce(new APIProblemError(409, undefined, undefined))
    getAuthoringLessonMock.mockResolvedValueOnce(lesson()).mockResolvedValueOnce(lesson({ title: 'Latest Lesson', revision: 4 }))
    await renderPage()
    const title = await screen.findByRole('textbox', { name: /Title/ })
    await fireEvent.update(title, 'My local title')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(await screen.findByText(/Lesson changed elsewhere/)).toBeTruthy()
    expect((title as HTMLInputElement).value).toBe('My local title')
    await fireEvent.click(screen.getByRole('button', { name: 'Reload latest Lesson' }))
    await waitFor(() => expect((screen.getByRole('textbox', { name: /Title/ }) as HTMLInputElement).value).toBe('Latest Lesson'))
  })

  it('keeps hidden Lesson results opaque and separates operational failures', async () => {
    getAuthoringLessonMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    const { markDraftUnavailable } = await renderPage()
    await waitFor(() => expect(markDraftUnavailable).toHaveBeenCalledTimes(1))

    cleanup()
    getAuthoringLessonMock.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(lesson())
    await renderPage()
    expect(await screen.findByText('We couldn’t load this Lesson right now.')).toBeTruthy()
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('heading', { level: 2, name: 'Lesson metadata' })).toBeTruthy()
  })

  it('does not retain a late Lesson response after navigating to another Lesson', async () => {
    let resolveFirst: (value: ReturnType<typeof lesson>) => void = () => undefined
    getAuthoringLessonMock.mockImplementationOnce(() => new Promise<ReturnType<typeof lesson>>((resolve) => { resolveFirst = resolve }))
    getAuthoringLessonMock.mockResolvedValueOnce(lesson({ id: secondLessonID, stable_key: 'routing', title: 'Message routing' }))
    const { router } = await renderPage()
    await waitFor(() => expect(getAuthoringLessonMock).toHaveBeenCalledTimes(1))
    await router.push(`/authoring/drafts/${draftID}/lessons/${secondLessonID}`)
    await screen.findByDisplayValue('Message routing')
    resolveFirst(lesson())
    await waitFor(() => expect(screen.queryByDisplayValue('What is EAI?')).toBeNull())
  })
  it.each(['success', 'hidden-404'])('ignores a late Lesson metadata %s after route switching and permits the new Lesson to save', async (outcome) => {
    let resolveSave!: (value: ReturnType<typeof mutation>) => void
    let rejectSave!: (error: Error) => void
    updateAuthoringLessonMock.mockImplementationOnce(() => new Promise((resolve, reject) => { resolveSave = resolve; rejectSave = reject }))
      .mockResolvedValueOnce(mutation({ id: secondLessonID, title: 'New edits', revision: 4 }))
    getAuthoringLessonMock.mockResolvedValueOnce(lesson()).mockResolvedValueOnce(lesson({ id: secondLessonID, title: 'Second Lesson' }))
    const { router, markDraftUnavailable } = await renderPage()
    await fireEvent.update(await screen.findByRole('textbox', { name: /^Title/ }), 'Old edits')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await router.push(`/authoring/drafts/${draftID}/lessons/${secondLessonID}`)
    await screen.findByDisplayValue('Second Lesson')
    if (outcome === 'success') resolveSave(mutation({ title: 'Old result', revision: 4 }))
    else rejectSave(new APIProblemError(404, undefined, undefined))
    await nextTick()
    expect(screen.getByDisplayValue('Second Lesson')).toBeTruthy()
    expect(markDraftUnavailable).not.toHaveBeenCalled()
    await fireEvent.update(screen.getByRole('textbox', { name: /^Title/ }), 'New edits')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(updateAuthoringLessonMock).toHaveBeenLastCalledWith(draftID, secondLessonID, { expectedLessonRevision: 3, title: 'New edits' }))
  })

})
