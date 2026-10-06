import { cleanup, fireEvent, screen, within } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AuthoringLessonContent, AuthoringLessonDetail } from '../authoring/authoring'
import AuthoringLessonContentEditor from './AuthoringLessonContentEditor.vue'
import { renderWithI18n as render } from '../test/i18n'
import { pseudoLocalize } from '../i18n/pseudo'

const replaceContent = vi.hoisted(() => vi.fn())
const listCourseWidgets = vi.hoisted(() => vi.fn())
const listAssessments = vi.hoisted(() => vi.fn())
vi.mock('../authoring/authoring', async (original) => ({
  ...(await original<typeof import('../authoring/authoring')>()),
  replaceAuthoringLessonContent: replaceContent,
  listAuthoringCourseWidgets: listCourseWidgets,
  listAuthoringDraftAssessments: listAssessments,
}))
const draftId = '11111111-1111-4111-8111-111111111111'
const lessonId = '22222222-2222-4222-8222-222222222222'

function lesson(content: AuthoringLessonContent = { schemaVersion: 1, blocks: [] }): AuthoringLessonDetail {
  return { id: lessonId, draft_id: draftId, module_id: '33333333-3333-4333-8333-333333333333', stable_key: 'intro', title: 'Introduction', description: '', objectives: [], estimated_duration_minutes: null, position: 0, revision: 7, recommended_prerequisite_keys: [], content, created_at: '', updated_at: '' }
}
async function add(type: string) {
  await fireEvent.click(screen.getByRole('button', { name: '+ Add content' }))
  await fireEvent.click(screen.getByRole('button', { name: type }))
  if (type !== 'Divider') await fireEvent.click(await screen.findByRole('button', { name: /Done editing block/ }))
}
async function edit(index: number) { await fireEvent.click(screen.getByRole('button', { name: new RegExp(`Edit block ${index}`) })) }
async function replaceEditorHTML(html: string) {
  const editor = screen.getByRole('textbox', { name: /Block \d+ text/ })
  editor.innerHTML = html
  await fireEvent.input(editor)
}

describe('AuthoringLessonContentEditor', () => {
  beforeEach(() => {
    replaceContent.mockImplementation(async (...args: unknown[]) => {
    const request = args[2] as { content: AuthoringLessonContent } | undefined
    const content = request?.content ?? { schemaVersion: 1, blocks: [] }
    return { lesson: { ...lesson(content), revision: 8, draftRevision: 9 }, content }
    })
    listCourseWidgets.mockResolvedValue({ widgets: [] })
    listAssessments.mockResolvedValue({ items: [], limit: 20, offset: 0, total: 0 })
  })
  afterEach(cleanup)

  it('renders compact rows and mounts one WYSIWYG editor only after Edit', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks: [
      { key: 'text', type: 'TEXT', payload: { content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Coupling describes dependency.', marks: [{ type: 'strong' }] }] }] } } },
      { key: 'divider', type: 'DIVIDER', payload: {} },
    ] }) } })
    expect(screen.getByText('Coupling describes dependency.')).toBeTruthy()
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(screen.queryByRole('button', { name: /Edit block 2/ })).toBeNull()
    await edit(1)
    expect(screen.queryByRole('dialog', { name: /Edit text block/ })).toBeNull()
    expect(screen.getByRole('region', { name: 'Editing block 1, text' })).toBeTruthy()
    expect(screen.getByRole('textbox', { name: 'Block 1 text' }).querySelector('strong')?.textContent).toBe('Coupling describes dependency.')
    expect(screen.getByRole('toolbar', { name: 'Block 1 text formatting' })).toBeTruthy()
  })

  it('renders the add-content picker through the pseudo-locale', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson() } }, 'en-XA')
    await fireEvent.click(screen.getByRole('button', { name: '+ Add content' }))
    expect(screen.getByRole('dialog', { name: pseudoLocalize('Add content') })).toBeTruthy()
    expect(screen.getByText(pseudoLocalize('Add paragraphs of written content.'))).toBeTruthy()
  })

  it('applies an edit locally, cancels isolated edits, and saves the complete document', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson() } })
    await add('Text')
    await edit(1)
    await replaceEditorHTML('<p>Applied text</p>')
    await fireEvent.click(screen.getByRole('button', { name: 'Done editing block 1, text' }))
    expect(screen.getByText('Applied text')).toBeTruthy()
    await edit(1)
    await replaceEditorHTML('<p>Cancelled</p>')
    await fireEvent.click(screen.getByRole('button', { name: 'Cancel changes for block 1, text' }))
    expect(screen.getByText('Applied text')).toBeTruthy()
    expect(replaceContent).not.toHaveBeenCalled()
    await fireEvent.click(screen.getByRole('button', { name: 'Save content' }))
    expect((replaceContent.mock.calls[0]![2] as any).content.blocks[0].payload.content.nodes[0].content[0].text).toBe('Applied text')
  })

  it('shows the lesson-level save bar only for dirty content and discards back to the saved document', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson() } })
    expect(screen.queryByRole('status', { name: /unsaved content/i })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Discard changes' })).toBeNull()
    await add('Divider')
    expect(screen.getByText('Unsaved changes')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Discard changes' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Save content' })).toBeTruthy()
    await fireEvent.click(screen.getByRole('button', { name: 'Discard changes' }))
    expect(screen.getByText('No lesson content yet.')).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Discard changes' })).toBeNull()
    expect(replaceContent).not.toHaveBeenCalled()
  })

  it('preserves rich marks and lists in the full replacement save payload', async () => {
    const content: AuthoringLessonContent = { schemaVersion: 1, blocks: [{ key: 'rich', type: 'TEXT', payload: { content: { nodes: [
      { type: 'paragraph', content: [
        { type: 'text', text: 'Bold', marks: [{ type: 'strong' }] },
        { type: 'text', text: ' italic', marks: [{ type: 'emphasis' }] },
        { type: 'text', text: ' code', marks: [{ type: 'inline_code' }] },
        { type: 'text', text: ' link', marks: [{ type: 'link', href: 'https://example.test/read' }] },
      ] },
      { type: 'bullet_list', items: [[{ type: 'text', text: 'First', marks: [] }], [{ type: 'text', text: 'Second', marks: [] }]] },
      { type: 'ordered_list', items: [[{ type: 'text', text: 'One', marks: [] }]] },
      { type: 'code_block', text: 'kubectl get pods\n  kubectl get services' },
    ] } } }] }
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson(content) } })
    await edit(1)
    const editor = screen.getByRole('textbox', { name: 'Block 1 text' })
    expect(editor.querySelector('strong')).toBeTruthy()
    expect(editor.querySelector('em')).toBeTruthy()
    expect(editor.querySelector('code')).toBeTruthy()
    expect(editor.querySelector('a')?.getAttribute('href')).toBe('https://example.test/read')
    expect(editor.querySelectorAll('ul li')).toHaveLength(2)
    expect(editor.querySelector('ol')).toBeTruthy()
    await replaceEditorHTML(`${editor.innerHTML}<p>Changed</p>`)
    await fireEvent.click(screen.getByRole('button', { name: 'Done editing block 1, text' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Save content' }))
    const saved = (replaceContent.mock.calls[0]![2] as any).content.blocks[0].payload.content
    expect(saved.nodes[0].content.map((item: any) => item.marks)).toEqual([
      [{ type: 'strong' }], [{ type: 'emphasis' }], [{ type: 'inline_code' }], [{ type: 'link', href: 'https://example.test/read' }],
    ])
    expect(saved.nodes[1].type).toBe('bullet_list')
    expect(saved.nodes[2].type).toBe('ordered_list')
    expect(saved.nodes[3]).toEqual({ type: 'code_block', text: 'kubectl get pods\n  kubectl get services' })
  })

  it('keeps numbering and full replacement order after moving and removing blocks', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson() } })
    await add('Text'); await add('Heading')
    expect(screen.getAllByText(/0[12]/)).toHaveLength(2)
    await fireEvent.click(screen.getByRole('button', { name: 'Actions for block 2, heading' })); await fireEvent.click(screen.getByRole('menuitem', { name: 'Move up' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Actions for block 2, text' })); await fireEvent.click(screen.getByRole('menuitem', { name: 'Remove' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Save content' }))
    expect((replaceContent.mock.calls[0]![2] as any).content.blocks.map((block: any) => block.type)).toEqual(['HEADING'])
  })

  it('dismisses an open block actions menu when a pointer lands outside it', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks: [
      { key: 'text', type: 'TEXT', payload: { content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Text', marks: [] }] }] } } },
    ] }) } })
    await fireEvent.click(screen.getByRole('button', { name: 'Actions for block 1, text' }))
    expect(screen.getByRole('menu')).toBeTruthy()
    await fireEvent.pointerDown(document.body)
    expect(screen.queryByRole('menu')).toBeNull()
  })

  it('reorders locally from the dedicated drag handle, announces the move, and saves the complete order', async () => {
    const { container } = render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks: [
      { key: 'heading', type: 'HEADING', payload: { level: 2, content: [{ type: 'text', text: 'Heading', marks: [] }] } },
      { key: 'divider', type: 'DIVIDER', payload: {} },
      { key: 'text', type: 'TEXT', payload: { content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Text', marks: [] }] }] } } },
    ] }) } })
    const target = container.querySelector<HTMLElement>('[data-content-block-key="heading"]')!
    Object.defineProperty(target, 'getBoundingClientRect', { configurable: true, value: () => ({ top: 100, height: 100 }) })
    const originalElementFromPoint = globalThis.document.elementFromPoint
    Object.defineProperty(globalThis.document, 'elementFromPoint', { configurable: true, value: vi.fn(() => target) })
    try {
      const handle = screen.getByRole('button', { name: 'Reorder block 3, text' })
      await fireEvent.pointerDown(handle, { button: 0, pointerId: 1, clientX: 10, clientY: 200 })
      await fireEvent.pointerMove(globalThis.document, { pointerId: 1, clientX: 10, clientY: 110 })
      await fireEvent.pointerUp(globalThis.document, { pointerId: 1, clientX: 10, clientY: 110 })
      expect(screen.getByText('Text block moved to position 1 of 3.')).toBeTruthy()
      await new Promise((resolve) => setTimeout(resolve))
      expect(globalThis.document.activeElement).toBe(screen.getByRole('button', { name: 'Reorder block 1, text' }))
      expect(replaceContent).not.toHaveBeenCalled()
      await fireEvent.click(screen.getByRole('button', { name: 'Save content' }))
      expect((replaceContent.mock.calls[0]![2] as any).content.blocks.map((block: any) => block.type)).toEqual(['TEXT', 'HEADING', 'DIVIDER'])
    } finally {
      Object.defineProperty(globalThis.document, 'elementFromPoint', { configurable: true, value: originalElementFromPoint })
    }
  })

  it('commits an expanded editor locally before another block begins dragging', async () => {
    const { container } = render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks: [
      { key: 'text', type: 'TEXT', payload: { content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Before', marks: [] }] }] } } },
      { key: 'heading', type: 'HEADING', payload: { level: 2, content: [{ type: 'text', text: 'Heading', marks: [] }] } },
    ] }) } })
    await edit(1)
    await replaceEditorHTML('<p>Local text</p>')
    const target = container.querySelector<HTMLElement>('[data-content-block-key="text"]')!
    Object.defineProperty(target, 'getBoundingClientRect', { configurable: true, value: () => ({ top: 100, height: 100 }) })
    const originalElementFromPoint = globalThis.document.elementFromPoint
    Object.defineProperty(globalThis.document, 'elementFromPoint', { configurable: true, value: vi.fn(() => target) })
    try {
      await fireEvent.pointerDown(screen.getByRole('button', { name: 'Reorder block 2, heading' }), { button: 0, pointerId: 1, clientX: 10, clientY: 200 })
      await fireEvent.pointerMove(globalThis.document, { pointerId: 1, clientX: 10, clientY: 110 })
      await fireEvent.pointerUp(globalThis.document, { pointerId: 1, clientX: 10, clientY: 110 })
      expect(screen.queryByRole('textbox', { name: 'Block 1 text' })).toBeNull()
      expect(screen.getByText('Local text')).toBeTruthy()
      await fireEvent.click(screen.getByRole('button', { name: 'Save content' }))
      expect((replaceContent.mock.calls[0]![2] as any).content.blocks.map((block: any) => block.type)).toEqual(['HEADING', 'TEXT'])
      expect((replaceContent.mock.calls[0]![2] as any).content.blocks[1].payload.content.nodes[0].content[0].text).toBe('Local text')
    } finally {
      Object.defineProperty(globalThis.document, 'elementFromPoint', { configurable: true, value: originalElementFromPoint })
    }
  })

  it('keeps picker and preview dirty warning available', async () => {
    const { emitted } = render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson() } })
    await fireEvent.click(screen.getByRole('button', { name: 'Preview' })); expect(emitted().preview).toHaveLength(1)
    await add('Divider'); await fireEvent.click(screen.getByRole('button', { name: 'Preview' }))
    expect(screen.getByRole('dialog', { name: 'You have unsaved changes' })).toBeTruthy()
  })

  it('switches blocks by retaining the first block’s local changes and mounts only one editor', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks: [
      { key: 'first', type: 'TEXT', payload: { content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'First', marks: [] }] }] } } },
      { key: 'second', type: 'TEXT', payload: { content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Second', marks: [] }] }] } } },
    ] }) } })
    await edit(1)
    await replaceEditorHTML('<p>Changed first</p>')
    await edit(2)
    expect(screen.queryByRole('textbox', { name: 'Block 1 text' })).toBeNull()
    expect(screen.getByRole('textbox', { name: 'Block 2 text' })).toBeTruthy()
    expect(screen.getByText('Changed first')).toBeTruthy()
    expect(screen.getAllByRole('textbox')).toHaveLength(1)
  })

  it('saves the active inline editor value and keeps Preview’s dirty warning', async () => {
    const { emitted } = render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks: [
      { key: 'text', type: 'TEXT', payload: { content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Before', marks: [] }] }] } } },
    ] }) } })
    await edit(1)
    await replaceEditorHTML('<p>Latest active value</p>')
    await fireEvent.click(screen.getByRole('button', { name: 'Preview' }))
    expect(screen.getByRole('dialog', { name: 'You have unsaved changes' })).toBeTruthy()
    expect(screen.getByText('Latest active value')).toBeTruthy()
    expect(emitted().preview).toBeUndefined()
    await fireEvent.click(screen.getByRole('button', { name: 'Save content' }))
    expect((replaceContent.mock.calls[0]![2] as any).content.blocks[0].payload.content.nodes[0].content[0].text).toBe('Latest active value')
  })

  it('expands asset controls inline and updates the collapsed summary after Done', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks: [
      { key: 'image', type: 'IMAGE', payload: { asset: { assetKey: 'image-asset' }, altText: 'Original alternative text', decorative: false, caption: '' } },
    ] }) } })
    await edit(1)
    expect(screen.queryByRole('dialog', { name: /Edit image block/ })).toBeNull()
    const alt = screen.getByRole('textbox', { name: /Alternative text/ })
    await fireEvent.update(alt, 'Updated alternative text')
    await fireEvent.update(screen.getByRole('textbox', { name: /Caption/ }), 'Updated caption')
    await fireEvent.click(screen.getByRole('button', { name: 'Done editing block 1, image' }))
    expect(screen.getByText('Updated caption')).toBeTruthy()
    await edit(1)
    expect(screen.getByRole('textbox', { name: /Alternative text/ })).toHaveProperty('value', 'Updated alternative text')
    expect(screen.getByRole('textbox', { name: /Caption/ })).toHaveProperty('value', 'Updated caption')
  })

  it('keeps the Image decorative toggle interaction intact with shared text inputs', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks: [
      { key: 'image', type: 'IMAGE', payload: { asset: { assetKey: 'image-asset' }, altText: 'Alternative text', decorative: false, caption: '' } },
    ] }) } })
    await edit(1)
    await fireEvent.click(screen.getByRole('checkbox', { name: /Decorative image/ }))
    expect(screen.getByRole('textbox', { name: /Alternative text/ })).toHaveProperty('disabled', true)
    await fireEvent.click(screen.getByRole('button', { name: 'Done editing block 1, image' }))
    await edit(1)
    expect(screen.getByRole('checkbox', { name: /Decorative image/ })).toHaveProperty('checked', true)
  })

  it('renders Image controls and the asset picker through the pseudo-locale', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks: [
      { key: 'image', type: 'IMAGE', payload: { asset: { assetKey: 'image-asset' }, altText: 'Alternative text', decorative: false, caption: '' } },
    ] }) } }, 'en-XA')
    const editImage = document.getElementById('authoring-content-edit-image')
    if (!editImage) throw new Error('Image edit control was not rendered')
    await fireEvent.click(editImage)
    expect(screen.getByRole('textbox', { name: (name) => name.startsWith(pseudoLocalize('Alternative text')) })).toBeTruthy()
    expect(screen.getByText(pseudoLocalize('Optional caption displayed below the image.'))).toBeTruthy()
    await fireEvent.click(screen.getByRole('button', { name: pseudoLocalize('Change image') }))
    const picker = screen.getByRole('dialog', { name: pseudoLocalize('Change image') })
    expect(picker).toBeTruthy()
    expect(screen.getByText(pseudoLocalize('Choose an existing image asset or upload a new one.'))).toBeTruthy()
    expect(within(picker).getByRole('button', { name: pseudoLocalize('Cancel') })).toBeTruthy()
  })

  it('uses shared localized controls for Heading, Code, Quote, Callout, and Divider blocks', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks: [
      { key: 'heading', type: 'HEADING', payload: { level: 2, content: [{ type: 'text', text: 'Heading', marks: [] }] } },
      { key: 'code', type: 'CODE', payload: { code: 'const value = 1', language: 'ts', title: 'Example' } },
      { key: 'quote', type: 'QUOTE', payload: { text: 'Quoted words', attribution: 'Ada', sourceUrl: 'https://example.test' } },
      { key: 'callout', type: 'CALLOUT', payload: { kind: 'NOTE', title: 'Remember', content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Callout text', marks: [] }] }] } } },
      { key: 'divider', type: 'DIVIDER', payload: {} },
    ] }) } })
    await edit(1)
    await fireEvent.update(screen.getByRole('combobox', { name: 'Heading level' }), '3')
    await fireEvent.update(screen.getByRole('textbox', { name: 'Heading text 1' }), 'Updated heading')
    await edit(2)
    await fireEvent.update(screen.getByRole('textbox', { name: 'Code language' }), 'go')
    await edit(3)
    await fireEvent.update(screen.getByRole('textbox', { name: 'Quote attribution' }), 'Grace')
    await edit(4)
    await fireEvent.update(screen.getByRole('combobox', { name: 'Callout kind' }), 'TIP')
    await fireEvent.update(screen.getByRole('textbox', { name: 'Callout title' }), 'Try this')
    await fireEvent.click(screen.getByRole('button', { name: 'Save content' }))
    const blocks = (replaceContent.mock.calls[0]![2] as any).content.blocks
    expect(blocks[0].payload.level).toBe(3)
    expect(blocks[1].payload.language).toBe('go')
    expect(blocks[2].payload.attribution).toBe('Grace')
    expect(blocks[3].payload.kind).toBe('TIP')
  })

  it('localizes simple block labels and validation through the pseudo-locale', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks: [
      { key: 'heading', type: 'HEADING', payload: { level: 2, content: [{ type: 'text', text: 'Heading', marks: [] }] } },
      { key: 'code', type: 'CODE', payload: { code: '', language: '', title: '' } },
      { key: 'quote', type: 'QUOTE', payload: { text: 'Quote', attribution: '', sourceUrl: '' } },
      { key: 'callout', type: 'CALLOUT', payload: { kind: 'NOTE', title: '', content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Callout', marks: [] }] }] } } },
      { key: 'divider', type: 'DIVIDER', payload: {} },
    ] }) } }, 'en-XA')
    expect(screen.getByText(pseudoLocalize('Divider').toLocaleLowerCase())).toBeTruthy()
    await fireEvent.click(document.getElementById('authoring-content-edit-code')!)
    expect(screen.getByRole('textbox', { name: pseudoLocalize('Code') })).toBeTruthy()
    expect(screen.getByText(pseudoLocalize('Enter code to display.'))).toBeTruthy()
    await fireEvent.click(document.getElementById('authoring-content-edit-heading')!)
    expect(screen.getByRole('combobox', { name: pseudoLocalize('Heading level') })).toBeTruthy()
    await fireEvent.click(document.getElementById('authoring-content-edit-quote')!)
    expect(screen.getByRole('textbox', { name: pseudoLocalize('Quote text') })).toBeTruthy()
    await fireEvent.click(document.getElementById('authoring-content-edit-callout')!)
    expect(screen.getByRole('combobox', { name: pseudoLocalize('Callout kind') })).toBeTruthy()
  })

  it('updates media fields through shared controls and keeps their attachment actions localized', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks: [
      { key: 'video', type: 'VIDEO', payload: { asset: { assetKey: 'video-asset' }, captionsAsset: { assetKey: 'captions-asset' }, title: 'Video', transcript: 'Transcript' } },
      { key: 'audio', type: 'AUDIO', payload: { asset: { assetKey: 'audio-asset' }, title: 'Audio', transcript: 'Transcript' } },
      { key: 'download', type: 'DOWNLOAD', payload: { asset: { assetKey: 'download-asset' }, label: 'Download', description: 'Description' } },
    ] }) } })
    await edit(1)
    await fireEvent.update(screen.getByRole('textbox', { name: 'Video title' }), 'Updated video')
    expect(screen.getByRole('button', { name: 'Change Video' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Change Captions' })).toBeTruthy()
    await fireEvent.update(screen.getByRole('textbox', { name: 'Video title' }), '')
    expect(screen.getByText('Enter a video title.')).toBeTruthy()
    await fireEvent.update(screen.getByRole('textbox', { name: 'Video title' }), 'Updated video')
    await edit(2)
    await fireEvent.update(screen.getByRole('textbox', { name: 'Audio transcript' }), 'Updated audio transcript')
    await edit(3)
    await fireEvent.update(screen.getByRole('textbox', { name: 'Download label' }), 'Updated download')
    await fireEvent.click(screen.getByRole('button', { name: 'Save content' }))
    const blocks = (replaceContent.mock.calls[0]![2] as any).content.blocks
    expect(blocks[0].payload.title).toBe('Updated video')
    expect(blocks[1].payload.transcript).toBe('Updated audio transcript')
    expect(blocks[2].payload.label).toBe('Updated download')
  })

  it('localizes media editors and attachment actions through the pseudo-locale', async () => {
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks: [
      { key: 'video', type: 'VIDEO', payload: { asset: { assetKey: 'video-asset' }, captionsAsset: { assetKey: 'captions-asset' }, title: 'Video', transcript: 'Transcript' } },
    ] }) } }, 'en-XA')
    await fireEvent.click(document.getElementById('authoring-content-edit-video')!)
    expect(screen.getByRole('textbox', { name: pseudoLocalize('Video title') })).toBeTruthy()
    expect(screen.getAllByRole('button', { name: /Çĥàñĝë/ })).toHaveLength(2)
  })

  it('localizes the widget authoring shell while preserving schema-provided labels', async () => {
    const courseWidgets = [{ pluginId: 'com.example.widget', pluginName: 'Example', pluginVersion: '1.0.0', artifactDigest: 'a'.repeat(64), widgetId: 'overview', widgetName: 'Overview widget', description: 'Domain description', configuration: { fields: [{ key: 'title', type: 'TEXT' as const, label: 'Plugin title', required: false }] } }]
    listCourseWidgets.mockResolvedValue({ widgets: courseWidgets })
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks: [{ key: 'widget', type: 'PLUGIN_WIDGET', payload: { pluginId: 'com.example.widget', pluginVersion: '1.0.0', artifactDigest: 'a'.repeat(64), widgetId: 'overview', widgetType: 'COURSE_WIDGET', configuration: {} } }] }) } }, 'en-XA')
    await fireEvent.click(document.getElementById('authoring-content-edit-widget')!)
    expect(screen.getByRole('combobox', { name: pseudoLocalize('Course widget') })).toBeTruthy()
    expect(screen.getByText(pseudoLocalize('Widget configuration is data only. This release is pinned when the Course is published.'))).toBeTruthy()
    expect(screen.getByRole('textbox', { name: 'Plugin title' })).toBeTruthy()
  })

  it('localizes knowledge-check counts and locale-formatted context through the pseudo-locale', async () => {
    listAssessments.mockResolvedValue({ items: [{ assessmentKey: '33333333-3333-4333-8333-333333333333', title: 'Terminology', questionCount: 2, revision: 1, updatedAt: '2026-10-06T10:00:00Z' }], limit: 20, offset: 0, total: 1 })
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks: [{ key: 'assessment', type: 'KNOWLEDGE_CHECK', payload: { assessmentKey: '33333333-3333-4333-8333-333333333333' } }] }) } }, 'en-XA')
    await fireEvent.click(document.querySelector<HTMLButtonElement>('[id^="authoring-content-edit-"]')!)
    expect(await screen.findByRole('list', { name: pseudoLocalize('Available assessments') })).toBeTruthy()
    expect(screen.getByRole('button', { name: /Üšë Terminology/ })).toBeTruthy()
    expect(screen.getByText(/2 ɋüëšţïöñš/)).toBeTruthy()
  })

  it('does not eagerly mount editors for a large lesson', async () => {
    const blocks: AuthoringLessonContent['blocks'] = [
      { key: 'text-0', type: 'TEXT', payload: { content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'First', marks: [] }] }] } } },
      ...Array.from({ length: 29 }, (_, index) => ({ key: `divider-${index}`, type: 'DIVIDER', payload: {} } as AuthoringLessonContent['blocks'][number])),
    ]
    render(AuthoringLessonContentEditor, { props: { draftId, lesson: lesson({ schemaVersion: 1, blocks }) } })
    expect(within(screen.getByRole('list', { name: 'Lesson content blocks' })).getAllByRole('listitem')).toHaveLength(30)
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(screen.queryByRole('dialog', { name: /Edit/ })).toBeNull()
    await edit(1)
    expect(screen.getAllByRole('textbox')).toHaveLength(1)
  })
})
