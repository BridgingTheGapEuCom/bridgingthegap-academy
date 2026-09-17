import { cleanup, render, screen } from '@testing-library/vue'
import { afterEach, describe, expect, it } from 'vitest'
import type { PublishedCourseVersionDetail } from '../courses/courses'
import PublishedCourseReaderLesson from './PublishedCourseReaderLesson.vue'

type PublishedLesson = PublishedCourseVersionDetail['modules'][number]['lessons'][number]

function lesson(content: unknown): PublishedLesson {
  return {
    stableKey: 'published-lesson',
    title: 'Published lesson',
    description: '',
    objectives: [],
    estimatedDurationMinutes: null,
    position: 0,
    prerequisiteStableKeys: [],
    content,
  } as PublishedLesson
}

function renderLesson(content: unknown) {
  return render(PublishedCourseReaderLesson, { props: { lesson: lesson(content) } })
}

describe('PublishedCourseReaderLesson', () => {
  afterEach(cleanup)

  it('renders the immutable canonical block document through the shared safe renderer', () => {
    renderLesson({
      schemaVersion: 1,
      blocks: [
        { key: 'text', type: 'TEXT', payload: { content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Read ', marks: [] }, { type: 'text', text: 'carefully', marks: [{ type: 'strong' }] }, { type: 'hard_break' }, { type: 'text', text: 'Reference', marks: [{ type: 'link', href: 'https://example.test/reference' }] }] }] } } },
        { key: 'heading', type: 'HEADING', payload: { level: 3, content: [{ type: 'text', text: 'Details', marks: [] }] } },
        { key: 'code', type: 'CODE', payload: { title: 'Example', language: 'go', code: 'fmt.Println("inert")' } },
        { key: 'quote', type: 'QUOTE', payload: { text: 'Quoted material', attribution: 'Ada' } },
        { key: 'callout', type: 'CALLOUT', payload: { kind: 'WARNING', title: 'Take care', content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Controlled variant', marks: [] }] }] } } },
        { key: 'table', type: 'TABLE', payload: { caption: 'Terms', headers: ['Term'], rows: [['Meaning']] } },
        { key: 'image', type: 'IMAGE', payload: { asset: { assetKey: 'diagram' }, decorative: false, altText: 'Event flow' } },
        { key: 'video', type: 'VIDEO', payload: { asset: { assetKey: 'video' }, title: 'Walkthrough', captionsAsset: { assetKey: 'captions' }, transcript: 'Video transcript' } },
        { key: 'audio', type: 'AUDIO', payload: { asset: { assetKey: 'audio' }, title: 'Audio guide', transcript: 'Audio transcript' } },
        { key: 'download', type: 'DOWNLOAD', payload: { asset: { assetKey: 'worksheet' }, label: 'Worksheet' } },
        { key: 'check', type: 'KNOWLEDGE_CHECK', payload: { assessmentKey: 'check-one' } },
        { key: 'divider', type: 'DIVIDER', payload: {} },
      ],
    })

    expect(screen.getByRole('heading', { level: 3, name: 'Details' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Reference' }).getAttribute('href')).toBe('https://example.test/reference')
    expect(document.querySelector('strong')?.textContent).toBe('carefully')
    expect(document.querySelector('pre code')?.textContent).toContain('fmt.Println')
    expect(document.querySelector('blockquote')?.textContent).toContain('Quoted material')
    expect(screen.getByRole('note', { name: 'Warning' })).toBeTruthy()
    expect(screen.getByRole('table')).toBeTruthy()
    expect(screen.getByRole('columnheader', { name: 'Term' })).toBeTruthy()
    expect(document.querySelector('.lesson-table-scroll')).toBeTruthy()
    expect(screen.getByRole('img', { name: 'Event flow' }).textContent).toContain('Image asset unavailable')
    expect(screen.getByText('Video transcript')).toBeTruthy()
    expect(screen.getByText('Audio transcript')).toBeTruthy()
    expect(screen.getByText('Download unavailable.')).toBeTruthy()
    expect(screen.getByText('Interactive knowledge checks will be available when assessments are enabled.')).toBeTruthy()
    expect(document.querySelector('hr')).toBeTruthy()
    expect(document.querySelector('img, video, audio, [src]')).toBeNull()
    expect(screen.queryByRole('link', { name: 'Worksheet' })).toBeNull()
  })

  it('keeps empty, unknown, and unsupported content safe and readable', () => {
    renderLesson({ schemaVersion: 1, blocks: [] })
    expect(screen.getByText('This lesson has no published content yet.')).toBeTruthy()

    cleanup()
    renderLesson({ schemaVersion: 1, blocks: [{ key: 'unknown', type: 'SCRIPT', payload: { source: '<script>window.executed = true</script>' } }] })
    expect(screen.getByRole('alert').textContent).toContain('cannot be displayed safely')
    expect(document.body.textContent).not.toContain('window.executed = true')
    expect(document.querySelector('script')).toBeNull()

    cleanup()
    renderLesson({ schemaVersion: 2, blocks: [] })
    expect(screen.getByText('This lesson uses a content format this version of the Academy cannot display.')).toBeTruthy()
  })

  it('uses exact immutable CourseVersion asset URLs with native accessible media', () => {
    const image = '50000000-0000-4000-8000-000000000001'
    const video = '50000000-0000-4000-8000-000000000002'
    const captions = '50000000-0000-4000-8000-000000000003'
    const audio = '50000000-0000-4000-8000-000000000004'
    const download = '50000000-0000-4000-8000-000000000005'
    const courseID = '10000000-0000-4000-8000-000000000001'
    render(PublishedCourseReaderLesson, {
      props: {
        lesson: lesson({
          schemaVersion: 1,
          blocks: [
            { key: 'image', type: 'IMAGE', payload: { asset: { assetKey: image }, decorative: false, altText: 'Published architecture' } },
            { key: 'video', type: 'VIDEO', payload: { asset: { assetKey: video }, title: 'Walkthrough', captionsAsset: { assetKey: captions }, transcript: 'Accessible transcript' } },
            { key: 'audio', type: 'AUDIO', payload: { asset: { assetKey: audio }, title: 'Audio guide', transcript: 'Audio transcript' } },
            { key: 'download', type: 'DOWNLOAD', payload: { asset: { assetKey: download }, label: 'Reference worksheet' } },
          ],
        }),
        courseId: courseID,
        version: '1.2.3',
      },
    })

    const base = `/api/courses/by-id/${courseID}/versions/1.2.3/assets/`
    expect(screen.getByRole('img', { name: 'Published architecture' }).getAttribute('src')).toBe(base + image)
    expect(document.querySelector('video')?.getAttribute('autoplay')).toBeNull()
    expect(document.querySelector('video source')?.getAttribute('src')).toBe(base + video)
    expect(document.querySelector('audio')?.getAttribute('autoplay')).toBeNull()
    expect(document.querySelector('audio source')?.getAttribute('src')).toBe(base + audio)
    expect(screen.getByRole('link', { name: 'Download captions' }).getAttribute('href')).toBe(base + captions + '?download=1')
    expect(screen.getByRole('link', { name: 'Reference worksheet' }).getAttribute('href')).toBe(base + download + '?download=1')
    expect(document.body.textContent).not.toContain('StorageObjectID')
  })
})
