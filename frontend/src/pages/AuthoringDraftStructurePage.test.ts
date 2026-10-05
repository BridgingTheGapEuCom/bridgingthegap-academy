import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive, ref } from 'vue'
import { authoringDraftContextKey } from '../authoring/draftContext'
import { renderWithI18n } from '../test/i18n'
import { pseudoLocalize } from '../i18n/pseudo'
import type { ApplicationLocale } from '../i18n/registry'

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

function structureWithLessons() {
  const result = structure()
  const first = result.modules[0]!.lessons[0]!
  result.modules[0]!.lessons = ['First lesson', 'Second lesson', 'Third lesson'].map((title, position) => ({ ...first, id: `44444444-4444-4444-8444-44444444444${position + 1}`, stable_key: title.toLowerCase().replace(' ', '-'), title, position }))
  result.modules[1]!.lessons = [{ ...first, id: '77777777-7777-4777-8777-777777777777', stable_key: 'other-lesson', title: 'Other lesson', position: 0 }]
  return result
}

function reorderStructure(value: ReturnType<typeof structure>, order: Array<{ moduleId: string; lessonIds: string[] }>) {
  const lessons = new Map(value.modules.flatMap((module) => module.lessons.map((lesson) => [lesson.id, lesson])))
  return { modules: value.modules.map((module) => ({ ...module, lessons: (order.find((entry) => entry.moduleId === module.id)?.lessonIds ?? []).map((id, position) => ({ ...lessons.get(id)!, position })) })) }
}

function mount(locale: ApplicationLocale = 'en') {
  const current = ref(draft)
  return renderWithI18n(Page, { global: { provide: { [authoringDraftContextKey as symbol]: {
    draft: current, replaceDraft: (value: typeof draft) => { current.value = value }, markDraftUnavailable: vi.fn(),
  } } } }, locale)
}

function openDialog() {
  return document.querySelector('[role="dialog"]') as HTMLElement
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

    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(within(outline).getByRole('button', { name: 'Expand Foundations' })).toBeTruthy()
    expect(within(outline).getByText('1 lesson')).toBeTruthy()

    await fireEvent.click(within(outline).getByRole('button', { name: 'Expand Foundations' }))
    expect(await within(outline).findByRole('button', { name: /What is EAI/ })).toBeTruthy()

    await fireEvent.update(screen.getByRole('searchbox', { name: 'Search structure' }), 'advanced')
    expect(within(outline).queryByRole('button', { name: /What is EAI/ })).toBeNull()
    expect(within(outline).getByText('Advanced')).toBeTruthy()
  })

  it('renders Structure controls through the pseudo-locale without hardcoded page copy', async () => {
    mount('en-XA')
    expect(await screen.findByRole('heading', { name: pseudoLocalize('Structure') })).toBeTruthy()
    expect(screen.getByRole('searchbox', { name: pseudoLocalize('Search structure') })).toBeTruthy()
    expect(screen.getByRole('button', { name: pseudoLocalize('Expand all') })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Add module' })).toBeNull()
    const outline = screen.getByRole('complementary', { name: pseudoLocalize('Course structure') })
    await fireEvent.click(within(outline).getAllByRole('button', { name: /Foundations/ })[0]!)
    await fireEvent.click(within(outline).getByRole('button', { name: /What is EAI/ }))
    expect(await screen.findByRole('heading', { name: pseudoLocalize('Description') })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Edit content' })).toBeNull()
  })

  it('creates modules and lessons through focused dialogs without stable-key inputs', async () => {
    mocks.createModule.mockResolvedValue({ draftRevision: 4 })
    mocks.createLesson.mockResolvedValue({ draftRevision: 4 })
    mount()
    const outline = await screen.findByRole('complementary', { name: 'Course structure' })

    await fireEvent.click(screen.getAllByRole('button', { name: 'Add module' })[0]!)
    const moduleDialog = openDialog()
    expect(within(moduleDialog).queryByLabelText(/stable key/i)).toBeNull()
    await fireEvent.update(within(moduleDialog).getByRole('textbox', { name: /Module title/ }), 'Messaging')
    await fireEvent.submit(within(moduleDialog).getByRole('button', { name: 'Create module' }).closest('form')!)
    await waitFor(() => expect(mocks.createModule).toHaveBeenCalledWith(draft.id, {
      expectedDraftRevision: 3, title: 'Messaging',
    }))
    await waitFor(() => expect(document.querySelector('[role="dialog"]')).toBeNull())

    await fireEvent.click(within(outline).getByRole('button', { name: 'Expand Foundations' }))
    await fireEvent.click(within(outline).getByRole('button', { name: 'Add lesson' }))
    const lessonDialog = openDialog()
    expect(within(lessonDialog).queryByLabelText(/stable key/i)).toBeNull()
    await fireEvent.update(within(lessonDialog).getByRole('textbox', { name: /Lesson title/ }), 'Routing')
    await fireEvent.update(within(lessonDialog).getByRole('textbox', { name: /Initial lesson description/ }), 'Route safely')
    await fireEvent.update(within(lessonDialog).getByRole('textbox', { name: 'Initial learning objective 1' }), 'Explain routing')
    await fireEvent.submit(within(lessonDialog).getByRole('button', { name: 'Create lesson' }).closest('form')!)
    await waitFor(() => expect(mocks.createLesson).toHaveBeenCalledWith(draft.id, '33333333-3333-4333-8333-333333333333', {
      expectedDraftRevision: 4, title: 'Routing', description: 'Route safely', objectives: ['Explain routing'],
    }))
    await waitFor(() => expect(document.querySelector('[role="dialog"]')).toBeNull())
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

  it('reorders a selected lesson from its keyboard drag handle without changing another module', async () => {
    let current = structureWithLessons()
    mocks.getStructure.mockImplementation(() => Promise.resolve(current))
    mocks.reorderLessons.mockImplementation(async (_draftID, _revision, order) => {
      current = reorderStructure(current, order)
      return { draftRevision: 4 }
    })
    mount()
    const outline = await screen.findByRole('complementary', { name: 'Course structure' })
    await fireEvent.click(within(outline).getByRole('button', { name: 'Expand Foundations' }))
    await fireEvent.click(within(outline).getByRole('button', { name: /Second lesson/ }))
    const handle = within(outline).getByRole('button', { name: 'Reorder lesson 02' })
    await fireEvent.keyDown(handle, { key: ' ' })
    await fireEvent.keyDown(handle, { key: 'ArrowUp' })
    await waitFor(() => expect(mocks.reorderLessons).toHaveBeenCalledWith(draft.id, 3, [
      { moduleId: '33333333-3333-4333-8333-333333333333', lessonIds: ['44444444-4444-4444-8444-444444444442', '44444444-4444-4444-8444-444444444441', '44444444-4444-4444-8444-444444444443'] },
      { moduleId: '55555555-5555-4555-8555-555555555555', lessonIds: ['77777777-7777-4777-8777-777777777777'] },
    ]))
    expect(mocks.getStructure).toHaveBeenCalledTimes(1)
    expect(within(outline).getByRole('button', { name: /Second lesson/ }).getAttribute('aria-pressed')).toBe('true')
    expect(screen.getByText('1 of 3')).toBeTruthy()
  })

  it('does not persist a same-position drop and disables drag handles while search filters the outline', async () => {
    mocks.getStructure.mockResolvedValue(structureWithLessons())
    mount()
    const outline = await screen.findByRole('complementary', { name: 'Course structure' })
    await fireEvent.click(within(outline).getByRole('button', { name: 'Expand Foundations' }))
    const handle = within(outline).getByRole('button', { name: 'Reorder lesson 02' })
    await fireEvent.pointerDown(handle, { pointerId: 1, button: 0 })
    await fireEvent.pointerUp(document, { pointerId: 1 })
    expect(mocks.reorderLessons).not.toHaveBeenCalled()
    await fireEvent.update(screen.getByRole('searchbox', { name: 'Search structure' }), 'second')
    expect(within(outline).getByRole('button', { name: 'Reorder lesson 02' }).hasAttribute('disabled')).toBe(true)
    expect(screen.getByText('Clear search to reorder lessons.')).toBeTruthy()
  })

  it('keeps the selected lesson and authoritative order when reorder persistence fails', async () => {
    const current = structureWithLessons()
    mocks.getStructure.mockResolvedValue(current)
    mocks.reorderLessons.mockRejectedValue(new Error('offline'))
    mount()
    const outline = await screen.findByRole('complementary', { name: 'Course structure' })
    await fireEvent.click(within(outline).getByRole('button', { name: 'Expand Foundations' }))
    await fireEvent.click(within(outline).getByRole('button', { name: /Second lesson/ }))
    const handle = within(outline).getByRole('button', { name: 'Reorder lesson 02' })
    await fireEvent.keyDown(handle, { key: ' ' })
    await fireEvent.keyDown(handle, { key: 'ArrowUp' })
    expect((await screen.findByRole('alert')).textContent).toContain('We couldn’t save this structure right now. Please try again.')
    expect(within(outline).getByRole('button', { name: /Second lesson/ }).getAttribute('aria-pressed')).toBe('true')
    expect(Array.from(outline.querySelectorAll('[data-structure-module-id="33333333-3333-4333-8333-333333333333"]')).map((row) => row.getAttribute('data-structure-lesson-id'))).toEqual(['44444444-4444-4444-8444-444444444441', '44444444-4444-4444-8444-444444444442', '44444444-4444-4444-8444-444444444443'])
  })

  it('surfaces selected lesson content without loading the rest of the outline', async () => {
    mount()
    const outline = await screen.findByRole('complementary', { name: 'Course structure' })
    await fireEvent.click(within(outline).getByRole('button', { name: 'Expand Foundations' }))
    await fireEvent.click(within(outline).getByRole('button', { name: /What is EAI/ }))
    expect(await screen.findByText('No lesson content yet.')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Edit content' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Edit lesson' })).toBeNull()
    expect(screen.getAllByRole('button', { name: 'Edit details' })).toHaveLength(2)
    expect(mocks.getLesson).toHaveBeenCalledWith(draft.id, '44444444-4444-4444-8444-444444444444')
    await fireEvent.click(screen.getByRole('button', { name: 'Edit content' }))
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
    expect(screen.getByText('Text')).toBeTruthy()
    expect(screen.getByText('Callout')).toBeTruthy()
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
    expect(screen.getByRole('button', { name: 'Previous' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Next' }).hasAttribute('disabled')).toBe(true)
    resolveFirst?.({ ...next.modules[0]!.lessons[0], draft_id: draft.id, module_id: next.modules[0]!.id, content: { schemaVersion: 1, blocks: [{ key: 'text', type: 'TEXT', payload: {} }] }, created_at: '', updated_at: '' })
    await waitFor(() => expect(screen.queryByText('Text')).toBeNull())
    expect(screen.getByText('Video')).toBeTruthy()
  })
})
