import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { APIProblemError } from '../api/client'
import type { AuthoringLessonContent, AuthoringLessonDetail } from '../authoring/authoring'
import { deferredBlocks } from '../authoring/contentEditor.fixtures'
import AuthoringLessonContentEditor from './AuthoringLessonContentEditor.vue'

const replaceContent = vi.hoisted(() => vi.fn())
const getLesson = vi.hoisted(() => vi.fn())
const uploadAsset = vi.hoisted(() => vi.fn())
const listAssets = vi.hoisted(() => vi.fn())
const listAssessments = vi.hoisted(() => vi.fn())
vi.mock('../authoring/authoring', async (original) => ({ ...(await original<typeof import('../authoring/authoring')>()), replaceAuthoringLessonContent: replaceContent, getAuthoringLesson: getLesson, uploadAuthoringAsset: uploadAsset, listAuthoringDraftAssets: listAssets, listAuthoringDraftAssessments: listAssessments }))
const draftID = '11111111-1111-4111-8111-111111111111'
const lessonID = '22222222-2222-4222-8222-222222222222'
function lesson(content: AuthoringLessonContent = { schemaVersion: 1, blocks: [] }): AuthoringLessonDetail {
  return { id: lessonID, draft_id: draftID, module_id: '33333333-3333-4333-8333-333333333333', stable_key: 'intro', title: 'Introduction', description: '', objectives: [], estimated_duration_minutes: null, position: 0, revision: 7, recommended_prerequisite_keys: [], content, created_at: '', updated_at: '' }
}
async function add(type: string) {
  await fireEvent.update(screen.getByRole('combobox', { name: 'Block type' }), type)
  await fireEvent.click(screen.getByRole('button', { name: 'Add block' }))
}
async function chooseFile(label: string, file: File) {
  const input = screen.getByLabelText(label) as HTMLInputElement
  Object.defineProperty(input, 'files', { configurable: true, value: [file] })
  await fireEvent.change(input)
}
function savedDocument(): AuthoringLessonContent { return replaceContent.mock.calls[0]![2].content }

describe('AuthoringLessonContentEditor', () => {
  beforeEach(() => {
    getLesson.mockReset(); replaceContent.mockReset(); uploadAsset.mockReset(); listAssets.mockReset(); listAssessments.mockReset()
    replaceContent.mockImplementation(async (_draft, _lesson, request) => ({ lesson: { ...lesson(request.content), revision: 8, draftRevision: 12 }, content: request.content }))
    uploadAsset.mockResolvedValue({ assetKey: '55555555-5555-4555-8555-555555555555', filename: 'diagram.png', mediaType: 'image/png', byteSize: 200, status: 'AVAILABLE' })
    listAssets.mockResolvedValue({ items: [], limit: 20, offset: 0, total: 0 })
    listAssessments.mockResolvedValue({ items: [], limit: 20, offset: 0, total: 0 })
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

  it('provides native uploads for all asset-capable block types', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson() } })
    for (const type of ['IMAGE', 'VIDEO', 'AUDIO', 'DOWNLOAD']) await add(type)
    expect(screen.getByLabelText('Image file').getAttribute('accept')).toBe('image/*')
    expect(screen.getByLabelText('Video file').getAttribute('accept')).toBe('video/*')
    expect(screen.getByLabelText('Audio file').getAttribute('accept')).toBe('audio/*')
    expect(screen.getByLabelText('Download file').getAttribute('accept')).toBeNull()
    expect(screen.getByLabelText('Captions file')).toBeTruthy()
  })

  it('chooses a Draft Assessment for a knowledge check and saves only assessmentKey', async () => {
    listAssessments.mockResolvedValue({ items: [{ assessmentKey: '77777777-7777-4777-8777-777777777777', title: 'Terminology check', questionCount: 2, revision: 3, updatedAt: '2026-09-17T10:00:00Z' }], limit: 20, offset: 0, total: 1 })
    render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson() } })
    await add('KNOWLEDGE_CHECK')
    await screen.findByRole('button', { name: 'Use Terminology check' })
    expect(screen.queryByText('correctOptionKeys')).toBeNull()
    await fireEvent.click(screen.getByRole('button', { name: 'Use Terminology check' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Save Lesson content' }))
    await screen.findByText('Lesson content saved.')
    expect(savedDocument().blocks[0]).toEqual({ key: expect.any(String), type: 'KNOWLEDGE_CHECK', payload: { assessmentKey: '77777777-7777-4777-8777-777777777777' } })
    expect(JSON.stringify(savedDocument())).not.toContain('Terminology check')
  })

  it('lists only compatible Draft Assets and attaches the selected key through the normal save', async () => {
    const original = { key: 'image', type: 'IMAGE' as const, payload: { asset: { assetKey: 'legacy-key' }, decorative: false, altText: 'Existing diagram' } }
    listAssets.mockResolvedValue({ items: [
      { assetKey: '55555555-5555-4555-8555-555555555555', filename: 'reusable.png', mediaType: 'image/png', byteSize: 200, createdAt: '2026-09-17T10:00:00Z' },
      { assetKey: '66666666-6666-4666-8666-666666666666', filename: 'hidden.mp3', mediaType: 'audio/mpeg', byteSize: 200, createdAt: '2026-09-17T09:00:00Z' },
    ], limit: 20, offset: 0, total: 2 })
    render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson({ schemaVersion: 1, blocks: [original] }) } })
    await screen.findByRole('button', { name: 'Use reusable.png' })
    expect(screen.queryByRole('button', { name: 'Use hidden.mp3' })).toBeNull()
    await fireEvent.click(screen.getByRole('button', { name: 'Use reusable.png' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Save Lesson content' }))
    await screen.findByText('Lesson content saved.')
    expect(savedDocument().blocks[0]).toMatchObject({ payload: { asset: { assetKey: '55555555-5555-4555-8555-555555555555' } } })
    expect(JSON.stringify(savedDocument())).not.toContain('reusable.png')
  })

  it('attaches only a server-issued compatible Asset key and saves it through the normal content mutation', async () => {
    const original = { key: 'image', type: 'IMAGE' as const, payload: { asset: { assetKey: 'existing-asset' }, decorative: false, altText: 'Existing diagram' } }
    render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson({ schemaVersion: 1, blocks: [original] }) } })
    const file = new File(['image bytes'], 'replacement.png', { type: 'image/png' })
    await chooseFile('Image file', file)
    await fireEvent.click(screen.getByRole('button', { name: 'Upload replacement' }))
    await screen.findByText(/diagram.png uploaded and attached/)
    expect(uploadAsset).toHaveBeenCalledWith(draftID, file)
    await fireEvent.click(screen.getByRole('button', { name: 'Save Lesson content' }))
    await screen.findByText('Lesson content saved.')
    expect(savedDocument().blocks[0]).toMatchObject({ key: 'image', type: 'IMAGE', payload: { asset: { assetKey: '55555555-5555-4555-8555-555555555555' } } })
    expect(JSON.stringify(savedDocument())).not.toContain('replacement.png')
    expect(JSON.stringify(savedDocument())).not.toContain('image/png')
  })

  it('keeps an existing Asset key until replacement upload succeeds and handles safe upload errors', async () => {
    const original = { key: 'image', type: 'IMAGE' as const, payload: { asset: { assetKey: 'existing-asset' }, decorative: false, altText: 'Existing diagram' } }
    let resolveUpload: ((asset: unknown) => void) | undefined
    uploadAsset.mockImplementationOnce(() => new Promise((resolve) => { resolveUpload = resolve }))
    render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson({ schemaVersion: 1, blocks: [original] }) } })
    await chooseFile('Image file', new File(['image bytes'], 'replacement.png', { type: 'image/png' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Upload replacement' }))
    expect(screen.getByRole('button', { name: 'Uploading…' }).hasAttribute('disabled')).toBe(true)
    expect(screen.getByRole('button', { name: 'Save Lesson content' }).hasAttribute('disabled')).toBe(true)
    resolveUpload?.({ assetKey: '55555555-5555-4555-8555-555555555555', filename: 'replacement.png', mediaType: 'image/png', byteSize: 200, status: 'AVAILABLE' })
    await screen.findByText(/uploaded and attached/)

    uploadAsset.mockRejectedValueOnce(new APIProblemError(413, undefined, undefined))
    await chooseFile('Image file', new File(['image bytes'], 'too-large.png', { type: 'image/png' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Upload replacement' }))
    await screen.findByText(/selected file is too large/)
    await fireEvent.click(screen.getByRole('button', { name: 'Save Lesson content' }))
    await screen.findByText('Lesson content saved.')
    expect(savedDocument().blocks[0]).toMatchObject({ payload: { asset: { assetKey: '55555555-5555-4555-8555-555555555555' } } })
  })

  it('does not attach an incompatible detected type, while downloads accept generic binary content', async () => {
    const image = { key: 'image', type: 'IMAGE' as const, payload: { asset: { assetKey: 'existing-asset' }, decorative: false, altText: 'Existing diagram' } }
    render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson({ schemaVersion: 1, blocks: [image] }) } })
    uploadAsset.mockResolvedValueOnce({ assetKey: '55555555-5555-4555-8555-555555555555', filename: 'actually-text.png', mediaType: 'text/plain', byteSize: 200, status: 'AVAILABLE' })
    await chooseFile('Image file', new File(['text'], 'actually-text.png', { type: 'image/png' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Upload replacement' }))
    await screen.findByText(/not suitable for this image block/)
    expect(screen.getByRole('button', { name: 'Save Lesson content' }).hasAttribute('disabled')).toBe(true)

    await add('DOWNLOAD')
    uploadAsset.mockResolvedValueOnce({ assetKey: '66666666-6666-4666-8666-666666666666', filename: 'notes.bin', mediaType: 'application/octet-stream', byteSize: 200, status: 'AVAILABLE' })
    await chooseFile('Download file', new File(['binary'], 'notes.bin'))
    await fireEvent.click(screen.getByRole('button', { name: 'Upload file' }))
    await screen.findByText(/notes.bin uploaded and attached/)
    await fireEvent.click(screen.getByRole('button', { name: 'Save Lesson content' }))
    await screen.findByText('Lesson content saved.')
    expect(savedDocument().blocks[1]).toMatchObject({ type: 'DOWNLOAD', payload: { asset: { assetKey: '66666666-6666-4666-8666-666666666666' } } })
  })

  it('does not attach server-detected incompatible media to video or audio blocks', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson() } })
    await add('VIDEO')
    uploadAsset.mockResolvedValueOnce({ assetKey: '55555555-5555-4555-8555-555555555555', filename: 'sound.mp4', mediaType: 'audio/mpeg', byteSize: 200, status: 'AVAILABLE' })
    await chooseFile('Video file', new File(['sound'], 'sound.mp4', { type: 'video/mp4' }))
    await fireEvent.click(within(screen.getByLabelText('Video attachment')).getByRole('button', { name: 'Upload file' }))
    await screen.findByText(/not suitable for this video block/)

    await add('AUDIO')
    uploadAsset.mockResolvedValueOnce({ assetKey: '66666666-6666-4666-8666-666666666666', filename: 'image.mp3', mediaType: 'image/png', byteSize: 200, status: 'AVAILABLE' })
    await chooseFile('Audio file', new File(['image'], 'image.mp3', { type: 'audio/mpeg' }))
    await fireEvent.click(within(screen.getByLabelText('Audio attachment')).getByRole('button', { name: 'Upload file' }))
    await screen.findByText(/not suitable for this audio block/)
    await fireEvent.click(screen.getByRole('button', { name: 'Save Lesson content' }))
    await screen.findByText(/Upload an asset before saving/)
    expect(replaceContent).not.toHaveBeenCalled()
  })

  it('uses the existing hidden-resource behavior for upload denial', async () => {
    const original = { key: 'image', type: 'IMAGE' as const, payload: { asset: { assetKey: 'existing-asset' }, decorative: false, altText: 'Existing diagram' } }
    uploadAsset.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    const { emitted } = render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson({ schemaVersion: 1, blocks: [original] }) } })
    await chooseFile('Image file', new File(['image'], 'hidden.png', { type: 'image/png' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Upload replacement' }))
    await waitFor(() => expect(emitted().unavailable).toHaveLength(1))
    expect(screen.queryByText(/membership|capability|permission/i)).toBeNull()
  })

  it('discards a late upload response after the exact Lesson context changes', async () => {
    const original = { key: 'image', type: 'IMAGE' as const, payload: { asset: { assetKey: 'existing-asset' }, decorative: false, altText: 'Existing diagram' } }
    let resolveUpload: ((asset: unknown) => void) | undefined
    uploadAsset.mockImplementationOnce(() => new Promise((resolve) => { resolveUpload = resolve }))
    const { rerender } = render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson({ schemaVersion: 1, blocks: [original] }) } })
    await chooseFile('Image file', new File(['image'], 'late.png', { type: 'image/png' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Upload replacement' }))
    await rerender({ draftId: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa', lesson: { ...lesson({ schemaVersion: 1, blocks: [{ ...original, key: 'new-image' }] }), id: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb', draft_id: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa' } })
    resolveUpload?.({ assetKey: '55555555-5555-4555-8555-555555555555', filename: 'late.png', mediaType: 'image/png', byteSize: 200, status: 'AVAILABLE' })
    await waitFor(() => expect(screen.queryByText(/late.png uploaded and attached/)).toBeNull())
    expect(screen.getByRole('button', { name: 'Save Lesson content' }).hasAttribute('disabled')).toBe(true)
  })

  it('discards a late upload response when its stable block key no longer exists', async () => {
    const original = { key: 'image', type: 'IMAGE' as const, payload: { asset: { assetKey: 'existing-asset' }, decorative: false, altText: 'Existing diagram' } }
    let resolveUpload: ((asset: unknown) => void) | undefined
    uploadAsset.mockImplementationOnce(() => new Promise((resolve) => { resolveUpload = resolve }))
    render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson({ schemaVersion: 1, blocks: [original] }) } })
    await chooseFile('Image file', new File(['image'], 'removed.png', { type: 'image/png' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Upload replacement' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Remove block 1 image' }))
    resolveUpload?.({ assetKey: '55555555-5555-4555-8555-555555555555', filename: 'removed.png', mediaType: 'image/png', byteSize: 200, status: 'AVAILABLE' })
    await waitFor(() => expect(screen.queryByText(/removed.png uploaded and attached/)).toBeNull())
    await fireEvent.click(screen.getByRole('button', { name: 'Save Lesson content' }))
    await screen.findByText('Lesson content saved.')
    expect(savedDocument().blocks).toEqual([])
  })

  it('keeps operational upload errors sanitized', async () => {
    const original = { key: 'image', type: 'IMAGE' as const, payload: { asset: { assetKey: 'existing-asset' }, decorative: false, altText: 'Existing diagram' } }
    uploadAsset.mockRejectedValueOnce(new APIProblemError(500, undefined, undefined))
    render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson({ schemaVersion: 1, blocks: [original] }) } })
    await chooseFile('Image file', new File(['image'], 'failed.png', { type: 'image/png' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Upload replacement' }))
    await screen.findByRole('alert')
    expect(screen.getByRole('alert').textContent).toBe('We couldn’t upload this file right now. Please try again.')
    expect(screen.getByRole('button', { name: 'Save Lesson content' }).hasAttribute('disabled')).toBe(true)
  })

  it('clears semantic dirty state when edits or movements are reverted', async () => {
    const content: AuthoringLessonContent = { schemaVersion: 1, blocks: [{ key: 'quote', type: 'QUOTE', payload: { text: 'Original' } }, { key: 'divider', type: 'DIVIDER', payload: {} }] }
    render(AuthoringLessonContentEditor, { props: { draftId: draftID, lesson: lesson(content) } })
    const input = screen.getByRole('textbox', { name: 'Quote text' })
    await fireEvent.update(input, 'Changed'); await fireEvent.update(input, 'Original')
    await fireEvent.click(screen.getByRole('button', { name: 'Move block 1 quote down' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Move block 2 quote up' }))
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
