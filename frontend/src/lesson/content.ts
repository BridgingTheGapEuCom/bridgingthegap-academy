import type { components } from '../api/generated'

const stableKeyPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/
const maxBlocks = 200

export type RichText = { nodes: RichTextNode[] }
export type RichTextNode =
  | { type: 'paragraph'; content: RichTextInline[] }
  | { type: 'bullet_list' | 'ordered_list'; items: RichTextInline[][] }
export type RichTextInline = { type: 'text'; text: string; marks: RichTextMark[] } | { type: 'hard_break' }
export type RichTextMark = { type: 'emphasis' | 'strong' | 'inline_code' } | { type: 'link'; href: string }

type AssetReference = { assetKey: string }
export type RenderableBlock =
  | { key: string; type: 'TEXT'; payload: { content: RichText } }
  | { key: string; type: 'HEADING'; payload: { level: 2 | 3 | 4; content: RichTextInline[] } }
  | { key: string; type: 'IMAGE'; payload: { asset: AssetReference; decorative: boolean; altText?: string; caption?: string } }
  | { key: string; type: 'VIDEO'; payload: { asset: AssetReference; title: string; transcript?: string; transcriptAsset?: AssetReference; captionsAsset: AssetReference } }
  | { key: string; type: 'AUDIO'; payload: { asset: AssetReference; title: string; transcript?: string; transcriptAsset?: AssetReference } }
  | { key: string; type: 'CODE'; payload: { code: string; language?: string; title?: string } }
  | { key: string; type: 'QUOTE'; payload: { text: string; attribution?: string; sourceUrl?: string } }
  | { key: string; type: 'CALLOUT'; payload: { kind: 'INFO' | 'NOTE' | 'WARNING' | 'TIP'; title?: string; content: RichText } }
  | { key: string; type: 'TABLE'; payload: { caption?: string; headers: string[]; rows: string[][] } }
  | { key: string; type: 'DOWNLOAD'; payload: { asset: AssetReference; label: string; description?: string } }
  | { key: string; type: 'KNOWLEDGE_CHECK'; payload: { assessmentKey: string } }
  | { key: string; type: 'DIVIDER'; payload: Record<string, never> }
  | { key: string; type: 'UNSUPPORTED' }

export type DecodedLessonContent =
  | { kind: 'ready'; schemaVersion: 1; blocks: RenderableBlock[] }
  | { kind: 'unsupported-schema' }
  | { kind: 'invalid-content' }

// Generated types describe the HTTP contract. This narrow decoded view is the
// rendering boundary: it validates untrusted runtime JSON again, keeps Vue out
// of arbitrary payloads, and deliberately has no HTML/editor-state escape hatch.
export function decodeLessonContent(value: components['schemas']['LessonContent'] | unknown): DecodedLessonContent {
  if (!isRecord(value) || value.schemaVersion !== 1) return { kind: 'unsupported-schema' }
  if (!Array.isArray(value.blocks) || value.blocks.length > maxBlocks) return { kind: 'invalid-content' }

  const keys = new Set<string>()
  const blocks: RenderableBlock[] = []
  for (const rawBlock of value.blocks) {
    if (!isRecord(rawBlock) || !isStableKey(rawBlock.key) || keys.has(rawBlock.key)) return { kind: 'invalid-content' }
    keys.add(rawBlock.key)
    blocks.push(decodeBlock(rawBlock))
  }
  return { kind: 'ready', schemaVersion: 1, blocks }
}

export function safePublishedURL(value: string | undefined): string | undefined {
  if (!value || value.length > 2_048) return undefined
  if (value.startsWith('/')) {
    try {
      return new URL(value, 'https://academy.invalid').origin === 'https://academy.invalid' ? value : undefined
    } catch {
      return undefined
    }
  }
  try {
    return new URL(value).protocol === 'https:' ? value : undefined
  } catch {
    return undefined
  }
}

function decodeBlock(block: Record<string, unknown>): RenderableBlock {
  const key = block.key as string
  const payload = isRecord(block.payload) ? block.payload : undefined
  if (!payload || !isString(block.type)) return unsupported(key)

  switch (block.type) {
    case 'TEXT': {
      const content = richText(payload.content)
      return content ? { key, type: 'TEXT', payload: { content } } : unsupported(key)
    }
    case 'HEADING': {
      const level = payload.level
      const content = inlineList(payload.content)
      return (level === 2 || level === 3 || level === 4) && content ? { key, type: 'HEADING', payload: { level, content } } : unsupported(key)
    }
    case 'IMAGE': {
      const asset = assetReference(payload.asset)
      const decorative = payload.decorative
      const altText = optionalString(payload.altText)
      const caption = optionalString(payload.caption)
      if (!asset || typeof decorative !== 'boolean' || (decorative && altText) || (!decorative && !altText)) return unsupported(key)
      return { key, type: 'IMAGE', payload: { asset, decorative, ...(altText ? { altText } : {}), ...(caption ? { caption } : {}) } }
    }
    case 'VIDEO': {
      const asset = assetReference(payload.asset)
      const captionsAsset = assetReference(payload.captionsAsset)
      const title = optionalString(payload.title)
      const transcript = optionalString(payload.transcript)
      const transcriptAsset = assetReference(payload.transcriptAsset)
      if (!asset || !captionsAsset || !title || (!transcript && !transcriptAsset)) return unsupported(key)
      return { key, type: 'VIDEO', payload: { asset, title, captionsAsset, ...(transcript ? { transcript } : {}), ...(transcriptAsset ? { transcriptAsset } : {}) } }
    }
    case 'AUDIO': {
      const asset = assetReference(payload.asset)
      const title = optionalString(payload.title)
      const transcript = optionalString(payload.transcript)
      const transcriptAsset = assetReference(payload.transcriptAsset)
      if (!asset || !title || (!transcript && !transcriptAsset)) return unsupported(key)
      return { key, type: 'AUDIO', payload: { asset, title, ...(transcript ? { transcript } : {}), ...(transcriptAsset ? { transcriptAsset } : {}) } }
    }
    case 'CODE': {
      const code = optionalString(payload.code)
      const language = optionalString(payload.language)
      const title = optionalString(payload.title)
      return code !== undefined ? { key, type: 'CODE', payload: { code, ...(language ? { language } : {}), ...(title ? { title } : {}) } } : unsupported(key)
    }
    case 'QUOTE': {
      const text = optionalString(payload.text)
      const attribution = optionalString(payload.attribution)
      const sourceUrl = safePublishedURL(optionalString(payload.sourceUrl))
      return text ? { key, type: 'QUOTE', payload: { text, ...(attribution ? { attribution } : {}), ...(sourceUrl ? { sourceUrl } : {}) } } : unsupported(key)
    }
    case 'CALLOUT': {
      const kind = payload.kind
      const title = optionalString(payload.title)
      const content = richText(payload.content)
      if ((kind !== 'INFO' && kind !== 'NOTE' && kind !== 'WARNING' && kind !== 'TIP') || !content) return unsupported(key)
      return { key, type: 'CALLOUT', payload: { kind, ...(title ? { title } : {}), content } }
    }
    case 'TABLE': {
      const caption = optionalString(payload.caption)
      const headers = stringList(payload.headers)
      const rows = stringMatrix(payload.rows)
      if (!headers?.length || !rows || rows.some((row) => row.length !== headers.length)) return unsupported(key)
      return { key, type: 'TABLE', payload: { headers, rows, ...(caption ? { caption } : {}) } }
    }
    case 'DOWNLOAD': {
      const asset = assetReference(payload.asset)
      const label = optionalString(payload.label)
      const description = optionalString(payload.description)
      return asset && label ? { key, type: 'DOWNLOAD', payload: { asset, label, ...(description ? { description } : {}) } } : unsupported(key)
    }
    case 'KNOWLEDGE_CHECK': {
      const assessmentKey = optionalString(payload.assessmentKey)
      return assessmentKey ? { key, type: 'KNOWLEDGE_CHECK', payload: { assessmentKey } } : unsupported(key)
    }
    case 'DIVIDER': return { key, type: 'DIVIDER', payload: {} }
    default: return unsupported(key)
  }
}

function richText(value: unknown): RichText | undefined {
  if (!isRecord(value) || !Array.isArray(value.nodes)) return undefined
  const nodes: RichTextNode[] = []
  for (const rawNode of value.nodes) {
    if (!isRecord(rawNode) || !isString(rawNode.type)) return undefined
    if (rawNode.type === 'paragraph') {
      const content = inlineList(rawNode.content)
      if (!content) return undefined
      nodes.push({ type: 'paragraph', content })
    } else if (rawNode.type === 'bullet_list' || rawNode.type === 'ordered_list') {
      if (!Array.isArray(rawNode.items)) return undefined
      const items = rawNode.items.map(inlineList)
      if (items.some((item) => !item)) return undefined
      nodes.push({ type: rawNode.type, items: items as RichTextInline[][] })
    } else return undefined
  }
  return { nodes }
}

function inlineList(value: unknown): RichTextInline[] | undefined {
  if (!Array.isArray(value)) return undefined
  const result: RichTextInline[] = []
  for (const rawInline of value) {
    if (!isRecord(rawInline) || !isString(rawInline.type)) return undefined
    if (rawInline.type === 'hard_break') result.push({ type: 'hard_break' })
    else if (rawInline.type === 'text' && isString(rawInline.text)) {
      const marks = marksList(rawInline.marks)
      if (!marks) return undefined
      result.push({ type: 'text', text: rawInline.text, marks })
    } else return undefined
  }
  return result
}

function marksList(value: unknown): RichTextMark[] | undefined {
  if (value === undefined) return []
  if (!Array.isArray(value)) return undefined
  const result: RichTextMark[] = []
  for (const rawMark of value) {
    if (!isRecord(rawMark) || !isString(rawMark.type)) return undefined
    if (rawMark.type === 'emphasis' || rawMark.type === 'strong' || rawMark.type === 'inline_code') result.push({ type: rawMark.type })
    else if (rawMark.type === 'link') {
      const href = safePublishedURL(optionalString(rawMark.href))
      if (!href) return undefined
      result.push({ type: 'link', href })
    } else return undefined
  }
  return result
}

function assetReference(value: unknown): AssetReference | undefined {
  return isRecord(value) && isString(value.assetKey) && value.assetKey.length >= 1 && value.assetKey.length <= 160 ? { assetKey: value.assetKey } : undefined
}
function stringList(value: unknown): string[] | undefined { return Array.isArray(value) && value.every(isString) ? value : undefined }
function stringMatrix(value: unknown): string[][] | undefined { return Array.isArray(value) && value.every((row) => stringList(row)) ? value as string[][] : undefined }
function optionalString(value: unknown): string | undefined { return isString(value) ? value : undefined }
function isString(value: unknown): value is string { return typeof value === 'string' }
function isStableKey(value: unknown): value is string { return isString(value) && value.length >= 3 && value.length <= 96 && stableKeyPattern.test(value) }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }
function unsupported(key: string): RenderableBlock { return { key, type: 'UNSUPPORTED' } }
