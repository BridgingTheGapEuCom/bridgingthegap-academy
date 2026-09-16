import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

const listPublishedCourseCatalogMock = vi.hoisted(() => vi.fn())
vi.mock('../courses/courses', async (importOriginal) => ({ ...(await importOriginal<typeof import('../courses/courses')>()), listPublishedCourseCatalog: listPublishedCourseCatalogMock }))

import CourseListPage from './CourseListPage.vue'

const firstCourse = {
  courseId: '10000000-0000-4000-8000-000000000001', version: '1.10.0', title: 'Architecture foundations', description: 'A practical published course.', sourceLanguage: 'en-GB',
  license: { kind: 'STANDARD', identifier: 'CC-BY-4.0', displayName: 'Creative Commons Attribution 4.0', url: 'https://creativecommons.org/licenses/by/4.0/', customText: '' },
  contributors: [{ displayName: 'Ada Author', role: 'AUTHOR', order: 0 }], publishedAt: '2026-09-16T12:00:00Z',
}
const secondCourse = {
  ...firstCourse, courseId: '10000000-0000-4000-8000-000000000002', version: '2.0.0', title: 'Boundary design', description: 'A second published course.',
}

function catalogPage(items = [firstCourse], offset = 0, total = items.length) {
  return { items, limit: 20, offset, total }
}

async function renderPage(path = '/courses') {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/courses', component: CourseListPage },
    { path: '/courses/by-id/:courseId', component: { template: '<main>Published course</main>' } },
  ] })
  await router.push(path)
  await router.isReady()
  return { ...render(CourseListPage, { global: { plugins: [router] } }), router }
}

describe('CourseListPage', () => {
  beforeEach(() => listPublishedCourseCatalogMock.mockReset())
  afterEach(cleanup)

  it('renders accessible published-course summaries in API order without internal provenance', async () => {
    listPublishedCourseCatalogMock.mockResolvedValue(catalogPage([firstCourse, secondCourse], 0, 2))
    await renderPage()
    const firstLink = await screen.findByRole('link', { name: 'Architecture foundations' })
    expect(firstLink.getAttribute('href')).toBe('/courses/by-id/10000000-0000-4000-8000-000000000001')
    const list = screen.getByRole('list', { name: 'Published courses' })
    expect(list.textContent).toContain('Version')
    expect(list.textContent).toContain('en-GB')
    expect(list.textContent).toContain('Ada Author')
    expect(list.textContent).not.toContain('reviewId')
    expect(list.textContent).not.toContain('draftId')
    expect(screen.getAllByRole('listitem')[0].textContent).toContain('Architecture foundations')
    expect(screen.getAllByRole('listitem')[1].textContent).toContain('Boundary design')
  })

  it('keeps an accessible loading state until the catalog response resolves', async () => {
    let resolveCatalog: ((value: ReturnType<typeof catalogPage>) => void) | undefined
    listPublishedCourseCatalogMock.mockImplementationOnce(() => new Promise<ReturnType<typeof catalogPage>>((resolve) => { resolveCatalog = resolve }))
    await renderPage()
    expect(screen.getByRole('status').textContent).toContain('Loading published courses')
    resolveCatalog?.(catalogPage())
    expect(await screen.findByRole('link', { name: 'Architecture foundations' })).toBeTruthy()
  })

  it('renders an empty catalog separately from a sanitized operational failure and retries', async () => {
    listPublishedCourseCatalogMock.mockResolvedValueOnce(catalogPage([]))
    await renderPage()
    expect(await screen.findByText('No published courses are available yet.')).toBeTruthy()

    cleanup()
    listPublishedCourseCatalogMock.mockRejectedValueOnce(new Error('private database failure')).mockResolvedValueOnce(catalogPage())
    await renderPage()
    await screen.findByRole('alert')
    expect(document.body.textContent).not.toContain('private database failure')
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('link', { name: 'Architecture foundations' })).toBeTruthy()
  })

  it('uses URL pagination state and enables only available page controls', async () => {
    listPublishedCourseCatalogMock.mockResolvedValueOnce(catalogPage([firstCourse], 0, 21)).mockResolvedValueOnce(catalogPage([secondCourse], 20, 21)).mockResolvedValueOnce(catalogPage([firstCourse], 0, 21))
    const { router } = await renderPage()
    await screen.findByRole('link', { name: 'Architecture foundations' })
    expect((screen.getByRole('button', { name: 'Previous' }) as HTMLButtonElement).disabled).toBe(true)
    expect((screen.getByRole('button', { name: 'Next' }) as HTMLButtonElement).disabled).toBe(false)

    await fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await waitFor(() => expect(router.currentRoute.value.query.offset).toBe('20'))
    expect(await screen.findByRole('link', { name: 'Boundary design' })).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Next' }) as HTMLButtonElement).disabled).toBe(true)

    await fireEvent.click(screen.getByRole('button', { name: 'Previous' }))
    await waitFor(() => expect(router.currentRoute.value.query.offset).toBeUndefined())
    expect(await screen.findByRole('link', { name: 'Architecture foundations' })).toBeTruthy()
    expect(listPublishedCourseCatalogMock.mock.calls.map(([query]) => query.offset)).toEqual([0, 20, 0])
  })

  it('guards rapid pagination activation until the route transition starts loading', async () => {
    let resolveNext: ((value: ReturnType<typeof catalogPage>) => void) | undefined
    listPublishedCourseCatalogMock.mockResolvedValueOnce(catalogPage([firstCourse], 0, 21))
      .mockImplementationOnce(() => new Promise<ReturnType<typeof catalogPage>>((resolve) => { resolveNext = resolve }))
    await renderPage()
    await screen.findByRole('link', { name: 'Architecture foundations' })
    const next = screen.getByRole('button', { name: 'Next' })
    await fireEvent.click(next)
    await fireEvent.click(next)
    await waitFor(() => expect(listPublishedCourseCatalogMock).toHaveBeenCalledTimes(2))
    resolveNext?.(catalogPage([secondCourse], 20, 21))
    expect(await screen.findByRole('link', { name: 'Boundary design' })).toBeTruthy()
  })

  it('normalizes malformed offsets and discards a late response from an earlier catalog query', async () => {
    let resolveFirst: ((value: ReturnType<typeof catalogPage>) => void) | undefined
    listPublishedCourseCatalogMock.mockImplementationOnce(() => new Promise<ReturnType<typeof catalogPage>>((resolve) => { resolveFirst = resolve }))
      .mockResolvedValueOnce(catalogPage([secondCourse], 20, 21))
    const { router } = await renderPage('/courses?offset=invalid')
    await waitFor(() => expect(router.currentRoute.value.query.offset).toBeUndefined())
    await waitFor(() => expect(listPublishedCourseCatalogMock).toHaveBeenCalledWith({ limit: 20, offset: 0 }))
    await router.push('/courses?offset=20')
    expect(await screen.findByRole('link', { name: 'Boundary design' })).toBeTruthy()
    resolveFirst?.(catalogPage([firstCourse], 0, 1))
    await waitFor(() => expect(screen.queryByRole('link', { name: 'Architecture foundations' })).toBeNull())
  })

  it('normalizes a page that became empty after publications became unavailable', async () => {
    listPublishedCourseCatalogMock
      .mockResolvedValueOnce(catalogPage([], 40, 21))
      .mockResolvedValueOnce(catalogPage([secondCourse], 20, 21))
    const { router } = await renderPage('/courses?offset=40')

    await waitFor(() => expect(router.currentRoute.value.query.offset).toBe('20'))
    expect(await screen.findByRole('link', { name: 'Boundary design' })).toBeTruthy()
    expect(screen.queryByText('No published courses are available yet.')).toBeNull()
    expect(listPublishedCourseCatalogMock.mock.calls.map(([query]) => query.offset)).toEqual([40, 20])
  })

  it('keeps pagination available for a transient empty nonzero page', async () => {
    listPublishedCourseCatalogMock.mockResolvedValueOnce(catalogPage([], 20, 40))
    await renderPage('/courses?offset=20')

    expect(await screen.findByText('No published courses are available on this page.')).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Previous' }) as HTMLButtonElement).disabled).toBe(false)
  })
})
