import type { JSONContent } from '@tiptap/vue-3'
import type { RichText, RichTextInline, RichTextMark } from '../lesson/content'
import { safePublishedURL } from '../lesson/content'

const markOrder: RichTextMark['type'][] = ['strong', 'emphasis', 'inline_code', 'link']

export function richTextToEditorDocument(content: RichText): JSONContent {
  return {
    type: 'doc',
    content: content.nodes.map((node) => node.type === 'paragraph'
      ? { type: 'paragraph', content: inlinesToEditor(node.content) }
      : {
          type: node.type === 'bullet_list' ? 'bulletList' : 'orderedList',
          content: node.items.map((item) => ({ type: 'listItem', content: [{ type: 'paragraph', content: inlinesToEditor(item) }] })),
        }),
  }
}

export function editorDocumentToRichText(document: JSONContent): RichText {
  const nodes: RichText['nodes'] = []
  for (const node of document.content ?? []) {
    if (node.type === 'paragraph') {
      const content = editorInlines(node.content)
      if (content.length) nodes.push({ type: 'paragraph', content })
      continue
    }
    if (node.type !== 'bulletList' && node.type !== 'orderedList') continue
    const items: RichTextInline[][] = []
    for (const item of node.content ?? []) collectListItem(item, items)
    if (items.length) nodes.push({ type: node.type === 'bulletList' ? 'bullet_list' : 'ordered_list', items })
  }
  return { nodes: nodes.length ? nodes : [{ type: 'paragraph', content: [] }] }
}

export function richTextPlainText(content: RichText): string {
  return content.nodes.flatMap((node) => node.type === 'paragraph' ? [inlineText(node.content)] : node.items.map(inlineText)).join(' ').replace(/\s+/g, ' ').trim()
}

function inlinesToEditor(inlines: RichTextInline[]): JSONContent[] {
  return inlines.map((inline) => inline.type === 'hard_break'
    ? { type: 'hardBreak' }
    : {
        type: 'text', text: inline.text,
        ...((inline.marks ?? []).length ? { marks: (inline.marks ?? []).map((mark) => mark.type === 'link'
          ? { type: 'link', attrs: { href: mark.href } }
          : { type: mark.type === 'emphasis' ? 'italic' : mark.type === 'strong' ? 'bold' : 'code' }) } : {}),
      })
}

function editorInlines(content: JSONContent[] | undefined): RichTextInline[] {
  const result: RichTextInline[] = []
  for (const inline of content ?? []) {
    if (inline.type === 'hardBreak') { result.push({ type: 'hard_break' }); continue }
    if (inline.type !== 'text' || !inline.text) continue
    const marks = canonicalMarks(inline.marks)
    const previous = result.at(-1)
    if (previous?.type === 'text' && sameMarks(previous.marks, marks)) previous.text += inline.text
    else result.push({ type: 'text', text: inline.text, marks })
  }
  return result
}

function collectListItem(item: JSONContent, result: RichTextInline[][]) {
  if (item.type !== 'listItem') return
  let current: RichTextInline[] = []
  for (const child of item.content ?? []) {
    if (child.type === 'paragraph') {
      const paragraph = editorInlines(child.content)
      if (current.length && paragraph.length) current.push({ type: 'hard_break' })
      current.push(...paragraph)
    } else if (child.type === 'bulletList' || child.type === 'orderedList') {
      if (current.length) { result.push(current); current = [] }
      for (const nested of child.content ?? []) collectListItem(nested, result)
    }
  }
  if (current.length) result.push(current)
}

function canonicalMarks(marks: JSONContent['marks']): RichTextMark[] {
  const found = new Map<RichTextMark['type'], RichTextMark>()
  for (const mark of marks ?? []) {
    if (mark.type === 'bold') found.set('strong', { type: 'strong' })
    else if (mark.type === 'italic') found.set('emphasis', { type: 'emphasis' })
    else if (mark.type === 'code') found.set('inline_code', { type: 'inline_code' })
    else if (mark.type === 'link') {
      const href = typeof mark.attrs?.href === 'string' ? safePublishedURL(mark.attrs.href) : undefined
      if (href) found.set('link', { type: 'link', href })
    }
  }
  return markOrder.flatMap((type) => found.has(type) ? [found.get(type)!] : [])
}

function sameMarks(left: RichTextMark[], right: RichTextMark[]) { return JSON.stringify(left) === JSON.stringify(right) }
function inlineText(inlines: RichTextInline[]) { return inlines.map((inline) => inline.type === 'text' ? inline.text : '\n').join(' ') }
