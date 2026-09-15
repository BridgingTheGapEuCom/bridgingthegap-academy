import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { ref } from 'vue'
import { APIProblemError } from '../api/client'
import { authoringDraftContextKey } from '../authoring/draftContext'

const getAuthoringStructureMock = vi.hoisted(() => vi.fn())
const getAuthoringDraftMock = vi.hoisted(() => vi.fn())
const createAuthoringModuleMock = vi.hoisted(() => vi.fn())
const updateAuthoringModuleMock = vi.hoisted(() => vi.fn())
const reorderAuthoringModulesMock = vi.hoisted(() => vi.fn())
const deleteAuthoringModuleMock = vi.hoisted(() => vi.fn())
const createAuthoringLessonMock = vi.hoisted(() => vi.fn())
const reorderAuthoringLessonsMock = vi.hoisted(() => vi.fn())
const deleteAuthoringLessonMock = vi.hoisted(() => vi.fn())

vi.mock('../authoring/authoring', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../authoring/authoring')>()),
  getAuthoringStructure: getAuthoringStructureMock,
  getAuthoringDraft: getAuthoringDraftMock,
  createAuthoringModule: createAuthoringModuleMock,
  updateAuthoringModule: updateAuthoringModuleMock,
  reorderAuthoringModules: reorderAuthoringModulesMock,
  deleteAuthoringModule: deleteAuthoringModuleMock,
  createAuthoringLesson: createAuthoringLessonMock,
  reorderAuthoringLessons: reorderAuthoringLessonsMock,
  deleteAuthoringLesson: deleteAuthoringLessonMock,
}))

import AuthoringDraftStructurePage from './AuthoringDraftStructurePage.vue'

const draftID = '11111111-1111-4111-8111-111111111111'
const moduleAID = '22222222-2222-4222-8222-222222222222'
const moduleBID = '33333333-3333-4333-8333-333333333333'
const lessonOneID = '44444444-4444-4444-8444-444444444444'
const lessonTwoID = '55555555-5555-4555-8555-555555555555'

const draft = () => ({
  id: draftID, course_id: '66666666-6666-4666-8666-666666666666', intended_version: '1.0.0', source_language: 'en', title: 'Integration foundations', description: 'Draft.', objectives: ['Explain ownership'], changelog: 'Initial Draft.', license: { kind: 'STANDARD' as const, identifier: '', display_name: 'All Rights Reserved', url: '', custom_text: '' }, status: 'ACTIVE' as const, revision: 3, created_at: '2026-09-15T10:00:00Z', updated_at: '2026-09-15T11:00:00Z',
})

const structure = () => ({
  modules: [
    { id: moduleAID, stable_key: 'foundations', title: 'Foundations', description: 'Core concepts.', position: 0, revision: 2, lessons: [
      { id: lessonOneID, stable_key: 'what-is-eai', title: 'What is EAI?', description: 'Start here.', objectives: ['Explain EAI'], estimated_duration_minutes: null, position: 0, revision: 2, recommended_prerequisite_keys: [] },
      { id: lessonTwoID, stable_key: 'sync-vs-async', title: 'Sync and async', description: 'Compare approaches.', objectives: ['Compare approaches'], estimated_duration_minutes: null, position: 1, revision: 2, recommended_prerequisite_keys: [] },
    ] },
    { id: moduleBID, stable_key: 'advanced', title: 'Advanced', description: '', position: 1, revision: 2, lessons: [] },
  ],
})

async function renderPage() {
  const currentDraft = ref(draft())
  const replaceDraft = vi.fn((nextDraft) => { currentDraft.value = nextDraft })
  const markDraftUnavailable = vi.fn()
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/authoring/drafts/:draftId/structure', component: AuthoringDraftStructurePage }, { path: '/authoring/drafts/:draftId/lessons/:lessonId', component: { template: '<p>Lesson</p>' } }] })
  await router.push(`/authoring/drafts/${draftID}/structure`)
  await router.isReady()
  return {
    ...render(AuthoringDraftStructurePage, { global: { plugins: [router], provide: { [authoringDraftContextKey as symbol]: { draft: currentDraft, replaceDraft, markDraftUnavailable } } } }),
    currentDraft, replaceDraft, markDraftUnavailable, router,
  }
}

describe('AuthoringDraftStructurePage', () => {
  beforeEach(() => {
    for (const mock of [getAuthoringStructureMock, getAuthoringDraftMock, createAuthoringModuleMock, updateAuthoringModuleMock, reorderAuthoringModulesMock, deleteAuthoringModuleMock, createAuthoringLessonMock, reorderAuthoringLessonsMock, deleteAuthoringLessonMock]) mock.mockReset()
    getAuthoringStructureMock.mockResolvedValue(structure())
  })
  afterEach(cleanup)

  it('renders backend ordering without requesting LessonContent and exposes semantic structure controls', async () => {
    await renderPage()
    await screen.findByRole('heading', { level: 3, name: 'Foundations' })
    expect(screen.getAllByRole('heading', { level: 3 }).map((heading) => heading.textContent)).toEqual(['Add module', 'Foundations', 'Advanced'])
    expect(screen.getAllByRole('link', { name: 'What is EAI?' })[0]?.getAttribute('href')).toBe(`/authoring/drafts/${draftID}/lessons/${lessonOneID}`)
    const foundations = screen.getByRole('heading', { level: 3, name: 'Foundations' }).closest('section') as HTMLElement
    expect(within(foundations).queryByRole('textbox', { name: /Module stable key/ })).toBeNull()
    expect(getAuthoringStructureMock).toHaveBeenCalledWith(draftID)
    expect(document.body.textContent).not.toContain('LessonContent')
    expect(screen.getAllByRole('button', { name: 'Delete empty module' })[0]?.hasAttribute('disabled')).toBe(true)
  })

  it('creates and updates Modules with authoritative revisions', async () => {
    createAuthoringModuleMock.mockResolvedValue({ module: structure().modules[1], draftRevision: 4 })
    updateAuthoringModuleMock.mockResolvedValue({ module: { ...structure().modules[0], title: 'Updated foundations', revision: 3 }, draftRevision: 5 })
    const { currentDraft } = await renderPage()
    await screen.findByRole('heading', { level: 3, name: 'Foundations' })
    await fireEvent.update(screen.getByRole('textbox', { name: /Module stable key/ }), 'advanced-patterns')
    await fireEvent.update(screen.getByRole('textbox', { name: /Module title/ }), 'Advanced patterns')
    await fireEvent.click(screen.getByRole('button', { name: 'Create module' }))
    await waitFor(() => expect(createAuthoringModuleMock).toHaveBeenCalledWith(draftID, expect.objectContaining({ expectedDraftRevision: 3, position: 2, stableKey: 'advanced-patterns' })))
    expect(currentDraft.value.revision).toBe(4)

    const foundations = screen.getByRole('heading', { level: 3, name: 'Foundations' }).closest('section') as HTMLElement
    await fireEvent.click(within(foundations).getByRole('button', { name: 'Edit module' }))
    await fireEvent.update(within(foundations).getByRole('textbox', { name: /Module title/ }), 'Updated foundations')
    await fireEvent.click(within(foundations).getByRole('button', { name: 'Save module' }))
    await waitFor(() => expect(updateAuthoringModuleMock).toHaveBeenCalledWith(draftID, moduleAID, { expectedModuleRevision: 2, title: 'Updated foundations' }))
  })

  it('sends a complete authoritative arrangement for lesson moves and preserves stable IDs', async () => {
    reorderAuthoringLessonsMock.mockResolvedValue({ draftRevision: 4 })
    await renderPage()
    await screen.findByRole('heading', { level: 3, name: 'Foundations' })
    const foundations = screen.getByRole('heading', { level: 3, name: 'Foundations' }).closest('section') as HTMLElement
    const firstLesson = within(foundations).getByRole('link', { name: 'What is EAI?' }).closest('li') as HTMLElement
    await fireEvent.click(within(firstLesson).getByRole('button', { name: 'Move to next module' }))
    await waitFor(() => expect(reorderAuthoringLessonsMock).toHaveBeenCalledWith(draftID, 3, [
      { moduleId: moduleAID, lessonIds: [lessonTwoID] },
      { moduleId: moduleBID, lessonIds: [lessonOneID] },
    ]))
  })

  it('creates Lessons with only required initial metadata and deletes only through the structural API', async () => {
    createAuthoringLessonMock.mockResolvedValue({ ...structure().modules[0].lessons[0], draft_id: draftID, module_id: moduleAID, draftRevision: 4 })
    deleteAuthoringModuleMock.mockResolvedValue({ draftRevision: 5 })
    deleteAuthoringLessonMock.mockResolvedValue({ draftRevision: 5 })
    await renderPage()
    await screen.findByRole('heading', { level: 3, name: 'Foundations' })
    const foundations = screen.getByRole('heading', { level: 3, name: 'Foundations' }).closest('section') as HTMLElement
    await fireEvent.update(within(foundations).getByRole('textbox', { name: /Lesson stable key/ }), 'message-routing')
    await fireEvent.update(within(foundations).getByRole('textbox', { name: /Lesson title/ }), 'Message routing')
    await fireEvent.update(within(foundations).getByRole('textbox', { name: /Initial lesson description/ }), 'Route messages safely.')
    await fireEvent.update(within(foundations).getByRole('textbox', { name: /Initial learning objectives/ }), 'Explain routing')
    await fireEvent.click(within(foundations).getByRole('button', { name: 'Create lesson' }))
    await waitFor(() => expect(createAuthoringLessonMock).toHaveBeenCalledWith(draftID, moduleAID, expect.objectContaining({
      expectedDraftRevision: 3,
      stableKey: 'message-routing',
      title: 'Message routing',
      description: 'Route messages safely.',
      objectives: ['Explain routing'],
      position: 2,
    })))
    await screen.findByText('Lesson created.')

    const advanced = screen.getByRole('heading', { level: 3, name: 'Advanced' }).closest('section') as HTMLElement
    await fireEvent.click(within(advanced).getByRole('button', { name: 'Delete empty module' }))
    await waitFor(() => expect(deleteAuthoringModuleMock).toHaveBeenCalledWith(draftID, moduleBID, 4, 2))
    await screen.findByText('Empty Module deleted.')

    const currentFoundations = screen.getByRole('heading', { level: 3, name: 'Foundations' }).closest('section') as HTMLElement
    const firstLesson = within(currentFoundations).getByRole('link', { name: 'What is EAI?' }).closest('li') as HTMLElement
    await fireEvent.click(within(firstLesson).getByRole('button', { name: 'Delete lesson' }))
    await waitFor(() => expect(deleteAuthoringLessonMock).toHaveBeenCalledWith(draftID, lessonOneID, 5, 2))
  })

  it('does not overwrite a stale structure and reloads only on request', async () => {
    reorderAuthoringModulesMock.mockRejectedValueOnce(new APIProblemError(409, undefined, undefined))
    getAuthoringDraftMock.mockResolvedValue({ ...draft(), revision: 4 })
    const { currentDraft } = await renderPage()
    await screen.findByRole('heading', { level: 3, name: 'Foundations' })
    const foundations = screen.getByRole('heading', { level: 3, name: 'Foundations' }).closest('section') as HTMLElement
    await fireEvent.click(within(foundations).getByRole('button', { name: 'Move module down' }))
    expect(await screen.findByText(/Draft changed elsewhere/)).toBeTruthy()
    expect(reorderAuthoringModulesMock).toHaveBeenCalledTimes(1)
    await fireEvent.click(screen.getByRole('button', { name: 'Reload latest Draft' }))
    await waitFor(() => expect(currentDraft.value.revision).toBe(4))
  })

  it('keeps hidden structure responses opaque and separates operational failures', async () => {
    getAuthoringStructureMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    const { markDraftUnavailable } = await renderPage()
    await waitFor(() => expect(markDraftUnavailable).toHaveBeenCalledTimes(1))

    cleanup()
    getAuthoringStructureMock.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(structure())
    await renderPage()
    expect(await screen.findByText('We couldn’t load this Draft structure right now.')).toBeTruthy()
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('heading', { level: 3, name: 'Foundations' })).toBeTruthy()
  })
})
