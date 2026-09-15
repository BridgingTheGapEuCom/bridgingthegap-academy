import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { APIProblemError } from '../api/client'
import type { AuthoringLessonContent, AuthoringLessonDetail } from '../authoring/authoring'
import { deferredBlocks } from '../authoring/contentEditor.fixtures'
import AuthoringLessonContentEditor from './AuthoringLessonContentEditor.vue'

const replaceContent = vi.hoisted(() => vi.fn())
const getLesson = vi.hoisted(() => vi.fn())
vi.mock('../authoring/authoring', async (original) => ({ ...(await original<typeof import('../authoring/authoring')>()), replaceAuthoringLessonContent: replaceContent, getAuthoringLesson: getLesson }))
const draftID = '11111111-1111-4111-8111-111111111111'
const lessonID = '22222222-2222-4222-8222-222222222222'
function lesson(content: AuthoringLessonContent = { schemaVersion: 1, blocks: [] }): AuthoringLessonDetail {
  return { id: lessonID, draft_id: draftID, module_id: '33333333-3333-4333-8333-333333333333', stable_key: 'intro', title: 'Introduction', description: '', objectives: [], estimated_duration_minutes: null, position: 0, revision: 7, recommended_prerequisite_keys: [], content, created_at: '', updated_at: '' }
}
async function add(type: string) {
  await fireEvent.update(screen.getByRole('combobox', { name: 'Block type' }), type)
  await fireEvent.click(screen.getByRole('button', { name: 'Add block' }))
}
function savedDocument(): AuthoringLessonContent { return replaceContent.mock.calls[0]![2].content }

describe('AuthoringLessonContentEditor', () => {
  beforeEach(() => {
    getLesson.mockReset(); replaceContent.mockReset()
    replaceContent.mockImplementation(async (_draft, _lesson, request) => ({ lesson: { ...lesson(request.content), revision: 8, draftRevision: 12 }, content: request.content }))
  })
  afterEach(cleanup)

  it('creates all six block types, edits canonical payloads, and manually saves the complete document', async () => {
    const { emitted } = render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson() } })
    const save = screen.getByRole('button', { name: 'Save Lesson content' })
    expect(save.hasAttribute('disabled')).toBe(true)
    for (const type of ['TEXT', 'HEADING', 'CODE', 'QUOTE', 'CALLOUT', 'DIVIDER']) await add(type)
    await fireEvent.update(screen.getByRole('textbox', { name: 'Block 1 text · Paragraph 1 · Text 1' }), 'Learning text')
    await fireEvent.update(screen.getByRole('combobox', { name: 'Heading level' }), '3')
    await fireEvent.update(screen.getByRole('textbox', { name: 'Block 2 heading · Text 1' }), 'A heading')
    await fireEvent.update(screen.getByRole('textbox', { name: 'Code' }), '  const x = 1;\n\n')
    await fireEvent.update(screen.getByRole('textbox', { name: 'Code language' }), 'typescript')
    await fireEvent.update(screen.getByRole('textbox', { name: 'Code title' }), 'Example')
    await fireEvent.update(screen.getByRole('textbox', { name: 'Quote attribution' }), 'Author')
    await fireEvent.update(screen.getByRole('textbox', { name: 'Quote source URL' }), 'https://example.org/source')
    await fireEvent.update(screen.getByRole('combobox', { name: 'Callout kind' }), 'TIP')
    await fireEvent.update(screen.getByRole('textbox', { name: 'Callout title' }), 'Remember')
    expect(replaceContent).not.toHaveBeenCalled()
    await fireEvent.click(save)
    await screen.findByText('Lesson content saved.')
    const content = savedDocument()
    expect(content.blocks.map((block) => block.type)).toEqual(['TEXT', 'HEADING', 'CODE', 'QUOTE', 'CALLOUT', 'DIVIDER'])
    expect(new Set(content.blocks.map((block) => block.key)).size).toBe(6)
    expect(content.blocks[1]?.payload).toEqual({ level: 3, content: [{ type: 'text', text: 'A heading' }] })
    expect(content.blocks[2]?.payload).toEqual({ code: '  const x = 1;\n\n', language: 'typescript', title: 'Example' })
    expect(replaceContent.mock.calls[0]![2].expectedLessonRevision).toBe(7)
    expect(Object.keys(content)).toEqual(['schemaVersion', 'blocks'])
    expect(emitted<unknown[]>().saved?.[0]?.[0]).toMatchObject({ lesson: { revision: 8, draftRevision: 12 } })
    expect(save.hasAttribute('disabled')).toBe(true)
  })

  it('edits text runs losslessly while retaining lists, marks, links, breaks, and canonical keys', async () => {
    const content: AuthoringLessonContent = { schemaVersion: 1, blocks: [{ key: 'rich-text', type: 'TEXT', payload: { content: { nodes: [
      { type: 'paragraph', content: [{ type: 'text', text: 'Strong', marks: [{ type: 'strong' }, { type: 'link', href: '/courses' }] }, { type: 'hard_break' }, { type: 'text', text: 'Code', marks: [{ type: 'inline_code' }] }] },
      { type: 'bullet_list', items: [[{ type: 'text', text: 'Bullet', marks: [{ type: 'emphasis' }] }]] },
      { type: 'ordered_list', items: [[{ type: 'text', text: 'Ordered' }]] },
    ] } } }] }
    render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson(content) } })
    await fireEvent.update(screen.getByRole('textbox', { name: 'Block 1 text · Paragraph 1 · Text 1' }), '<script>alert(1)</script>')
    await fireEvent.update(screen.getByRole('textbox', { name: 'Block 1 text · List 2 · Item 1 · Text 1' }), 'Edited bullet')
    await fireEvent.click(screen.getByRole('button', { name: 'Save Lesson content' }))
    await screen.findByText('Lesson content saved.')
    const expected = structuredClone(content)
    if (expected.blocks[0]?.type === 'TEXT') {
      expected.blocks[0].payload.content.nodes[0]!.content![0]!.text = '<script>alert(1)</script>'
      expected.blocks[0].payload.content.nodes[1]!.items![0]![0]!.text = 'Edited bullet'
    }
    expect(savedDocument()).toEqual(expected)
    expect(document.querySelector('script')).toBeNull()
    expect(content.blocks[0]).not.toEqual(expected.blocks[0])
  })

  it('preserves deferred blocks exactly through movement/save and permits explicit removal', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson({ schemaVersion: 1, blocks: deferredBlocks }) } })
    const list = screen.getByRole('list', { name: 'Lesson content blocks' })
    expect(within(list).getAllByRole('listitem')).toHaveLength(6)
    expect(document.querySelectorAll('img,video,audio,a[href]')).toHaveLength(0)
    await fireEvent.click(screen.getByRole('button', { name: 'Move block 1 image down' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Save Lesson content' }))
    await screen.findByText('Lesson content saved.')
    expect(savedDocument().blocks).toEqual([deferredBlocks[1], deferredBlocks[0], ...deferredBlocks.slice(2)])
    await fireEvent.click(screen.getByRole('button', { name: 'Remove block 1 video' }))
    expect(within(list).getAllByRole('listitem')).toHaveLength(5)
  })

  it('clears semantic dirty state when edits or movements are reverted', async () => {
    const content: AuthoringLessonContent = { schemaVersion: 1, blocks: [{ key: 'quote', type: 'QUOTE', payload: { text: 'Original' } }, { key: 'divider', type: 'DIVIDER', payload: {} }] }
    render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson(content) } })
    const input = screen.getByRole('textbox', { name: 'Quote text' })
    await fireEvent.update(input, 'Changed'); await fireEvent.update(input, 'Original')
    await fireEvent.click(screen.getByRole('button', { name: 'Move block 1 Quote down' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Move block 2 Quote up' }))
    expect(screen.getByRole('button', { name: 'Save Lesson content' }).hasAttribute('disabled')).toBe(true)
    expect(replaceContent).not.toHaveBeenCalled()
  })

  it('preserves local content after 409 and explicitly reloads the authoritative Lesson', async () => {
    replaceContent.mockRejectedValueOnce(new APIProblemError(409, undefined, undefined))
    getLesson.mockResolvedValue({ ...lesson(), revision: 9 })
    const { emitted } = render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson() } })
    await add('CODE')
    await fireEvent.click(screen.getByRole('button', { name: 'Save Lesson content' }))
    await screen.findByText(/unsaved blocks are still here/)
    expect(screen.getByRole('textbox', { name: 'Code' })).toBeTruthy()
    expect(getLesson).not.toHaveBeenCalled()
    expect(replaceContent).toHaveBeenCalledTimes(1)
    await fireEvent.click(screen.getByRole('button', { name: 'Reload latest content' }))
    await screen.findByText('No content blocks yet. Add a block to begin.')
    expect(emitted<unknown[]>().replaceLesson?.[0]?.[0]).toMatchObject({ revision: 9 })
  })

  it.each([400, 413, 500, 401, 404])('handles HTTP %s safely without discarding local content', async (status) => {
    replaceContent.mockRejectedValueOnce(new APIProblemError(status, undefined, undefined))
    const { emitted } = render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson() } })
    await add('CODE'); await fireEvent.click(screen.getByRole('button', { name: 'Save Lesson content' }))
    if (status === 404) await waitFor(() => expect(emitted().unavailable).toHaveLength(1))
    else expect(await screen.findByRole('alert')).toBeTruthy()
    expect(screen.getByRole('textbox', { name: 'Code' })).toBeTruthy()
  })

  it('prevents duplicate submissions and validates empty runs before requesting', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson() } })
    await add('TEXT')
    await fireEvent.update(screen.getByRole('textbox', { name: 'Block 1 text · Paragraph 1 · Text 1' }), '')
    await fireEvent.submit(document.querySelector('form')!)
    expect(replaceContent).not.toHaveBeenCalled()
    expect(screen.getByRole('textbox').getAttribute('aria-invalid')).toBe('true')
    await fireEvent.update(screen.getByRole('textbox'), 'Valid')
    replaceContent.mockImplementationOnce(() => new Promise(() => {}))
    await fireEvent.submit(document.querySelector('form')!); await fireEvent.submit(document.querySelector('form')!)
    expect(replaceContent).toHaveBeenCalledTimes(1)
    expect((screen.getByRole('textbox') as HTMLTextAreaElement).closest('fieldset')?.disabled).toBe(false)
    expect(document.querySelector<HTMLFieldSetElement>('.authoring-content__controls')?.disabled).toBe(true)
  })

  it('does not render editable controls for unknown schemas or types', () => {
    const future = { schemaVersion: 2, blocks: [] } as unknown as AuthoringLessonContent
    const { rerender } = render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson(future) } })
    expect(screen.getByRole('alert').textContent).toContain('cannot be edited safely')
    expect(screen.queryByRole('button', { name: 'Save Lesson content' })).toBeNull()
    return rerender({ lesson: lesson({ schemaVersion: 1, blocks: [{ key: 'widget', type: 'WIDGET', payload: {} }] } as unknown as AuthoringLessonContent) }).then(() => expect(screen.getByRole('alert')).toBeTruthy())
  })

  it('preserves unsaved work if a different section reloads newer content', async () => {
    const { rerender } = render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson() } })
    await add('CODE')
    await rerender({ lesson: lesson({ schemaVersion: 1, blocks: [{ key: 'new', type: 'DIVIDER', payload: {} }] }) })
    expect(screen.getByRole('textbox', { name: 'Code' })).toBeTruthy()
    expect(screen.getByText(/unsaved blocks are still here/)).toBeTruthy()
  })
})
