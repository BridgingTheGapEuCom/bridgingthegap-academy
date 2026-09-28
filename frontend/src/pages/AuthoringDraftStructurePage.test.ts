import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive, ref } from 'vue'
import { authoringDraftContextKey } from '../authoring/draftContext'

const mocks = vi.hoisted(() => ({
  getStructure: vi.fn(), getDraft: vi.fn(), createModule: vi.fn(), updateModule: vi.fn(),
  createLesson: vi.fn(), updateLesson: vi.fn(), reorderModules: vi.fn(), reorderLessons: vi.fn(),
  deleteModule: vi.fn(), deleteLesson: vi.fn(), getLesson: vi.fn(), push: vi.fn(),
}))

const route = reactive({ query: {} as Record<string, string | undefined> })
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRoute: () => route,
  useRouter: () => ({ push: mocks.push }),
}))

vi.mock('../authoring/authoring', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../authoring/authoring')>()),
  getAuthoringStructure: mocks.getStructure,
  getAuthoringDraft: mocks.getDraft,
  createAuthoringModule: mocks.createModule,
  updateAuthoringModule: mocks.updateModule,
  createAuthoringLesson: mocks.createLesson,
  updateAuthoringLesson: mocks.updateLesson,
  reorderAuthoringModules: mocks.reorderModules,
  reorderAuthoringLessons: mocks.reorderLessons,
  deleteAuthoringModule: mocks.deleteModule,
  deleteAuthoringLesson: mocks.deleteLesson,
  getAuthoringLesson: mocks.getLesson,
}))

import Page from './AuthoringDraftStructurePage.vue'

const draft = {
  id: '11111111-1111-4111-8111-111111111111', course_id: '22222222-2222-4222-8222-222222222222',
  intended_version: '1.0.0', source_language: 'en', title: 'Draft', description: 'Description',
  objectives: ['One'], changelog: 'Initial', license: { kind: 'ALL_RIGHTS_RESERVED' as const, identifier: '', display_name: 'All Rights Reserved', url: '', custom_text: '' },
  status: 'ACTIVE' as const, revision: 3, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
}

const structure = () => ({ modules: [
  { id: '33333333-3333-4333-8333-333333333333', stable_key: 'foundation', title: 'Foundations', description: 'Core concepts', position: 0, revision: 2, lessons: [
    { id: '44444444-4444-4444-8444-444444444444', stable_key: 'intro', title: 'What is EAI?', description: 'Start here', objectives: ['Explain EAI'], estimated_duration_minutes: null, position: 0, revision: 2, recommended_prerequisite_keys: [] },
  ] },
  { id: '55555555-5555-4555-8555-555555555555', stable_key: 'advanced', title: 'Advanced', description: '', position: 1, revision: 2, lessons: [] },
] })

function mount() {
  const current = ref(draft)
  return render(Page, { global: { provide: { [authoringDraftContextKey as symbol]: {
    draft: current, replaceDraft: (value: typeof draft) => { current.value = value }, markDraftUnavailable: vi.fn(),
  } } } })
}

function openDialog() {
  return document.querySelector('dialog[open]') as HTMLDialogElement
}

describe('AuthoringDraftStructurePage', () => {
  beforeEach(() => {
    Object.assign(HTMLDialogElement.prototype, {
      showModal(this: HTMLDialogElement) { this.setAttribute('open', '') },
      close(this: HTMLDialogElement) { this.removeAttribute('open') },
    })
    for (const mock of Object.values(mocks)) mock.mockReset()
    route.query = {}
    mocks.getStructure.mockResolvedValue(structure())
    mocks.getLesson.mockResolvedValue({
      ...structure().modules[0]!.lessons[0], draft_id: draft.id, module_id: structure().modules[0]!.id,
      content: { schemaVersion: 1, blocks: [] }, created_at: '', updated_at: '',
    })
  })

  afterEach(cleanup)

  it('renders a collapsed searchable outline without persistent creation forms', async () => {
    mount()
    const outline = await screen.findByRole('complementary', { name: 'Course structure' })

    expect(document.querySelector('dialog[open]')).toBeNull()
    expect(within(outline).getByRole('button', { name: 'Expand Foundations' })).toBeTruthy()
    expect(within(outline).getByText('1 lesson')).toBeTruthy()

    await fireEvent.click(within(outline).getByRole('button', { name: 'Expand Foundations' }))
    expect(await within(outline).findByRole('button', { name: /What is EAI/ })).toBeTruthy()

    await fireEvent.update(screen.getByRole('searchbox', { name: 'Search structure' }), 'advanced')
    expect(within(outline).queryByRole('button', { name: /What is EAI/ })).toBeNull()
    expect(within(outline).getByText('Advanced')).toBeTruthy()
  })

  it('creates modules and lessons through focused dialogs without stable-key inputs', async () => {
    mocks.createModule.mockResolvedValue({ draftRevision: 4 })
    mocks.createLesson.mockResolvedValue({ draftRevision: 4 })
    mount()
    const outline = await screen.findByRole('complementary', { name: 'Course structure' })

    await fireEvent.click(screen.getByRole('button', { name: '+ Add module' }))
    const moduleDialog = openDialog()
    expect(within(moduleDialog).queryByLabelText(/stable key/i)).toBeNull()
    await fireEvent.update(within(moduleDialog).getByRole('textbox', { name: /Module title/ }), 'Messaging')
    await fireEvent.submit(within(moduleDialog).getByRole('button', { name: 'Create module' }).closest('form')!)
    await waitFor(() => expect(mocks.createModule).toHaveBeenCalledWith(draft.id, {
      expectedDraftRevision: 3, title: 'Messaging',
    }))

    await fireEvent.click(within(outline).getByRole('button', { name: 'Expand Foundations' }))
    await fireEvent.click(within(outline).getByRole('button', { name: '+ Add lesson' }))
    const lessonDialog = openDialog()
    expect(within(lessonDialog).queryByLabelText(/stable key/i)).toBeNull()
    await fireEvent.update(within(lessonDialog).getByRole('textbox', { name: /Lesson title/ }), 'Routing')
    await fireEvent.update(within(lessonDialog).getByRole('textbox', { name: /Initial lesson description/ }), 'Route safely')
    await fireEvent.update(within(lessonDialog).getByRole('textbox', { name: 'Initial learning objective 1' }), 'Explain routing')
    await fireEvent.submit(within(lessonDialog).getByRole('button', { name: 'Create lesson' }).closest('form')!)
    await waitFor(() => expect(mocks.createLesson).toHaveBeenCalledWith(draft.id, '33333333-3333-4333-8333-333333333333', {
      expectedDraftRevision: 4, title: 'Routing', description: 'Route safely', objectives: ['Explain routing'],
    }))
  })

  it('moves a selected lesson with the authoritative full structure map', async () => {
    mocks.reorderLessons.mockResolvedValue({ draftRevision: 4 })
    mount()
    const outline = await screen.findByRole('complementary', { name: 'Course structure' })

    await fireEvent.click(within(outline).getByRole('button', { name: 'Expand Foundations' }))
    await fireEvent.click(within(outline).getByRole('button', { name: /What is EAI/ }))
    await fireEvent.click(await screen.findByRole('button', { name: 'Move lesson' }))
    const moveDialog = openDialog()
    await fireEvent.change(within(moveDialog).getByRole('combobox', { name: 'Module' }), {
      target: { value: '55555555-5555-4555-8555-555555555555' },
    })
    await fireEvent.submit(within(moveDialog).getByRole('button', { name: 'Move lesson' }).closest('form')!)
    await waitFor(() => expect(mocks.reorderLessons).toHaveBeenCalledWith(draft.id, 3, [
      { moduleId: '33333333-3333-4333-8333-333333333333', lessonIds: [] },
      { moduleId: '55555555-5555-4555-8555-555555555555', lessonIds: ['44444444-4444-4444-8444-444444444444'] },
    ]))
  })

  it('surfaces selected lesson content without loading the rest of the outline', async () => {
    mount()
    const outline = await screen.findByRole('complementary', { name: 'Course structure' })
    await fireEvent.click(within(outline).getByRole('button', { name: 'Expand Foundations' }))
    await fireEvent.click(within(outline).getByRole('button', { name: /What is EAI/ }))
    expect(await screen.findByText('No lesson content yet.')).toBeTruthy()
    expect(screen.getByRole('button', { name: '+ Add content' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Edit lesson' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Edit details' })).toBeTruthy()
    expect(mocks.getLesson).toHaveBeenCalledWith(draft.id, '44444444-4444-4444-8444-444444444444')
    await fireEvent.click(screen.getByRole('button', { name: '+ Add content' }))
    expect(mocks.push).toHaveBeenCalledWith(expect.objectContaining({
      name: 'authoring-draft-lesson-content',
      params: { draftId: draft.id, lessonId: '44444444-4444-4444-8444-444444444444' },
    }))
  })

  it('shows a compact summary for existing selected lesson content', async () => {
    mocks.getLesson.mockResolvedValue({
      ...structure().modules[0]!.lessons[0], draft_id: draft.id, module_id: structure().modules[0]!.id,
      content: { schemaVersion: 1, blocks: [{ key: 'text', type: 'TEXT', payload: {} }, { key: 'callout', type: 'CALLOUT', payload: {} }] }, created_at: '', updated_at: '',
    })
    mount()
    const outline = await screen.findByRole('complementary', { name: 'Course structure' })
    await fireEvent.click(within(outline).getByRole('button', { name: 'Expand Foundations' }))
    await fireEvent.click(within(outline).getByRole('button', { name: /What is EAI/ }))
    expect(await screen.findByText('2 blocks')).toBeTruthy()
    expect(screen.getByText('Text · Callout')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Edit content' })).toBeTruthy()
  })

  it('does not show a late content summary for a lesson that is no longer selected', async () => {
    const secondLesson = { ...structure().modules[0]!.lessons[0]!, id: '66666666-6666-4666-8666-666666666666', stable_key: 'second', title: 'Second lesson', position: 1 }
    const next = structure()
    next.modules[0]!.lessons.push(secondLesson)
    let resolveFirst: ((value: unknown) => void) | undefined
    mocks.getStructure.mockResolvedValue(next)
    mocks.getLesson.mockImplementation((_draftID, lessonID) => lessonID === secondLesson.id
      ? Promise.resolve({ ...secondLesson, draft_id: draft.id, module_id: next.modules[0]!.id, content: { schemaVersion: 1, blocks: [{ key: 'video', type: 'VIDEO', payload: {} }] }, created_at: '', updated_at: '' })
      : new Promise((resolve) => { resolveFirst = resolve }))
    mount()
    const outline = await screen.findByRole('complementary', { name: 'Course structure' })
    await fireEvent.click(within(outline).getByRole('button', { name: 'Expand Foundations' }))
    await fireEvent.click(within(outline).getByRole('button', { name: /What is EAI/ }))
    await fireEvent.click(within(outline).getByRole('button', { name: /Second lesson/ }))
    expect(await screen.findByText('Video')).toBeTruthy()
    resolveFirst?.({ ...next.modules[0]!.lessons[0], draft_id: draft.id, module_id: next.modules[0]!.id, content: { schemaVersion: 1, blocks: [{ key: 'text', type: 'TEXT', payload: {} }] }, created_at: '', updated_at: '' })
    await waitFor(() => expect(screen.queryByText('Text')).toBeNull())
    expect(screen.getByText('Video')).toBeTruthy()
  })
})
