import { describe, expect, it } from 'vitest'
import { contentEditorError, contentFingerprint, copyContent, createContentBlock, editableBlockTypes } from './contentEditor'
import type { AuthoringLessonContent } from './authoring'
import { deferredBlocks } from './contentEditor.fixtures'


describe('canonical content editor helpers', () => {
  it.each(editableBlockTypes)('creates valid $type defaults with unique stable keys independent of text', ({ type }) => {
    const first = createContentBlock(type, [])
    const second = createContentBlock(type, [first.key])
    expect(first.key).toMatch(/^block-[a-z0-9]+(?:-[a-z0-9]+)*$/)
    expect(first.key.length).toBeLessThanOrEqual(80)
    expect(second.key).not.toBe(first.key)
    expect(contentEditorError({ schemaVersion: 1, blocks: [first, second] })).toBeUndefined()
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

  it('rejects unsafe links, duplicate keys, invalid levels, and canonical size boundaries', () => {
    expect(contentEditorError({ schemaVersion: 1, blocks: [{ key: 'quote', type: 'QUOTE', payload: { text: 'Quote', sourceUrl: 'javascript:alert(1)' } }] })).toBeTruthy()
    expect(contentEditorError({ schemaVersion: 1, blocks: [{ key: 'heading', type: 'HEADING', payload: { level: 1, content: [{ type: 'text', text: 'Heading' }] } }] })).toBeTruthy()
    expect(contentEditorError({ schemaVersion: 1, blocks: [{ key: 'code', type: 'CODE', payload: { code: 'x'.repeat(100001) } }] })).toBeTruthy()
    const divider = { key: 'same', type: 'DIVIDER' as const, payload: {} }
    expect(contentEditorError({ schemaVersion: 1, blocks: [divider, divider] })).toBeTruthy()
    expect(contentEditorError({ schemaVersion: 1, blocks: Array.from({ length: 201 }, (_, n) => ({ ...divider, key: `block-${n}` })) })).toBeTruthy()
    expect(contentEditorError({ schemaVersion: 1, blocks: [{ key: 'quote', type: 'QUOTE', payload: { text: 'x'.repeat(1 << 20) } }] })).toContain('1 MiB')
  })
})
