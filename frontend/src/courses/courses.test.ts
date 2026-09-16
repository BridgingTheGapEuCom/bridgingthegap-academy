import { describe, expect, it, vi } from 'vitest'
import { catalogOffsetFromRoute, getCourse, getCourseVersion, getLatestPublishedCourse, getLesson, getPublishedCourseVersionByID, formatDuration, isCourseSlug, isCourseVersion, lessonPath, listCourses, listPublishedCourseCatalog, publishedCoursePath, publishedCourseVersionPath, publishedLessonKeyFromRoute } from './courses'

const publishedLicense = { kind: 'STANDARD', identifier: 'CC-BY-4.0', displayName: 'Creative Commons Attribution 4.0', url: 'https://creativecommons.org/licenses/by/4.0/', customText: '' }
const publishedCatalogItem = {
  courseId: '10000000-0000-4000-8000-000000000001', version: '1.10.0', title: 'Architecture', description: 'A course.', sourceLanguage: 'en-GB',
  license: publishedLicense, contributors: [{ displayName: 'Ada', role: 'AUTHOR', order: 0 }], publishedAt: '2026-09-16T12:00:00Z',
}
const publishedCourse = {
  ...publishedCatalogItem,
  objectives: ['Understand boundaries'],
  changelog: 'Initial release.',
  modules: [{
    stableKey: 'foundations', title: 'Foundations', description: 'Core concepts.', position: 0,
    lessons: [{
      stableKey: 'introduction', title: 'Introduction', description: 'Start here.', objectives: [], estimatedDurationMinutes: 10,
      position: 0, prerequisiteStableKeys: [], content: { schemaVersion: 1, blocks: [] },
    }],
  }],
}

describe('Courses service', () => {
  it('uses the shared API client and validates public course paths', async () => {
    const request = vi.fn().mockResolvedValue({ courses: [] })
    await listCourses({ request })
    expect(request).toHaveBeenCalledWith('/api/courses')

    await getCourse('event-driven-architecture', { request })
    expect(request).toHaveBeenLastCalledWith('/api/courses/event-driven-architecture')
    expect(isCourseSlug('event-driven-architecture')).toBe(true)
    expect(isCourseSlug('not valid')).toBe(false)
    await expect(getCourse('not valid', { request })).rejects.toThrow('Invalid course slug')
  })

  it('formats informational durations and immutable lesson URLs deterministically', () => {
    expect(formatDuration(10)).toBe('10 min')
    expect(formatDuration(60)).toBe('1 hr')
    expect(formatDuration(75)).toBe('1 hr 15 min')
    expect(formatDuration(0)).toBeUndefined()
    expect(lessonPath('event-driven-architecture', '1.10.0', 'sync-vs-async')).toBe('/courses/event-driven-architecture/versions/1.10.0/lessons/sync-vs-async')
  })

  it('uses only bounded canonical exact-version lesson paths', async () => {
    const request = vi.fn().mockResolvedValue({})
    await getCourseVersion('event-driven-architecture', '1.10.0', { request })
    expect(request).toHaveBeenCalledWith('/api/courses/event-driven-architecture/versions/1.10.0')
    await getLesson('event-driven-architecture', '1.10.0', 'sync-vs-async', { request })
    expect(request).toHaveBeenLastCalledWith('/api/courses/event-driven-architecture/versions/1.10.0/lessons/sync-vs-async')
    expect(isCourseVersion('1.10.0')).toBe(true)
    expect(isCourseVersion('01.10.0')).toBe(false)
    await expect(getLesson('event-driven-architecture', 'not-a-version', 'sync-vs-async', { request })).rejects.toThrow('Invalid course route')
  })

  it('uses the generated published catalog contract with bounded pagination', async () => {
    const request = vi.fn().mockResolvedValue({ items: [publishedCatalogItem], limit: 20, offset: 20, total: 21 })
    await listPublishedCourseCatalog({ offset: 20, language: 'en-GB' }, { request })
    expect(request).toHaveBeenCalledWith('/api/courses/catalog?limit=20&offset=20&language=en-GB', { cache: 'no-store' })
    expect(publishedCoursePath('10000000-0000-4000-8000-000000000001')).toBe('/courses/by-id/10000000-0000-4000-8000-000000000001')
    expect(catalogOffsetFromRoute(undefined)).toBe(0)
    expect(catalogOffsetFromRoute('20')).toBe(20)
    expect(catalogOffsetFromRoute('-1')).toBeUndefined()
    expect(catalogOffsetFromRoute(['20'])).toBeUndefined()
    await expect(listPublishedCourseCatalog({ offset: -1 }, { request })).rejects.toThrow('Invalid course route')
  })

  it('uses exact Courses-owned endpoints for immutable published versions', async () => {
    const request = vi.fn().mockResolvedValue(publishedCourse)
    const courseID = '10000000-0000-4000-8000-000000000001'
    await getLatestPublishedCourse(courseID, { request })
    expect(request).toHaveBeenCalledWith(`/api/courses/by-id/${courseID}/latest`, { cache: 'no-store' })
    await getPublishedCourseVersionByID(courseID, '1.10.0', { request })
    expect(request).toHaveBeenLastCalledWith(`/api/courses/by-id/${courseID}/versions/1.10.0`, { cache: 'no-store' })
    expect(publishedCourseVersionPath(courseID, '1.10.0')).toBe(`/courses/by-id/${courseID}/versions/1.10.0`)
    expect(publishedLessonKeyFromRoute('sync-vs-async')).toBe('sync-vs-async')
    expect(publishedLessonKeyFromRoute('not valid')).toBeUndefined()
    await expect(getLatestPublishedCourse('not-a-uuid', { request })).rejects.toThrow('Invalid course route')
  })

  it('fails closed for malformed public catalog and reader payloads', async () => {
    const wrongPage = { request: vi.fn().mockResolvedValue({ items: [publishedCatalogItem], limit: 20, offset: 0, total: 1 }) }
    await expect(listPublishedCourseCatalog({ offset: 20 }, wrongPage)).rejects.toThrow('Invalid published course response')

    const malformedCatalog = { request: vi.fn().mockResolvedValue({ items: [{ ...publishedCatalogItem, courseId: 'draft-id' }], limit: 20, offset: 0, total: 1 }) }
    await expect(listPublishedCourseCatalog({}, malformedCatalog)).rejects.toThrow('Invalid published course response')

    const malformedCourse = { request: vi.fn().mockResolvedValue({ ...publishedCourse, modules: null }) }
    await expect(getLatestPublishedCourse(publishedCatalogItem.courseId, malformedCourse)).rejects.toThrow('Invalid published course response')
  })
})
