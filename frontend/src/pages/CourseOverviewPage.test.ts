import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { APIProblemError } from '../api/client'

const getCourseMock = vi.hoisted(() => vi.fn())
vi.mock('../courses/courses', async (importOriginal) => ({ ...(await importOriginal<typeof import('../courses/courses')>()), getCourse: getCourseMock }))

import CourseOverviewPage from './CourseOverviewPage.vue'

const course = {
  course: { slug: 'event-driven-architecture' },
  version: {
    version: '1.10.0', status: 'PUBLISHED', title: 'Event-driven architecture', description: 'A practical course.', source_language: 'en-GB',
    objectives: ['Explain event ownership', 'Describe asynchronous boundaries'],
    contributors: [{ display_name: 'Ada Author', role: 'AUTHOR', order: 0 }, { display_name: 'Mina Maintainer', role: 'MAINTAINER', order: 1 }],
    license: { kind: 'STANDARD', identifier: 'CC-BY-4.0', display_name: 'Creative Commons Attribution 4.0', url: 'https://creativecommons.org/licenses/by/4.0/' },
  },
  modules: [
    { module: { key: 'fundamentals', title: 'Fundamentals', description: 'Core concepts.', position: 0 }, lessons: [
      { key: 'what-is-eai', title: 'What is EAI?', description: 'A starting point.', objectives: [], position: 0, estimated_duration_minutes: 10, recommended_prerequisite_keys: [] },
      { key: 'sync-vs-async', title: 'Synchronous and asynchronous', description: 'Compare approaches.', objectives: [], position: 1, estimated_duration_minutes: 75, recommended_prerequisite_keys: ['what-is-eai'] },
    ] },
    { module: { key: 'advanced', title: 'Advanced', position: 1 }, lessons: [] },
  ],
}

async function renderPage(slug = 'event-driven-architecture') {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/courses/:slug', component: CourseOverviewPage }, { path: '/courses', component: { template: '<main>Courses</main>' } }, { path: '/courses/:slug/versions/:version/lessons/:lessonKey', component: { template: '<main>Lesson</main>' } }] })
  await router.push(`/courses/${slug}`)
  await router.isReady()
  return { ...render(CourseOverviewPage, { global: { plugins: [router] } }), router }
}

describe('CourseOverviewPage', () => {
  beforeEach(() => getCourseMock.mockReset())
  afterEach(cleanup)

  it('renders ordered metadata, outline, provenance links, and advisory prerequisites', async () => {
    getCourseMock.mockResolvedValue(course)
    await renderPage()
    await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: 'Event-driven architecture' })).toBeTruthy())
    expect(screen.getByRole('heading', { level: 2, name: 'What you’ll learn' })).toBeTruthy()
    expect(screen.getAllByRole('list')[0].textContent).toContain('Explain event ownership')
    expect(screen.getByText(/Ada Author/)).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Creative Commons Attribution 4.0' }).getAttribute('href')).toBe('https://creativecommons.org/licenses/by/4.0/')
    expect(screen.getByText('Estimated duration: 1 hr 15 min')).toBeTruthy()
    expect(screen.getByText('Recommended before this lesson: What is EAI?')).toBeTruthy()
    const lessonLink = screen.getByRole('link', { name: 'Synchronous and asynchronous' })
    expect(lessonLink.getAttribute('href')).toBe('/courses/event-driven-architecture/versions/1.10.0/lessons/sync-vs-async')
    expect(document.body.textContent).not.toContain('Progress')
    expect(document.body.textContent).not.toContain('Difficulty')
  })

  it('keeps a stable loading state until the course API resolves', async () => {
    let resolveCourse: ((value: typeof course) => void) | undefined
    getCourseMock.mockImplementationOnce(() => new Promise<typeof course>((resolve) => { resolveCourse = resolve }))
    await renderPage()
    expect(screen.getByRole('status').textContent).toContain('Loading course')
    resolveCourse?.(course)
    await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: 'Event-driven architecture' })).toBeTruthy())
  })

  it('maps 404 separately and retries an operational failure', async () => {
    getCourseMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    await renderPage()
    expect(await screen.findByRole('heading', { level: 1, name: 'Course not found' })).toBeTruthy()

    cleanup()
    getCourseMock.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(course)
    await renderPage()
    await screen.findByRole('heading', { level: 1, name: 'Course unavailable' })
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Event-driven architecture' })).toBeTruthy()
  })
})
