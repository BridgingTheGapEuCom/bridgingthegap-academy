import { describe, expect, it, vi } from 'vitest'
import { getCourse, formatDuration, isCourseSlug, lessonPath, listCourses } from './courses'

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
})
