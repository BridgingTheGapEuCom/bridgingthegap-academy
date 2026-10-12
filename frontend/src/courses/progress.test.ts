import { describe, expect, it } from 'vitest'
import type { APIClient, APIRequestOptions } from '../api/client'
import { completeLearnerCourseLesson, createLearnerProgressSession, getLearnerCourseProgress } from './progress'

const progress = { revision: 1, completedLessonKeys: [], updatedAt: '2026-10-11T10:00:00Z' }
function client(values: unknown[]) {
  const calls: Array<{ path: string; options?: APIRequestOptions }> = []
  return { calls, client: { request: async <T>(path: string, options?: APIRequestOptions) => { calls.push({ path, options }); return values.shift() as T } } satisfies APIClient }
}

describe('learner course progress', () => {
  it('uses authenticated course-version routes without learner identity', async () => {
    const fake = client([progress, { ...progress, revision: 2, completedLessonKeys: ['lesson-one'] }])
    await getLearnerCourseProgress('20000000-0000-4000-8000-000000000001', '1.2.3', fake.client)
    await completeLearnerCourseLesson('20000000-0000-4000-8000-000000000001', '1.2.3', 'lesson-one', 1, fake.client)
    expect(fake.calls[0].path).toBe('/api/courses/by-id/20000000-0000-4000-8000-000000000001/versions/1.2.3/progress')
    expect(fake.calls[1].path).toBe('/api/courses/by-id/20000000-0000-4000-8000-000000000001/versions/1.2.3/progress/lessons/lesson-one')
    expect(JSON.parse(String(fake.calls[1].options?.body))).toEqual({ expectedRevision: 1 })
    expect(String(fake.calls[1].options?.body)).not.toContain('learner')
  })

  it('reconciles completion from the server and survives an explicit reload', async () => {
    const completed = { ...progress, revision: 2, completedLessonKeys: ['lesson-one'], updatedAt: '2026-10-11T10:01:00Z' }
    const fake = client([progress, completed, completed])
    const session = createLearnerProgressSession({ courseId: '20000000-0000-4000-8000-000000000001', version: '1.2.3' }, fake.client)
    expect(await session.load()).toBe(true)
    expect(await session.complete('lesson-one')).toBe(true)
    expect(session.progress.value).toEqual(completed)
    expect(await session.load()).toBe(true)
    expect(session.progress.value?.completedLessonKeys).toEqual(['lesson-one'])
  })
})
