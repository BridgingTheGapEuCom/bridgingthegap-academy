import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

const listPublishedCourseCatalogMock = vi.hoisted(() => vi.fn())
const getLatestPublishedCourseMock = vi.hoisted(() => vi.fn())
const getPublishedCourseVersionByIDMock = vi.hoisted(() => vi.fn())
vi.mock('../courses/courses', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../courses/courses')>()),
  listPublishedCourseCatalog: listPublishedCourseCatalogMock,
  getLatestPublishedCourse: getLatestPublishedCourseMock,
  getPublishedCourseVersionByID: getPublishedCourseVersionByIDMock,
}))

import CourseListPage from './CourseListPage.vue'
import PublishedCourseReaderPage from './PublishedCourseReaderPage.vue'

const courseID = '10000000-0000-4000-8000-000000000001'
const license = { kind: 'STANDARD', identifier: 'CC-BY-4.0', displayName: 'Creative Commons Attribution 4.0', url: 'https://creativecommons.org/licenses/by/4.0/', customText: '' }
const catalogItem = {
  courseId: courseID, version: '1.0.0', title: 'Catalog title', description: 'Discovery summary.', sourceLanguage: 'en-GB', license,
  contributors: [{ displayName: 'Ada Author', role: 'AUTHOR' as const, order: 0 }], publishedAt: '2026-09-16T12:00:00Z',
}
const readerCourse = {
  ...catalogItem,
  version: '2.0.0',
  title: 'Authoritative reader title',
  description: 'Authoritative reader detail.',
  objectives: [],
  changelog: 'Current release.',
  modules: [{
    stableKey: 'foundations', title: 'Foundations', description: '', position: 0,
    lessons: [{ stableKey: 'introduction', title: 'Introduction', description: '', objectives: [], estimatedDurationMinutes: 10, position: 0, prerequisiteStableKeys: [], content: { schemaVersion: 1, blocks: [] } }],
  }],
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((settle) => { resolve = settle })
  return { promise, resolve }
}

async function renderFlow(path = '/courses') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/courses', component: CourseListPage },
      { path: '/courses/by-id/:courseId', name: 'published-course-latest', component: PublishedCourseReaderPage },
      { path: '/courses/by-id/:courseId/versions/:version', name: 'published-course-version', component: PublishedCourseReaderPage },
    ],
  })
  await router.push(path)
  await router.isReady()
  return { ...render({ template: '<RouterView />' }, { global: { plugins: [router] } }), router }
}

describe('published course catalog to reader flow', () => {
  beforeEach(() => {
    listPublishedCourseCatalogMock.mockReset()
    getLatestPublishedCourseMock.mockReset()
    getPublishedCourseVersionByIDMock.mockReset()
  })
  afterEach(cleanup)

  it('uses catalog data only for discovery and lets the backend latest response own reader content', async () => {
    listPublishedCourseCatalogMock.mockResolvedValue({ items: [catalogItem], limit: 20, offset: 0, total: 1 })
    getLatestPublishedCourseMock.mockResolvedValue(readerCourse)
    const { router } = await renderFlow()

    await fireEvent.click(await screen.findByRole('link', { name: 'Catalog title' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Authoritative reader title' })).toBeTruthy()
    expect(screen.getByText('Authoritative reader detail.')).toBeTruthy()
    expect(screen.getByText('2.0.0')).toBeTruthy()
    expect(screen.queryByText('Discovery summary.')).toBeNull()
    expect(getLatestPublishedCourseMock).toHaveBeenCalledWith(courseID)
    expect(getPublishedCourseVersionByIDMock).not.toHaveBeenCalled()

    router.back()
    expect(await screen.findByRole('heading', { level: 1, name: 'Courses' })).toBeTruthy()
    expect(await screen.findByRole('link', { name: 'Catalog title' })).toBeTruthy()
  })

  it('does not let a late catalog response replace a reader after navigation', async () => {
    const catalog = deferred<{ items: (typeof catalogItem)[]; limit: number; offset: number; total: number }>()
    listPublishedCourseCatalogMock.mockReturnValue(catalog.promise)
    getLatestPublishedCourseMock.mockResolvedValue(readerCourse)
    const { router } = await renderFlow()

    await router.push(`/courses/by-id/${courseID}`)
    expect(await screen.findByRole('heading', { level: 1, name: 'Authoritative reader title' })).toBeTruthy()
    catalog.resolve({ items: [catalogItem], limit: 20, offset: 0, total: 1 })
    await waitFor(() => expect(screen.queryByRole('heading', { level: 1, name: 'Courses' })).toBeNull())
    expect(router.currentRoute.value.name).toBe('published-course-latest')
  })

  it('does not let a late reader response replace the catalog after navigation', async () => {
    const reader = deferred<typeof readerCourse>()
    getLatestPublishedCourseMock.mockReturnValue(reader.promise)
    listPublishedCourseCatalogMock.mockResolvedValue({ items: [catalogItem], limit: 20, offset: 0, total: 1 })
    const { router } = await renderFlow(`/courses/by-id/${courseID}`)

    await router.push('/courses')
    expect(await screen.findByRole('heading', { level: 1, name: 'Courses' })).toBeTruthy()
    reader.resolve(readerCourse)
    await waitFor(() => expect(screen.queryByRole('heading', { level: 1, name: 'Authoritative reader title' })).toBeNull())
    expect(router.currentRoute.value.path).toBe('/courses')
  })
})
