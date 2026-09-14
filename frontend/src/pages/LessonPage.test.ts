import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { APIProblemError } from '../api/client'

const getLessonMock = vi.hoisted(() => vi.fn())
const getCourseVersionMock = vi.hoisted(() => vi.fn())
vi.mock('../courses/courses', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../courses/courses')>()),
  getLesson: getLessonMock,
  getCourseVersion: getCourseVersionMock,
}))

import LessonPage from './LessonPage.vue'

const content = {
  schemaVersion: 1,
  blocks: [
    { key: 'intro', type: 'TEXT', payload: { content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Introduction', marks: [] }] }] } } },
    { key: 'code', type: 'CODE', payload: { code: 'fmt.Println("safe")', language: 'go', title: 'Example' } },
    { key: 'divider', type: 'DIVIDER', payload: {} },
  ],
}
const lesson = {
  course: { slug: 'event-driven-architecture' },
  version: { version: '1.10.0', status: 'DEPRECATED', title: 'Event-driven architecture' },
  module: { key: 'fundamentals', title: 'Fundamentals', position: 0 },
  lesson: { key: 'sync-vs-async', title: 'Synchronous and asynchronous', description: 'Compare approaches.', objectives: ['Explain the trade-off'], estimated_duration_minutes: 10, position: 1, recommended_prerequisite_keys: ['what-is-eai'], content },
}
const structure = {
  course: { slug: 'event-driven-architecture' },
  version: { version: '1.10.0', status: 'DEPRECATED', title: 'Event-driven architecture' },
  modules: [{ module: { key: 'fundamentals', title: 'Fundamentals', position: 0 }, lessons: [
    { key: 'what-is-eai', title: 'What is EAI?', description: '', objectives: [], position: 0, recommended_prerequisite_keys: [] },
    { key: 'sync-vs-async', title: 'Synchronous and asynchronous', description: '', objectives: [], position: 1, recommended_prerequisite_keys: ['what-is-eai'] },
    { key: 'event-flow', title: 'Event flow', description: '', objectives: [], position: 2, recommended_prerequisite_keys: [] },
  ] }],
}

async function renderPage(path = '/courses/event-driven-architecture/versions/1.10.0/lessons/sync-vs-async') {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/courses/:slug/versions/:version/lessons/:lessonKey', component: LessonPage },
    { path: '/courses', component: { template: '<main>Courses</main>' } },
    { path: '/courses/:slug', component: { template: '<main>Course</main>' } },
  ] })
  await router.push(path)
  await router.isReady()
  return { ...render(LessonPage, { global: { plugins: [router] } }), router }
}

describe('LessonPage', () => {
  beforeEach(() => { getLessonMock.mockReset(); getCourseVersionMock.mockReset(); getLessonMock.mockResolvedValue(lesson); getCourseVersionMock.mockResolvedValue(structure) })
  afterEach(cleanup)

  it('renders exact-version lesson metadata and all canonical blocks in continuous mode', async () => {
    await renderPage()
    await screen.findByRole('heading', { level: 1, name: 'Synchronous and asynchronous' })
    expect(screen.getByText('This lesson is from a deprecated course version.')).toBeTruthy()
    expect(screen.getByText('Estimated duration')).toBeTruthy()
    expect(screen.getByRole('heading', { level: 2, name: 'Lesson objectives' })).toBeTruthy()
    expect(screen.getByText('What is EAI?')).toBeTruthy()
    expect(screen.getByText('Introduction')).toBeTruthy()
    expect(screen.getByText('fmt.Println("safe")')).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Previous lesson' }).getAttribute('href')).toContain('/versions/1.10.0/')
    expect(screen.getByRole('link', { name: 'Next lesson' }).getAttribute('href')).toContain('/versions/1.10.0/')
    expect(document.body.textContent).not.toContain('Progress')
    expect(document.body.textContent).not.toContain('Enrollment')
  })

  it('switches to a single accessible Focus block and honors navigation boundaries', async () => {
    await renderPage()
    await screen.findByRole('heading', { level: 1, name: 'Synchronous and asynchronous' })
    await fireEvent.click(screen.getByLabelText('Focus'))
    expect(screen.getByText('Block 1 of 3')).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Previous block' }) as HTMLButtonElement).disabled).toBe(true)
    expect(screen.getByText('Introduction')).toBeTruthy()
    expect(screen.queryByText('fmt.Println("safe")')).toBeNull()
    await fireEvent.click(screen.getByRole('button', { name: 'Next block' }))
    expect(screen.getByText('Block 2 of 3')).toBeTruthy()
    expect(screen.getByText('fmt.Println("safe")')).toBeTruthy()
    await fireEvent.click(screen.getByLabelText('Continuous'))
    expect(screen.getByText('Introduction')).toBeTruthy()
    expect(screen.getByText('fmt.Println("safe")')).toBeTruthy()
  })

  it('separates not found, operational failure, and unsupported content safely', async () => {
    getLessonMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    await renderPage()
    expect(await screen.findByRole('heading', { level: 1, name: 'Lesson not found' })).toBeTruthy()

    cleanup()
    getLessonMock.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(lesson)
    await renderPage()
    await screen.findByRole('heading', { level: 1, name: 'Lesson unavailable' })
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Synchronous and asynchronous' })).toBeTruthy()

    cleanup()
    getLessonMock.mockResolvedValueOnce({ ...lesson, lesson: { ...lesson.lesson, content: { ...content, schemaVersion: 2 } } })
    await renderPage()
    expect(await screen.findByRole('heading', { level: 1, name: 'Lesson format unavailable' })).toBeTruthy()
  })

  it('does not retain stale lesson responses after route changes', async () => {
    let resolveOld: ((value: typeof lesson) => void) | undefined
    getLessonMock.mockImplementationOnce(() => new Promise<typeof lesson>((resolve) => { resolveOld = resolve }))
    getLessonMock.mockResolvedValueOnce({ ...lesson, lesson: { ...lesson.lesson, key: 'event-flow', title: 'Event flow' } })
    const { router } = await renderPage()
    await router.push('/courses/event-driven-architecture/versions/1.10.0/lessons/event-flow')
    await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: 'Event flow' })).toBeTruthy())
    resolveOld?.(lesson)
    await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: 'Event flow' })).toBeTruthy())
  })
})
