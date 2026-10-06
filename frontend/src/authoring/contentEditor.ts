import type { components } from '../api/generated'
import { decodeLessonContent, safePublishedURL } from '../lesson/content'
import type { AuthoringLessonContent } from './authoring'

export type CanonicalBlock = components['schemas']['LessonBlock']
export type EditableBlockType = 'TEXT' | 'HEADING' | 'IMAGE' | 'VIDEO' | 'AUDIO' | 'CODE' | 'QUOTE' | 'CALLOUT' | 'DOWNLOAD' | 'KNOWLEDGE_CHECK' | 'PLUGIN_WIDGET' | 'DIVIDER'
export const editableBlockTypes: { type: EditableBlockType }[] = [
  { type: 'TEXT' }, { type: 'HEADING' }, { type: 'IMAGE' }, { type: 'VIDEO' }, { type: 'AUDIO' },
  { type: 'CODE' }, { type: 'QUOTE' }, { type: 'CALLOUT' }, { type: 'DOWNLOAD' }, { type: 'KNOWLEDGE_CHECK' }, { type: 'PLUGIN_WIDGET' }, { type: 'DIVIDER' },
]
export type ContentEditorValidationCode =
  | 'content_too_large'
  | 'unsupported_schema'
  | 'asset_required'
  | 'video_captions_required'
  | 'unsafe_document'
  | 'widget_required'
  | 'widget_configuration_too_large'
  | 'code_invalid'
  | 'quote_invalid'
  | 'quote_url_invalid'
  | 'rich_text_invalid'
  | 'rich_text_run_invalid'
  | 'rich_text_code_invalid'

export interface ContentEditorValidationIssue {
  code: ContentEditorValidationCode
}

export interface ContentBlockInitialText {
  text: string
  heading: string
  videoTitle: string
  mediaTranscript: string
  code: string
  quote: string
  downloadLabel: string
}
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

export function createContentBlock(type: EditableBlockType, existingKeys: string[], initial: ContentBlockInitialText): CanonicalBlock {
  let key: string
  do { key = `block-${crypto.randomUUID()}` } while (existingKeys.includes(key))
  const content = { nodes: [{ type: 'paragraph' as const, content: [{ type: 'text' as const, text: initial.text }] }] }
  switch (type) {
    case 'TEXT': return { key, type, payload: { content } }
    case 'HEADING': return { key, type, payload: { level: 2, content: [{ type: 'text', text: initial.heading }] } }
    // Empty references are local editor state only. They cannot pass canonical
    // validation or be saved until a server-issued Asset key is attached.
    case 'IMAGE': return { key, type, payload: { asset: { assetKey: '' }, decorative: false, altText: '' } }
    case 'VIDEO': return { key, type, payload: { asset: { assetKey: '' }, title: initial.videoTitle, transcript: initial.mediaTranscript, captionsAsset: { assetKey: '' } } }
    case 'AUDIO': return { key, type, payload: { asset: { assetKey: '' }, title: initial.videoTitle, transcript: initial.mediaTranscript } }
    case 'CODE': return { key, type, payload: { code: initial.code } }
    case 'QUOTE': return { key, type, payload: { text: initial.quote } }
    case 'CALLOUT': return { key, type, payload: { kind: 'NOTE', content } }
    case 'DOWNLOAD': return { key, type, payload: { asset: { assetKey: '' }, label: initial.downloadLabel } }
    // A selected Assessment key is the sole canonical reference. It is saved
    // through the normal Lesson mutation and remains publication-unresolved.
    // A non-empty placeholder preserves the established migration-compatible
    // canonical shape until an author chooses a real Draft Assessment.
    case 'KNOWLEDGE_CHECK': return { key, type, payload: { assessmentKey: 'assessment-placeholder' } }
    case 'PLUGIN_WIDGET': return { key, type, payload: { pluginId: 'com.example.placeholder.widget', pluginVersion: '0.0.0', artifactDigest: '0'.repeat(64), widgetId: 'widget-placeholder', widgetType: 'COURSE_WIDGET', configuration: {} } }
    case 'DIVIDER': return { key, type, payload: {} }
  }
}

// This is a basic usability/safety check, not a second canonical parser. Decoded
// rendering views are never converted back to persistence: the original typed
// canonical document stays intact, including blocks this editor cannot edit.
export function contentEditorError(content: AuthoringLessonContent): ContentEditorValidationIssue | undefined {
  if (new TextEncoder().encode(JSON.stringify(content)).length > contentEditorLimits.bytes) return { code: 'content_too_large' }
  const decoded = decodeLessonContent(content)
  if (decoded.kind === 'unsupported-schema') return { code: 'unsupported_schema' }
  if (decoded.kind !== 'ready' || decoded.blocks.some((block) => block.type === 'UNSUPPORTED')) {
    if (content.blocks.some((block) => ('asset' in block.payload) && block.payload.asset.assetKey === '')) return { code: 'asset_required' }
    if (content.blocks.some((block) => block.type === 'VIDEO' && block.payload.captionsAsset.assetKey === '')) return { code: 'video_captions_required' }
    return { code: 'unsafe_document' }
  }
  for (const block of content.blocks) {
	if (block.type === 'PLUGIN_WIDGET' && block.payload.pluginId === 'com.example.placeholder.widget') return { code: 'widget_required' }
	if (block.type === 'PLUGIN_WIDGET' && new TextEncoder().encode(JSON.stringify(block.payload.configuration)).length > 16 * 1024) return { code: 'widget_configuration_too_large' }
    if (block.type === 'CODE' && (!block.payload.code || new TextEncoder().encode(block.payload.code).length > contentEditorLimits.codeBytes)) return { code: 'code_invalid' }
    if (block.type === 'QUOTE' && (!block.payload.text.trim() || new TextEncoder().encode(block.payload.text).length > contentEditorLimits.text)) return { code: 'quote_invalid' }
    if (block.type === 'QUOTE' && block.payload.sourceUrl && !safePublishedURL(block.payload.sourceUrl)) return { code: 'quote_url_invalid' }
    const inlineGroups = block.type === 'HEADING' ? [block.payload.content]
      : block.type === 'TEXT' || block.type === 'CALLOUT' ? block.payload.content.nodes.flatMap((node) => node.type === 'paragraph' ? [node.content ?? []] : node.type === 'code_block' ? [] : node.items ?? []) : []
    const codeText = block.type === 'TEXT' || block.type === 'CALLOUT'
      ? block.payload.content.nodes.filter((node) => node.type === 'code_block').map((node) => node.text ?? '') : []
    if ((block.type === 'HEADING' || block.type === 'TEXT' || block.type === 'CALLOUT')
      && ((!inlineGroups.length && !codeText.length) || inlineGroups.some((items) => !items.length)
        || (!inlineGroups.some((items) => items.some((item) => item.type === 'text' && item.text)) && !codeText.some((text) => text)))) return { code: 'rich_text_invalid' }
    if ((inlineGroups.length || codeText.length) && (inlineGroups.some((items) => items.some((item) => item.type === 'text' && !item.text))
      || inlineGroups.reduce((count, items) => count + items.reduce((n, item) => n + Array.from(item.text ?? '').length, 0), 0)
        + codeText.reduce((count, text) => count + Array.from(text).length, 0) > contentEditorLimits.text)) return { code: 'rich_text_run_invalid' }
    if (codeText.some((text) => text === '')) return { code: 'rich_text_code_invalid' }
  }
}
