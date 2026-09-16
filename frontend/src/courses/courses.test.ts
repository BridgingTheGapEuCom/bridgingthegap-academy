import { describe, expect, it, vi } from 'vitest'
import { catalogOffsetFromRoute, getCourse, getCourseVersion, getLesson, formatDuration, isCourseSlug, isCourseVersion, lessonPath, listCourses, listPublishedCourseCatalog, publishedCoursePath } from './courses'

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
    const request = vi.fn().mockResolvedValue({ items: [], limit: 20, offset: 20, total: 21 })
    await listPublishedCourseCatalog({ offset: 20, language: 'en-GB' }, { request })
    expect(request).toHaveBeenCalledWith('/api/courses/catalog?limit=20&offset=20&language=en-GB')
    expect(publishedCoursePath('10000000-0000-4000-8000-000000000001')).toBe('/courses/by-id/10000000-0000-4000-8000-000000000001')
    expect(catalogOffsetFromRoute(undefined)).toBe(0)
    expect(catalogOffsetFromRoute('20')).toBe(20)
    expect(catalogOffsetFromRoute('-1')).toBeUndefined()
    expect(catalogOffsetFromRoute(['20'])).toBeUndefined()
    await expect(listPublishedCourseCatalog({ offset: -1 }, { request })).rejects.toThrow('Invalid course route')
  })
})
