import { cleanup, render, screen } from '@testing-library/vue'
import { afterEach, describe, expect, it } from 'vitest'
import LessonBlockRenderer from './LessonBlockRenderer.vue'
import type { RenderableBlock } from '../lesson/content'

const richText = { nodes: [{ type: 'paragraph' as const, content: [{ type: 'text' as const, text: 'A safe paragraph', marks: [{ type: 'strong' as const }, { type: 'emphasis' as const }] }] }] }

function renderBlock(block: RenderableBlock) { return render(LessonBlockRenderer, { props: { block } }) }

describe('LessonBlockRenderer', () => {
  afterEach(cleanup)

  it('renders constrained rich text, headings, and safe links as semantic DOM', () => {
    renderBlock({ key: 'text', type: 'TEXT', payload: { content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Reference', marks: [{ type: 'link', href: 'https://example.test' }] }] }, { type: 'ordered_list', items: [[{ type: 'text', text: 'First', marks: [] }]] }] } } })
    expect(screen.getByRole('link', { name: 'Reference' }).getAttribute('href')).toBe('https://example.test')
    expect(screen.getByRole('list')).toBeTruthy()
    cleanup()
    renderBlock({ key: 'heading', type: 'HEADING', payload: { level: 2, content: [{ type: 'text', text: 'A heading', marks: [] }] } })
    expect(screen.getByRole('heading', { level: 2, name: 'A heading' })).toBeTruthy()
  })

  it('renders controlled semantic variants without executable or fabricated media URLs', () => {
    const cases: Array<[RenderableBlock, RegExp, string]> = [
      [{ key: 'image', type: 'IMAGE', payload: { asset: { assetKey: 'diagram' }, decorative: false, altText: 'Event flow' } }, /Image asset unavailable/, 'img'],
      [{ key: 'video', type: 'VIDEO', payload: { asset: { assetKey: 'movie' }, title: 'Walkthrough', transcript: 'Transcript text', captionsAsset: { assetKey: 'captions' } } }, /Transcript text/, 'video'],
      [{ key: 'audio', type: 'AUDIO', payload: { asset: { assetKey: 'audio' }, title: 'Audio guide', transcript: 'Audio transcript' } }, /Audio transcript/, 'audio'],
      [{ key: 'code', type: 'CODE', payload: { code: 'line one\n  line two', language: 'go', title: 'Example' } }, /line one/, 'code'],
      [{ key: 'quote', type: 'QUOTE', payload: { text: 'Quoted words', attribution: 'Ada' } }, /Quoted words/, 'quote'],
      [{ key: 'callout', type: 'CALLOUT', payload: { kind: 'WARNING', title: 'Take care', content: richText } }, /Warning/, 'callout'],
      [{ key: 'download', type: 'DOWNLOAD', payload: { asset: { assetKey: 'worksheet' }, label: 'Worksheet' } }, /Download unavailable/, 'download'],
      [{ key: 'check', type: 'KNOWLEDGE_CHECK', payload: { assessmentKey: 'opaque-check' } }, /Interactive knowledge checks/, 'check'],
    ]
    for (const [block, expected] of cases) {
      renderBlock(block)
      expect(screen.getByText(expected)).toBeTruthy()
      expect(document.querySelector('[src]')).toBeNull()
      cleanup()
    }
  })

  it('renders quote, callout, table, divider, and unsupported content with clear semantics', () => {
    renderBlock({ key: 'quote', type: 'QUOTE', payload: { text: 'Quoted words', attribution: 'Ada' } })
    expect(document.querySelector('blockquote cite')?.textContent).toBe('Ada')
    cleanup()
    renderBlock({ key: 'table', type: 'TABLE', payload: { caption: 'Terms', headers: ['Term'], rows: [['Meaning']] } })
    expect(screen.getByRole('table')).toBeTruthy()
    expect(screen.getByRole('columnheader', { name: 'Term' })).toBeTruthy()
    expect(screen.getByRole('region', { name: 'Scrollable table: Terms' }).getAttribute('tabindex')).toBe('0')
    cleanup()
    renderBlock({ key: 'divider', type: 'DIVIDER', payload: {} })
    expect(document.querySelector('hr')).toBeTruthy()
    cleanup()
    renderBlock({ key: 'bad', type: 'UNSUPPORTED' })
    expect(screen.getByRole('alert').textContent).toContain('cannot be displayed safely')
  })
})
