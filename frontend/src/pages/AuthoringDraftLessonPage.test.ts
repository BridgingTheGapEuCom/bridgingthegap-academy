import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { ref } from 'vue'
import { APIProblemError } from '../api/client'
import { authoringDraftContextKey } from '../authoring/draftContext'

const getAuthoringLessonMock = vi.hoisted(() => vi.fn())
const updateAuthoringLessonMock = vi.hoisted(() => vi.fn())

vi.mock('../authoring/authoring', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../authoring/authoring')>()),
  getAuthoringLesson: getAuthoringLessonMock,
  updateAuthoringLesson: updateAuthoringLessonMock,
}))

import AuthoringDraftLessonPage from './AuthoringDraftLessonPage.vue'

const draftID = '11111111-1111-4111-8111-111111111111'
const moduleID = '22222222-2222-4222-8222-222222222222'
const lessonID = '33333333-3333-4333-8333-333333333333'
const secondLessonID = '55555555-5555-4555-8555-555555555555'
const draft = () => ({
  id: draftID, course_id: '44444444-4444-4444-8444-444444444444', intended_version: '1.0.0', source_language: 'en', title: 'Integration foundations', description: 'Draft.', objectives: ['Explain ownership'], changelog: 'Initial Draft.', license: { kind: 'STANDARD' as const, identifier: '', display_name: 'All Rights Reserved', url: '', custom_text: '' }, status: 'ACTIVE' as const, revision: 3, created_at: '2026-09-15T10:00:00Z', updated_at: '2026-09-15T11:00:00Z',
})
const lesson = (overrides = {}) => ({
  id: lessonID, draft_id: draftID, module_id: moduleID, stable_key: 'what-is-eai', title: 'What is EAI?', description: 'Start here.', objectives: ['Explain EAI', 'Recognise integration boundaries'], estimated_duration_minutes: 15, position: 0, revision: 3, recommended_prerequisite_keys: [], content: { schemaVersion: 1, blocks: [] }, created_at: '2026-09-15T10:00:00Z', updated_at: '2026-09-15T11:00:00Z', ...overrides,
})

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
    getAuthoringLessonMock.mockResolvedValue(lesson())
  })
  afterEach(cleanup)

  it('loads editable Lesson metadata without rendering content or structure controls', async () => {
    await renderPage()
    expect(await screen.findByRole('heading', { level: 2, name: 'Lesson metadata' })).toBeTruthy()
    expect((screen.getByRole('textbox', { name: /Title/ }) as HTMLInputElement).value).toBe('What is EAI?')
    expect((screen.getByRole('textbox', { name: /Description/ }) as HTMLTextAreaElement).value).toBe('Start here.')
    expect(screen.getByText('what-is-eai').tagName).toBe('CODE')
    expect(screen.queryByText('LessonContent')).toBeNull()
    expect(screen.queryByText(/Prerequisite/)).toBeNull()
    expect(screen.queryByRole('button', { name: /Move|Delete lesson|Edit content/ })).toBeNull()
    expect(getAuthoringLessonMock).toHaveBeenCalledWith(draftID, lessonID)
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
})
