import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

const listCoursesMock = vi.hoisted(() => vi.fn())
vi.mock('../courses/courses', async (importOriginal) => ({ ...(await importOriginal<typeof import('../courses/courses')>()), listCourses: listCoursesMock }))

import CourseListPage from './CourseListPage.vue'

const courses = [{
  slug: 'event-driven-architecture',
  version: { version: '1.2.0', status: 'PUBLISHED', title: 'Event-driven architecture', description: 'A practical course.', source_language: 'en', contributors: [{ display_name: 'Ada Author', role: 'AUTHOR', order: 0 }] },
}]

async function renderPage() {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/courses', component: CourseListPage }, { path: '/courses/:slug', component: { template: '<main>Course</main>' } }] })
  await router.push('/courses')
  await router.isReady()
  return { ...render(CourseListPage, { global: { plugins: [router] } }), router }
}

describe('CourseListPage', () => {
  beforeEach(() => listCoursesMock.mockReset())
  afterEach(cleanup)

  it('renders accessible ordered course summaries from the public API', async () => {
    listCoursesMock.mockResolvedValue({ courses })
    await renderPage()
    expect(screen.getByRole('status').textContent).toContain('Loading courses')
    await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: 'Courses' })).toBeTruthy())
    const link = screen.getByRole('link', { name: 'Event-driven architecture' })
    expect(link.getAttribute('href')).toBe('/courses/event-driven-architecture')
    expect(screen.getByRole('list')).toBeTruthy()
    expect(document.body.textContent).not.toContain('Progress')
    expect(document.body.textContent).not.toContain('Difficulty')
  })

  it('distinguishes an empty response from an operational failure and retries', async () => {
    listCoursesMock.mockResolvedValueOnce({ courses: [] })
    await renderPage()
    expect(await screen.findByText('No courses are available yet.')).toBeTruthy()

    cleanup()
    listCoursesMock.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ courses })
    await renderPage()
    await screen.findByRole('heading', { level: 1, name: 'Courses unavailable' })
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('link', { name: 'Event-driven architecture' })).toBeTruthy()
  })
})
