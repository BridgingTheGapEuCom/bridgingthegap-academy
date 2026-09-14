import { describe, expect, it } from 'vitest'
import { decodeLessonContent, safePublishedURL } from './content'

const textBlock = { key: 'intro', type: 'TEXT', payload: { content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Safe text', marks: [{ type: 'strong' }] }] }] } } }

describe('published lesson content decoder', () => {
  it('preserves canonical block order while rejecting unsupported schemas and duplicate keys', () => {
    const decoded = decodeLessonContent({ schemaVersion: 1, blocks: [textBlock, { key: 'divider', type: 'DIVIDER', payload: {} }] })
    expect(decoded.kind).toBe('ready')
    if (decoded.kind === 'ready') expect(decoded.blocks.map((block) => block.key)).toEqual(['intro', 'divider'])

    expect(decodeLessonContent({ schemaVersion: 2, blocks: [textBlock] }).kind).toBe('unsupported-schema')
    expect(decodeLessonContent({ schemaVersion: 1, blocks: [] })).toEqual({ kind: 'ready', schemaVersion: 1, blocks: [] })
    expect(decodeLessonContent({ schemaVersion: 1, blocks: [textBlock, textBlock] }).kind).toBe('invalid-content')
  })

  it('fails individual unknown or malformed blocks closed without rendering their payload', () => {
    const decoded = decodeLessonContent({ schemaVersion: 1, blocks: [{ key: 'unknown', type: 'SCRIPT', payload: { source: '<script>' } }] })
    expect(decoded).toMatchObject({ kind: 'ready', blocks: [{ key: 'unknown', type: 'UNSUPPORTED' }] })
    expect(decodeLessonContent({ schemaVersion: 1, blocks: [{ key: 'image', type: 'IMAGE', payload: { asset: { assetKey: 'diagram' }, decorative: false } }] })).toMatchObject({ kind: 'ready', blocks: [{ type: 'UNSUPPORTED' }] })
  })

  it('allows only root-relative and HTTPS published links', () => {
    expect(safePublishedURL('/courses/example')).toBe('/courses/example')
    expect(safePublishedURL('https://example.test/reference')).toBe('https://example.test/reference')
    expect(safePublishedURL('//example.test')).toBeUndefined()
    expect(safePublishedURL('/\\evil.example')).toBeUndefined()
    expect(safePublishedURL('javascript:alert(1)')).toBeUndefined()
  })
})
