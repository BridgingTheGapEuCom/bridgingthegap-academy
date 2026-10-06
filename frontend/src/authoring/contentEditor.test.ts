import { describe, expect, it } from 'vitest'
import { contentEditorError, contentFingerprint, copyContent, createContentBlock, editableBlockTypes, type ContentBlockInitialText } from './contentEditor'
import type { AuthoringLessonContent } from './authoring'
import { deferredBlocks } from './contentEditor.fixtures'

const initialText: ContentBlockInitialText = {
  text: 'New text', heading: 'New heading', videoTitle: 'New video', mediaTranscript: 'Add a transcript.', code: '// Code example', quote: 'New quote', downloadLabel: 'New download',
}


describe('canonical content editor helpers', () => {
  it.each(editableBlockTypes)('creates $type defaults with unique stable keys independent of text', ({ type }) => {
    const first = createContentBlock(type, [], initialText)
    const second = createContentBlock(type, [first.key], initialText)
    expect(first.key).toMatch(/^block-[a-z0-9]+(?:-[a-z0-9]+)*$/)
    expect(first.key.length).toBeLessThanOrEqual(80)
    expect(second.key).not.toBe(first.key)
    const error = contentEditorError({ schemaVersion: 1, blocks: [first, second] })
    if (type === 'IMAGE' || type === 'VIDEO' || type === 'AUDIO' || type === 'DOWNLOAD') expect(error?.code).toBe('asset_required')
    else if (type === 'PLUGIN_WIDGET') expect(error?.code).toBe('widget_required')
    else expect(error).toBeUndefined()
  })

  it('copies deferred canonical payloads without normalization or mutation', () => {
    const original: AuthoringLessonContent = { schemaVersion: 1, blocks: deferredBlocks }
    const copy = copyContent(original)
    expect(copy).toEqual(original)
    copy.blocks.reverse()
    expect(original.blocks[0]?.type).toBe('IMAGE')
    expect(contentEditorError(original)).toBeUndefined()
  })

  it('ignores incidental object order while retaining canonical array order', () => {
    const one: AuthoringLessonContent = { schemaVersion: 1, blocks: [{ key: 'first', type: 'DIVIDER', payload: {} }, { key: 'second', type: 'DIVIDER', payload: {} }] }
    const reorderedFields: AuthoringLessonContent = { blocks: [{ payload: {}, type: 'DIVIDER', key: 'first' }, one.blocks[1]!], schemaVersion: 1 }
    expect(contentFingerprint(one)).toBe(contentFingerprint(reorderedFields))
    expect(contentFingerprint(one)).not.toBe(contentFingerprint({ ...one, blocks: [...one.blocks].reverse() }))
  })

  it('returns stable validation codes rather than UI prose', () => {
    expect(contentEditorError({ schemaVersion: 1, blocks: [{ key: 'quote', type: 'QUOTE', payload: { text: 'Quote', sourceUrl: 'javascript:alert(1)' } }] })?.code).toBe('quote_url_invalid')
    expect(contentEditorError({ schemaVersion: 1, blocks: [{ key: 'heading', type: 'HEADING', payload: { level: 1, content: [{ type: 'text', text: 'Heading' }] } }] })?.code).toBe('unsafe_document')
    expect(contentEditorError({ schemaVersion: 1, blocks: [{ key: 'code', type: 'CODE', payload: { code: 'x'.repeat(100001) } }] })?.code).toBe('code_invalid')
    const divider = { key: 'same', type: 'DIVIDER' as const, payload: {} }
    expect(contentEditorError({ schemaVersion: 1, blocks: [divider, divider] })?.code).toBe('unsafe_document')
    expect(contentEditorError({ schemaVersion: 1, blocks: Array.from({ length: 201 }, (_, n) => ({ ...divider, key: `block-${n}` })) })?.code).toBe('unsafe_document')
    expect(contentEditorError({ schemaVersion: 1, blocks: [{ key: 'quote', type: 'QUOTE', payload: { text: 'x'.repeat(1 << 20) } }] })?.code).toBe('content_too_large')
  })
})
