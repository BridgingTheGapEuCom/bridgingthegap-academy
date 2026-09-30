import { describe, expect, it } from 'vitest'
import { editorDocumentToRichText, richTextPlainText, richTextToEditorDocument } from './richText'
import type { RichText } from '../lesson/content'

describe('rich-text authoring conversion', () => {
  it('round trips every supported semantic node and mark deterministically', () => {
    const content: RichText = { nodes: [
      { type: 'paragraph', content: [
        { type: 'text', text: 'Plain ', marks: [] },
        { type: 'text', text: 'bold', marks: [{ type: 'strong' }] },
        { type: 'text', text: ' italic', marks: [{ type: 'emphasis' }] },
        { type: 'text', text: ' code', marks: [{ type: 'inline_code' }] },
        { type: 'hard_break' },
        { type: 'text', text: 'reference', marks: [{ type: 'link', href: '/reference' }] },
      ] },
      { type: 'bullet_list', items: [[{ type: 'text', text: 'Bullet', marks: [] }]] },
      { type: 'ordered_list', items: [[{ type: 'text', text: 'First', marks: [] }], [{ type: 'text', text: 'Second', marks: [] }]] },
      { type: 'code_block', text: 'kubectl get pods\n  kubectl get services' },
    ] }
    expect(editorDocumentToRichText(richTextToEditorDocument(content))).toEqual(content)
    expect(richTextPlainText(content)).toContain('Plain bold italic code')
    expect(richTextPlainText(content)).toContain('kubectl get services')
  })

  it('preserves multiline code text and indentation from the editor document', () => {
    const converted = editorDocumentToRichText({ type: 'doc', content: [
      { type: 'paragraph', content: [{ type: 'text', text: 'Run this:' }] },
      { type: 'codeBlock', content: [{ type: 'text', text: 'first line\n  indented line' }] },
      { type: 'paragraph', content: [{ type: 'text', text: 'Then continue.' }] },
    ] })
    expect(converted.nodes[1]).toEqual({ type: 'code_block', text: 'first line\n  indented line' })
    expect(richTextToEditorDocument(converted).content?.[1]).toEqual({ type: 'codeBlock', content: [{ type: 'text', text: 'first line\n  indented line' }] })
  })

  it('normalizes unsupported editor nodes and strips dangerous or cosmetic marks', () => {
    const converted = editorDocumentToRichText({ type: 'doc', content: [
      { type: 'heading', attrs: { level: 1 }, content: [{ type: 'text', text: 'Not a text heading' }] },
      { type: 'paragraph', content: [
        { type: 'text', text: 'Safe text', marks: [
          { type: 'textStyle', attrs: { color: 'red' } },
          { type: 'link', attrs: { href: 'javascript:alert(1)', onclick: 'alert(1)' } },
          { type: 'bold' },
        ] },
      ] },
      { type: 'script', content: [{ type: 'text', text: 'bad' }] },
    ] })
    expect(converted).toEqual({ nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Safe text', marks: [{ type: 'strong' }] }] }] })
    expect(JSON.stringify(converted)).not.toContain('javascript:')
    expect(JSON.stringify(converted)).not.toContain('onclick')
    expect(JSON.stringify(converted)).not.toContain('color')
  })

  it('converts plain paragraph content to the canonical document without migration', () => {
    const legacyPlain: RichText = { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Existing plain text', marks: [] }] }] }
    expect(editorDocumentToRichText(richTextToEditorDocument(legacyPlain))).toEqual(legacyPlain)
  })
})
