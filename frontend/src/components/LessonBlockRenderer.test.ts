import { cleanup, render, screen } from '@testing-library/vue'
import { afterEach, describe, expect, it } from 'vitest'
import LessonBlockRenderer from './LessonBlockRenderer.vue'
import type { RenderableBlock } from '../lesson/content'
import type { PublishedAssessmentLearnerView } from '../courses/courses'

const richText = { nodes: [{ type: 'paragraph' as const, content: [{ type: 'text' as const, text: 'A safe paragraph', marks: [{ type: 'strong' as const }, { type: 'emphasis' as const }] }] }] }

function renderBlock(block: RenderableBlock) { return render(LessonBlockRenderer, { props: { block } }) }

const learnerAssessment: PublishedAssessmentLearnerView = {
  assessmentKey: '10000000-0000-4000-8000-000000000001',
  questions: [{ stableKey: 'question', type: 'SINGLE_CHOICE', prompt: 'Choose safely', position: 0, options: [{ stableKey: 'one', text: 'One', position: 0 }, { stableKey: 'two', text: 'Two', position: 1 }], leftItems: [], rightItems: [] }],
}

describe('LessonBlockRenderer', () => {
  afterEach(cleanup)

  it('renders constrained rich text, headings, and safe links as semantic DOM', () => {
    renderBlock({ key: 'text', type: 'TEXT', payload: { content: { nodes: [
      { type: 'paragraph', content: [
        { type: 'text', text: 'Bold', marks: [{ type: 'strong' }] },
        { type: 'text', text: ' italic', marks: [{ type: 'emphasis' }] },
        { type: 'text', text: ' code', marks: [{ type: 'inline_code' }] },
        { type: 'text', text: ' Reference', marks: [{ type: 'link', href: 'https://example.test' }] },
      ] },
      { type: 'bullet_list', items: [[{ type: 'text', text: 'Bullet', marks: [] }]] },
      { type: 'ordered_list', items: [[{ type: 'text', text: 'First', marks: [] }]] },
      { type: 'code_block', text: 'kubectl get pods\n  kubectl get services' },
    ] } } })
    const link = screen.getByRole('link', { name: 'Reference' })
    expect(link.getAttribute('href')).toBe('https://example.test')
    expect(link.getAttribute('rel')).toBe('noopener noreferrer')
    expect(document.querySelector('strong')?.textContent).toBe('Bold')
    expect(document.querySelector('em')?.textContent).toBe(' italic')
    expect(document.querySelector('code')?.textContent).toBe(' code')
    expect(document.querySelectorAll('ul li')).toHaveLength(1)
    expect(document.querySelectorAll('ol li')).toHaveLength(1)
    expect(document.querySelector('.lesson-rich-text__code')?.textContent).toBe('kubectl get pods\n  kubectl get services')
    expect(document.querySelector('.lesson-rich-text__code > code')).toBeTruthy()
    cleanup()
    renderBlock({ key: 'heading', type: 'HEADING', payload: { level: 2, content: [{ type: 'text', text: 'A heading', marks: [] }] } })
    expect(screen.getByRole('heading', { level: 2, name: 'A heading' })).toBeTruthy()
  })

  it('renders controlled semantic variants without executable or fabricated media URLs', () => {
    const cases: Array<[RenderableBlock, RegExp, string]> = [
      [{ key: 'image', type: 'IMAGE', payload: { asset: { assetKey: 'diagram' }, decorative: false, altText: 'Event flow' } }, /Image unavailable\./, 'img'],
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

  it('resolves media through the supplied rendering context and otherwise fails closed', () => {
    const block: RenderableBlock = { key: 'image', type: 'IMAGE', payload: { asset: { assetKey: '55555555-5555-4555-8555-555555555555' }, decorative: false, altText: 'Event flow' } }
    render(LessonBlockRenderer, { props: { block, assetContext: { kind: 'draft-preview', draftID: '11111111-1111-4111-8111-111111111111' } } })
    expect(screen.getByRole('img', { name: 'Event flow' }).getAttribute('src')).toBe('/api/authoring/drafts/11111111-1111-4111-8111-111111111111/assets/55555555-5555-4555-8555-555555555555/content')
    expect(document.body.innerHTML).not.toContain('storage')
    cleanup()
    renderBlock(block)
    expect(screen.getByText('Image unavailable.')).toBeTruthy()
    expect(document.querySelector('[src]')).toBeNull()
  })

  it('keeps published media on immutable Course delivery URLs', () => {
    render(LessonBlockRenderer, { props: { block: { key: 'download', type: 'DOWNLOAD', payload: { asset: { assetKey: 'asset-key' }, label: 'Worksheet' } }, assetContext: { kind: 'published', courseID: 'course-id', version: '1.2.3' } } })
    expect(screen.getByRole('link', { name: 'Worksheet' }).getAttribute('href')).toBe('/api/courses/by-id/course-id/versions/1.2.3/assets/asset-key?download=1')
  })

  it('uses authenticated Draft URLs for video, audio, and downloads in Preview', () => {
    const assetContext = { kind: 'draft-preview' as const, draftID: '11111111-1111-4111-8111-111111111111' }
    render(LessonBlockRenderer, { props: { block: { key: 'video', type: 'VIDEO', payload: { asset: { assetKey: 'video-asset' }, title: 'Walkthrough', transcript: 'Transcript', captionsAsset: { assetKey: 'captions-asset' } } }, assetContext } })
    expect(document.querySelector('video source')?.getAttribute('src')).toContain('/api/authoring/drafts/11111111-1111-4111-8111-111111111111/assets/video-asset/content')
    cleanup()
    render(LessonBlockRenderer, { props: { block: { key: 'audio', type: 'AUDIO', payload: { asset: { assetKey: 'audio-asset' }, title: 'Audio guide', transcript: 'Transcript' } }, assetContext } })
    expect(document.querySelector('audio source')?.getAttribute('src')).toContain('/api/authoring/drafts/11111111-1111-4111-8111-111111111111/assets/audio-asset/content')
    cleanup()
    render(LessonBlockRenderer, { props: { block: { key: 'download', type: 'DOWNLOAD', payload: { asset: { assetKey: 'file-asset' }, label: 'Worksheet' } }, assetContext } })
    expect(screen.getByRole('link', { name: 'Worksheet' }).getAttribute('href')).toBe('/api/authoring/drafts/11111111-1111-4111-8111-111111111111/assets/file-asset/content?download=1')
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

  it('renders a supplied immutable learner projection without exposing answer data', () => {
    render(LessonBlockRenderer, { props: { block: { key: 'check', type: 'KNOWLEDGE_CHECK', payload: { assessmentKey: learnerAssessment.assessmentKey } }, publishedAssessment: learnerAssessment } })
    expect(screen.getByRole('radio', { name: 'One' })).toBeTruthy()
    expect(screen.queryByText(/Interactive knowledge checks will be available/i)).toBeNull()
    expect(document.body.innerHTML).not.toMatch(/correctOption|correctPairs|answerKey/i)
  })
})
