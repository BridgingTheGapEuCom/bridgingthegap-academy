import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

const listPublishedCourseCatalogMock = vi.hoisted(() => vi.fn())
const authMock = vi.hoisted(() => ({ state: { value: { status: 'unauthenticated' as 'unauthenticated' | 'authenticated' } } }))

vi.mock('../courses/courses', async (importOriginal) => ({ ...(await importOriginal<typeof import('../courses/courses')>()), listPublishedCourseCatalog: listPublishedCourseCatalogMock }))
vi.mock('../auth/auth', () => ({ useAuth: () => authMock }))

import HomePage from './HomePage.vue'

const firstCourse = {
  courseId: '10000000-0000-4000-8000-000000000001', version: '1.0.0', title: 'Integration foundations', description: 'A practical published course.', sourceLanguage: 'en',
  license: { kind: 'STANDARD', identifier: 'CC-BY-4.0', displayName: 'Creative Commons Attribution 4.0', url: 'https://creativecommons.org/licenses/by/4.0/', customText: '' },
  contributors: [], publishedAt: '2026-09-26T12:00:00Z',
}
const secondCourse = { ...firstCourse, courseId: '20000000-0000-4000-8000-000000000002', version: '1.1.0', title: 'Event boundaries' }
const thirdCourse = { ...firstCourse, courseId: '30000000-0000-4000-8000-000000000003', version: '2.0.0', title: 'Messaging patterns' }

function catalog(items = [firstCourse, secondCourse, thirdCourse]) { return { items, limit: 2, offset: 0, total: items.length } }

async function renderPage() {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/', component: HomePage },
    { path: '/courses', component: { template: '<main>Courses</main>' } },
    { path: '/login', component: { template: '<main>Sign in</main>' } },
    { path: '/courses/by-id/:courseId', component: { template: '<main>Course</main>' } },
  ] })
  await router.push('/')
  await router.isReady()
  return { ...render({ template: '<RouterView />' }, { global: { plugins: [router] } }), router }
}

describe('HomePage', () => {
  beforeEach(() => {
    listPublishedCourseCatalogMock.mockReset()
    listPublishedCourseCatalogMock.mockResolvedValue(catalog())
    authMock.state.value = { status: 'unauthenticated' }
  })
  afterEach(cleanup)

  it('renders the learner-facing hierarchy and first two authoritative courses', async () => {
    await renderPage()
    expect(await screen.findByRole('heading', { level: 1, name: /Learn integration architecture/i })).toBeTruthy()
    expect(screen.queryByText(/application foundation is ready/i)).toBeNull()
    expect(screen.getByRole('heading', { level: 2, name: 'What you can learn' })).toBeTruthy()
    expect(screen.getByRole('heading', { level: 3, name: 'Integration Architecture' })).toBeTruthy()
    expect(screen.getByRole('heading', { level: 3, name: 'Event-Driven Architecture' })).toBeTruthy()
    expect(screen.getByRole('heading', { level: 3, name: 'Messaging & Integration Patterns' })).toBeTruthy()
    expect(screen.getByRole('heading', { level: 2, name: 'Designed for focused learning' })).toBeTruthy()
    expect(screen.getByRole('heading', { level: 3, name: 'Structured' })).toBeTruthy()
    expect(screen.getByRole('heading', { level: 3, name: 'Accessible' })).toBeTruthy()
    expect(screen.getByRole('heading', { level: 3, name: 'Vendor-neutral' })).toBeTruthy()
    expect(screen.getByRole('heading', { level: 3, name: 'Self-paced' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Integration foundations' }).getAttribute('href')).toBe('/courses/by-id/10000000-0000-4000-8000-000000000001')
    expect(screen.getByRole('link', { name: 'Event boundaries' })).toBeTruthy()
    expect(screen.queryByRole('link', { name: 'Messaging patterns' })).toBeNull()
    expect(listPublishedCourseCatalogMock).toHaveBeenCalledWith({ limit: 2, offset: 0 })
    expect(screen.getAllByRole('link', { name: /Browse courses/i }).every((link) => link.getAttribute('href') === '/courses')).toBe(true)
    expect(screen.getByRole('link', { name: /View all/i }).getAttribute('href')).toBe('/courses')
  })

  it('keeps an empty or failed course request separate from the informational homepage', async () => {
    listPublishedCourseCatalogMock.mockResolvedValueOnce(catalog([]))
    await renderPage()
    expect(await screen.findByText('Courses are being prepared.')).toBeTruthy()
    expect(within(screen.getByRole('status')).getByRole('link', { name: 'Browse courses' }).getAttribute('href')).toBe('/courses')

    cleanup()
    listPublishedCourseCatalogMock.mockRejectedValueOnce(new Error('private error')).mockResolvedValueOnce(catalog([firstCourse]))
    await renderPage()
    expect((await screen.findByRole('alert')).textContent).toContain('Published courses are unavailable right now.')
    expect(screen.getByRole('heading', { level: 2, name: 'What you can learn' })).toBeTruthy()
    expect(document.body.textContent).not.toContain('private error')
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('link', { name: 'Integration foundations' })).toBeTruthy()
  })

  it('offers only the real sign-in route to signed-out visitors', async () => {
    const { router } = await renderPage()
    await screen.findByRole('heading', { level: 1 })
    await fireEvent.click(screen.getByRole('link', { name: 'Sign in' }))
    await waitFor(() => expect(router.currentRoute.value.path).toBe('/login'))

    cleanup()
    authMock.state.value = { status: 'authenticated' }
    await renderPage()
    await screen.findByRole('heading', { level: 1 })
    expect(screen.queryByRole('link', { name: 'Sign in' })).toBeNull()
  })
})
