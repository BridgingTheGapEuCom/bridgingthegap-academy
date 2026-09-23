import { describe, expect, it } from 'vitest'
import type { APIClient, APIRequestOptions } from '../api/client'
import { createLearnerAttempt, submitLearnerAttempt, updateLearnerAttempt } from './attempts'

const attempt = {
  attemptId: '30000000-0000-4000-8000-000000000001', assessmentKey: '10000000-0000-4000-8000-000000000001', state: 'IN_PROGRESS' as const, revision: 1,
  responses: [], createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z', submittedAt: null, result: null,
}
function client(value: unknown) { const calls: Array<{ path: string; options?: APIRequestOptions }> = []; return { calls, client: { request: async <T>(path: string, options?: APIRequestOptions) => { calls.push({ path, options }); return value as T } } satisfies APIClient } }

describe('learner attempt API', () => {
  it('uses the exact CourseVersion create route without learner identity', async () => {
    const fake = client(attempt)
    await createLearnerAttempt('20000000-0000-4000-8000-000000000001', '1.2.3', attempt.assessmentKey, fake.client)
    expect(fake.calls[0]).toEqual({ path: `/api/courses/by-id/20000000-0000-4000-8000-000000000001/versions/1.2.3/assessments/${attempt.assessmentKey}/attempts`, options: { method: 'POST', cache: 'no-store' } })
  })
  it('sends only expected revision and learner responses for save and submit', async () => {
    const fake = client(attempt)
    const responses = [{ questionKey: 'question-one', type: 'SINGLE_CHOICE' as const, selectedOptionKey: 'option-one', selectedOptionKeys: [], pairs: [] }]
    await updateLearnerAttempt(attempt.attemptId, 1, responses, fake.client)
    await submitLearnerAttempt(attempt.attemptId, 1, fake.client)
    expect(JSON.parse(String(fake.calls[0].options?.body))).toEqual({ expectedRevision: 1, responses })
    expect(JSON.parse(String(fake.calls[1].options?.body))).toEqual({ expectedRevision: 1 })
    expect(String(fake.calls[0].options?.body)).not.toMatch(/learnerId|answerKey|correct/i)
  })
  it('rejects malformed successful attempt payloads', async () => {
    const fake = client({ ...attempt, result: { correctOptionKey: 'secret' } })
    await expect(createLearnerAttempt('20000000-0000-4000-8000-000000000001', '1.2.3', attempt.assessmentKey, fake.client)).rejects.toThrow('Invalid published course response')
  })
})

it('creates lazily, saves response changes, then submits with the returned revision', async () => {
  const submitted = { ...attempt, state: 'SUBMITTED' as const, revision: 3, responses: [{ questionKey: 'question-one', type: 'SINGLE_CHOICE' as const, selectedOptionKey: 'option-one', selectedOptionKeys: [], pairs: [] }], submittedAt: '2026-01-01T00:01:00Z', result: { correctCount: 1, totalCount: 1, percentage: 100 } }
  const calls: Array<{ path: string; options?: APIRequestOptions }> = []
  const values = [attempt, { ...attempt, revision: 2, responses: submitted.responses }, submitted]
  const fake: APIClient = { request: async <T>(path: string, options?: APIRequestOptions) => { calls.push({ path, options }); return values.shift() as T } }
  const { createLearnerAttemptSession } = await import('./attempts')
  const session = createLearnerAttemptSession({ courseId: '20000000-0000-4000-8000-000000000001', version: '1.2.3', assessmentKey: attempt.assessmentKey }, fake)
  session.responses.value = submitted.responses
  expect(await session.submit()).toBe(true)
  expect(calls.map((call) => call.options?.method)).toEqual(['POST', 'PUT', 'POST'])
  expect(session.attempt.value?.state).toBe('SUBMITTED')
  expect(session.attempt.value?.result).toEqual(submitted.result)
})
