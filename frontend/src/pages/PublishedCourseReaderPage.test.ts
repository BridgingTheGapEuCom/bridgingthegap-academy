import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { APIProblemError } from '../api/client'

const getLatestPublishedCourseMock = vi.hoisted(() => vi.fn())
const getPublishedCourseVersionByIDMock = vi.hoisted(() => vi.fn())
vi.mock('../courses/courses', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../courses/courses')>()),
  getLatestPublishedCourse: getLatestPublishedCourseMock,
  getPublishedCourseVersionByID: getPublishedCourseVersionByIDMock,
}))

import PublishedCourseReaderPage from './PublishedCourseReaderPage.vue'

const courseID = '10000000-0000-4000-8000-000000000001'
const course = {
  courseId: courseID,
  version: '1.10.0',
  title: 'Event-driven architecture',
  description: 'A practical published course.',
  objectives: ['Explain event ownership'],
  sourceLanguage: 'en-GB',
  changelog: 'First public release.',
  license: { kind: 'STANDARD', identifier: 'CC-BY-4.0', displayName: 'Creative Commons Attribution 4.0', url: 'https://creativecommons.org/licenses/by/4.0/', customText: '' },
  contributors: [{ displayName: 'Ada Author', role: 'AUTHOR' as const, order: 0 }],
  publishedAt: '2026-09-16T12:00:00Z',
  modules: [
    {
      stableKey: 'fundamentals', title: 'Fundamentals', description: 'Core concepts.', position: 0,
      lessons: [
        { stableKey: 'what-is-eai', title: 'What is EAI?', description: 'A starting point.', objectives: ['Recognise integration'], estimatedDurationMinutes: 10, position: 0, prerequisiteStableKeys: [], content: { schemaVersion: 1, blocks: [{ stableKey: 'hidden-block', type: 'PARAGRAPH', payload: { text: 'Raw canonical payload must not render.' } }] } },
        { stableKey: 'sync-vs-async', title: 'Synchronous and asynchronous', description: 'Compare approaches.', objectives: [], estimatedDurationMinutes: 75, position: 1, prerequisiteStableKeys: ['what-is-eai'], content: { schemaVersion: 1, blocks: [] } },
      ],
    },
    {
      stableKey: 'delivery', title: 'Delivery', description: '', position: 1,
      lessons: [{ stableKey: 'event-contracts', title: 'Event contracts', description: '', objectives: [], estimatedDurationMinutes: null, position: 0, prerequisiteStableKeys: [], content: { schemaVersion: 1, blocks: [] } }],
    },
  ],
}

function routerFor(path: string) {
  return createRouter({ history: createMemoryHistory(), routes: [
    { path: '/courses', component: { template: '<main>Catalog</main>' } },
    { path: '/courses/by-id/:courseId', component: PublishedCourseReaderPage },
    { path: '/courses/by-id/:courseId/versions/:version', component: PublishedCourseReaderPage },
  ] })
}

async function renderPage(path = `/courses/by-id/${courseID}`) {
  const router = routerFor(path)
  await router.push(path)
  await router.isReady()
  return { ...render(PublishedCourseReaderPage, { global: { plugins: [router] } }), router }
}

describe('PublishedCourseReaderPage', () => {
  beforeEach(() => {
    getLatestPublishedCourseMock.mockReset()
    getPublishedCourseVersionByIDMock.mockReset()
  })
  afterEach(cleanup)

  it('loads the latest immutable version and preserves module and lesson order without rendering canonical content', async () => {
    getLatestPublishedCourseMock.mockResolvedValue(course)
    const { router } = await renderPage()
    expect(await screen.findByRole('heading', { level: 1, name: 'Event-driven architecture' })).toBeTruthy()
    expect(getLatestPublishedCourseMock).toHaveBeenCalledWith(courseID)
    expect(screen.getAllByRole('heading', { level: 2 }).map((heading) => heading.textContent)).toEqual(expect.arrayContaining(['Fundamentals', 'Delivery', 'What is EAI?']))
    const links = [
      screen.getByRole('link', { name: 'What is EAI?, estimated duration 10 min' }),
      screen.getByRole('link', { name: 'Synchronous and asynchronous, estimated duration 1 hr 15 min' }),
      screen.getByRole('link', { name: 'Event contracts' }),
    ]
    expect(links.map((link) => link.textContent?.replace(/\s+/g, ' ').trim())).toEqual(['What is EAI?10 min', 'Synchronous and asynchronous1 hr 15 min', 'Event contracts'])
    expect(links[0].getAttribute('aria-current')).toBe('page')
    await waitFor(() => expect(router.currentRoute.value.query.lesson).toBe('what-is-eai'))
    expect(screen.getByText('Lesson content will be available here.')).toBeTruthy()
    expect(document.body.textContent).not.toContain('Raw canonical payload must not render.')
    expect(screen.getByRole('link', { name: 'Browse courses' }).getAttribute('href')).toBe('/courses')
  })

  it('loads an exact version, honors a valid lesson URL, and updates URL/current semantics on lesson navigation', async () => {
    const secondVersion = { ...course, version: '2.0.0', modules: [course.modules[1]] }
    getPublishedCourseVersionByIDMock.mockResolvedValueOnce(course).mockResolvedValueOnce(secondVersion)
    const { router } = await renderPage(`/courses/by-id/${courseID}/versions/1.10.0?lesson=sync-vs-async`)
    expect(await screen.findByRole('heading', { level: 2, name: 'Synchronous and asynchronous' })).toBeTruthy()
    expect(getPublishedCourseVersionByIDMock).toHaveBeenCalledWith(courseID, '1.10.0')
    expect(screen.getByRole('link', { name: 'Synchronous and asynchronous, estimated duration 1 hr 15 min' }).getAttribute('aria-current')).toBe('page')
    await fireEvent.click(screen.getByRole('link', { name: 'Event contracts' }))
    await waitFor(() => expect(router.currentRoute.value.query.lesson).toBe('event-contracts'))
    expect(await screen.findByRole('heading', { level: 2, name: 'Event contracts' })).toBeTruthy()
    expect(screen.getByRole('heading', { level: 2, name: 'Event contracts' }).getAttribute('tabindex')).toBe('-1')

    await router.push(`/courses/by-id/${courseID}/versions/2.0.0?lesson=sync-vs-async`)
    expect(await screen.findByText('2.0.0')).toBeTruthy()
    expect(await screen.findByRole('heading', { level: 2, name: 'Event contracts' })).toBeTruthy()
    await waitFor(() => expect(router.currentRoute.value.query.lesson).toBe('event-contracts'))
  })

  it('normalizes an invalid lesson for a loaded version and handles empty, unavailable, and not-found courses safely', async () => {
    getLatestPublishedCourseMock.mockResolvedValueOnce(course)
    const { router } = await renderPage(`/courses/by-id/${courseID}?lesson=missing-lesson`)
    await screen.findByRole('heading', { level: 2, name: 'What is EAI?' })
    await waitFor(() => expect(router.currentRoute.value.query.lesson).toBe('what-is-eai'))

    cleanup()
    getLatestPublishedCourseMock.mockResolvedValueOnce({ ...course, modules: [{ stableKey: 'empty-module', title: 'Empty module', description: '', position: 0, lessons: [] }] })
    await renderPage()
    expect(await screen.findByText('This course does not contain any lessons yet.')).toBeTruthy()

    cleanup()
    getLatestPublishedCourseMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    await renderPage()
    expect(await screen.findByRole('heading', { level: 1, name: 'Course not found' })).toBeTruthy()

    cleanup()
    getLatestPublishedCourseMock.mockRejectedValueOnce(new Error('private database failure')).mockResolvedValueOnce(course)
    await renderPage()
    await screen.findByRole('alert')
    expect(document.body.textContent).not.toContain('private database failure')
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Event-driven architecture' })).toBeTruthy()
  })

  it('discards a late course response and does not retain a lesson from another course', async () => {
    let resolveFirst: ((value: typeof course) => void) | undefined
    const secondCourseID = '10000000-0000-4000-8000-000000000002'
    const secondCourse = { ...course, courseId: secondCourseID, title: 'Second course', modules: [{ ...course.modules[0], lessons: [course.modules[0].lessons[1]] }] }
    getLatestPublishedCourseMock.mockImplementation((requestedCourseID: string) => {
      if (requestedCourseID === courseID) return new Promise<typeof course>((resolve) => { resolveFirst = resolve })
      return Promise.resolve(secondCourse)
    })
    const { router } = await renderPage()
    await router.push(`/courses/by-id/${secondCourseID}?lesson=what-is-eai`)
    expect(await screen.findByRole('heading', { level: 2, name: 'Synchronous and asynchronous' })).toBeTruthy()
    await waitFor(() => expect(router.currentRoute.value.query.lesson).toBe('sync-vs-async'))
    resolveFirst?.(course)
    await waitFor(() => expect(screen.queryByRole('heading', { level: 2, name: 'What is EAI?' })).toBeNull())
    expect(screen.getByRole('heading', { level: 1, name: 'Second course' })).toBeTruthy()
  })
})
