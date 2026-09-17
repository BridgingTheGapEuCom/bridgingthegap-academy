import type { components } from '../api/generated'
import { decodeLessonContent, safePublishedURL } from '../lesson/content'
import type { AuthoringLessonContent } from './authoring'

export type CanonicalBlock = components['schemas']['LessonBlock']
export type EditableBlockType = 'TEXT' | 'HEADING' | 'IMAGE' | 'VIDEO' | 'AUDIO' | 'CODE' | 'QUOTE' | 'CALLOUT' | 'DOWNLOAD' | 'DIVIDER'
export const editableBlockTypes: { type: EditableBlockType; label: string }[] = [
  { type: 'TEXT', label: 'Text' }, { type: 'HEADING', label: 'Heading' },
  { type: 'IMAGE', label: 'Image' }, { type: 'VIDEO', label: 'Video' }, { type: 'AUDIO', label: 'Audio' },
  { type: 'CODE', label: 'Code' }, { type: 'QUOTE', label: 'Quote' },
  { type: 'CALLOUT', label: 'Callout' }, { type: 'DOWNLOAD', label: 'Download' }, { type: 'DIVIDER', label: 'Divider' },
]
// Mirrors the canonical Go content limits; no editor-only fields enter this document.
export const contentEditorLimits = { blocks: 200, bytes: 1 << 20, text: 50_000, codeBytes: 100_000 }
export function copyContent(content: AuthoringLessonContent): AuthoringLessonContent { return JSON.parse(JSON.stringify(content)) as AuthoringLessonContent }

// Object property order is incidental; array order (including blocks and marks) is semantic.
export function contentFingerprint(content: AuthoringLessonContent): string {
  return JSON.stringify(content, (_key, value: unknown) => {
    if (typeof value !== 'object' || value === null || Array.isArray(value)) return value
    const record = value as Record<string, unknown>
    return Object.fromEntries(Object.keys(record).sort().map((key) => [key, record[key]]))
  })
}

export function createContentBlock(type: EditableBlockType, existingKeys: string[]): CanonicalBlock {
  let key: string
  do { key = `block-${crypto.randomUUID()}` } while (existingKeys.includes(key))
  const content = { nodes: [{ type: 'paragraph' as const, content: [{ type: 'text' as const, text: 'New text' }] }] }
  switch (type) {
    case 'TEXT': return { key, type, payload: { content } }
    case 'HEADING': return { key, type, payload: { level: 2, content: [{ type: 'text', text: 'New heading' }] } }
    // Empty references are local editor state only. They cannot pass canonical
    // validation or be saved until a server-issued Asset key is attached.
    case 'IMAGE': return { key, type, payload: { asset: { assetKey: '' }, decorative: false, altText: '' } }
    case 'VIDEO': return { key, type, payload: { asset: { assetKey: '' }, title: 'New video', transcript: 'Add a transcript.', captionsAsset: { assetKey: '' } } }
    case 'AUDIO': return { key, type, payload: { asset: { assetKey: '' }, title: 'New audio', transcript: 'Add a transcript.' } }
    case 'CODE': return { key, type, payload: { code: '// Code example' } }
    case 'QUOTE': return { key, type, payload: { text: 'New quote' } }
    case 'CALLOUT': return { key, type, payload: { kind: 'NOTE', content } }
    case 'DOWNLOAD': return { key, type, payload: { asset: { assetKey: '' }, label: 'New download' } }
    case 'DIVIDER': return { key, type, payload: {} }
  }
}

// This is a basic usability/safety check, not a second canonical parser. Decoded
// rendering views are never converted back to persistence: the original typed
// canonical document stays intact, including blocks this editor cannot edit.
export function contentEditorError(content: AuthoringLessonContent): string | undefined {
  if (new TextEncoder().encode(JSON.stringify(content)).length > contentEditorLimits.bytes) return 'Lesson content must fit within 1 MiB. Shorten the content before saving.'
  const decoded = decodeLessonContent(content)
  if (decoded.kind === 'unsupported-schema') return 'This content format cannot be edited by this version of the Academy.'
  if (decoded.kind !== 'ready' || decoded.blocks.some((block) => block.type === 'UNSUPPORTED')) {
    if (content.blocks.some((block) => ('asset' in block.payload) && block.payload.asset.assetKey === '')) return 'Upload an asset before saving this media or download block.'
    if (content.blocks.some((block) => block.type === 'VIDEO' && block.payload.captionsAsset.assetKey === '')) return 'Upload captions before saving this video block.'
    return 'This content document cannot be edited safely. Check block keys and payloads.'
  }
  for (const block of content.blocks) {
    if (block.type === 'CODE' && (!block.payload.code || new TextEncoder().encode(block.payload.code).length > contentEditorLimits.codeBytes)) return 'Code must contain text and fit within 100,000 bytes.'
    if (block.type === 'QUOTE' && (!block.payload.text.trim() || new TextEncoder().encode(block.payload.text).length > contentEditorLimits.text)) return 'Enter quote text within the 50,000-byte limit.'
    if (block.type === 'QUOTE' && block.payload.sourceUrl && !safePublishedURL(block.payload.sourceUrl)) return 'Use a safe HTTPS or internal source URL.'
    const inlineGroups = block.type === 'HEADING' ? [block.payload.content]
      : block.type === 'TEXT' || block.type === 'CALLOUT' ? block.payload.content.nodes.flatMap((node) => node.type === 'paragraph' ? [node.content ?? []] : node.items ?? []) : []
    if (inlineGroups.length && (inlineGroups.some((items) => items.some((item) => item.type === 'text' && !item.text))
      || inlineGroups.reduce((count, items) => count + items.reduce((n, item) => n + Array.from(item.text ?? '').length, 0), 0) > contentEditorLimits.text)) return 'Each text run needs text, within the 50,000-character block limit.'
  }
}
