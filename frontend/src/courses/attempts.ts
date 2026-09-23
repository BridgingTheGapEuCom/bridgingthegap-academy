import { computed, ref, type ComputedRef, type Ref } from 'vue'
import { APIProblemError, type APIClient } from '../api/client'
import type { components } from '../api/generated'
import { isCourseVersion, isPublishedCourseID, InvalidCourseRouteError, InvalidPublishedCourseResponseError } from './courses'

export type LearnerAssessmentAttempt = components['schemas']['LearnerAssessmentAttempt']
export type LearnerAssessmentAttemptResponse = components['schemas']['LearnerAssessmentAttemptResponse']
export type LearnerAssessmentAttemptResult = components['schemas']['LearnerAssessmentAttemptResult']

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i
const key = /^[a-z0-9]+(?:-[a-z0-9]+)*$/

export async function createLearnerAttempt(courseId: string, version: string, assessmentKey: string, client: APIClient): Promise<LearnerAssessmentAttempt> {
  if (!isPublishedCourseID(courseId) || !isCourseVersion(version) || !isUUID(assessmentKey)) throw new InvalidCourseRouteError()
  return requestAttempt(client, `/api/courses/by-id/${encodeURIComponent(courseId)}/versions/${encodeURIComponent(version)}/assessments/${encodeURIComponent(assessmentKey)}/attempts`, { method: 'POST', cache: 'no-store' })
}

export async function getLearnerAttempt(attemptId: string, client: APIClient): Promise<LearnerAssessmentAttempt> {
  if (!isUUID(attemptId)) throw new InvalidCourseRouteError()
  return requestAttempt(client, `/api/learner/assessment-attempts/${encodeURIComponent(attemptId)}`, { cache: 'no-store' })
}

export async function updateLearnerAttempt(attemptId: string, expectedRevision: number, responses: LearnerAssessmentAttemptResponse[], client: APIClient): Promise<LearnerAssessmentAttempt> {
  if (!isUUID(attemptId) || !isPositiveInteger(expectedRevision) || !isResponses(responses)) throw new InvalidCourseRouteError()
  return requestAttempt(client, `/api/learner/assessment-attempts/${encodeURIComponent(attemptId)}`, {
    method: 'PUT', cache: 'no-store', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ expectedRevision, responses }),
  })
}

export async function submitLearnerAttempt(attemptId: string, expectedRevision: number, client: APIClient): Promise<LearnerAssessmentAttempt> {
  if (!isUUID(attemptId) || !isPositiveInteger(expectedRevision)) throw new InvalidCourseRouteError()
  return requestAttempt(client, `/api/learner/assessment-attempts/${encodeURIComponent(attemptId)}/submit`, {
    method: 'POST', cache: 'no-store', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ expectedRevision }),
  })
}

async function requestAttempt(client: APIClient, path: string, options: Parameters<APIClient['request']>[1]): Promise<LearnerAssessmentAttempt> {
  const value = await client.request<unknown>(path, options)
  if (!isLearnerAssessmentAttempt(value)) throw new InvalidPublishedCourseResponseError()
  return value
}

export function isLearnerAssessmentAttempt(value: unknown): value is LearnerAssessmentAttempt {
  return isRecord(value) && hasOnlyKeys(value, ['attemptId', 'assessmentKey', 'state', 'revision', 'responses', 'createdAt', 'updatedAt', 'submittedAt', 'result'])
    && isUUID(value.attemptId) && isUUID(value.assessmentKey)
    && (value.state === 'IN_PROGRESS' || value.state === 'SUBMITTED') && isPositiveInteger(value.revision)
    && isResponses(value.responses) && isDate(value.createdAt) && isDate(value.updatedAt)
    && (value.submittedAt === null || isDate(value.submittedAt))
    && (value.result === null || isResult(value.result))
    && ((value.state === 'IN_PROGRESS' && value.submittedAt === null && value.result === null) || (value.state === 'SUBMITTED' && value.submittedAt !== null && value.result !== null))
}

function isResponses(value: unknown): value is LearnerAssessmentAttemptResponse[] {
  if (!Array.isArray(value) || value.length > 100) return false
  const questions = new Set<string>()
  return value.every((response) => {
    if (!isRecord(response) || !hasOnlyKeys(response, ['questionKey', 'type', 'selectedOptionKey', 'selectedOptionKeys', 'pairs']) || !isStableKey(response.questionKey) || questions.has(response.questionKey) || typeof response.selectedOptionKey !== 'string' || !Array.isArray(response.selectedOptionKeys) || !Array.isArray(response.pairs)) return false
    questions.add(response.questionKey)
    if (response.type === 'SINGLE_CHOICE') return isStableKey(response.selectedOptionKey) && response.selectedOptionKeys.length === 0 && response.pairs.length === 0
    if (response.type === 'MULTIPLE_CHOICE') return response.selectedOptionKey === '' && isKeySet(response.selectedOptionKeys) && response.pairs.length === 0
    return response.type === 'MATCHING' && response.selectedOptionKey === '' && response.selectedOptionKeys.length === 0 && isPairs(response.pairs)
  })
}
function isKeySet(value: unknown): value is string[] { return Array.isArray(value) && value.length > 0 && value.every(isStableKey) && new Set(value).size === value.length }
function isPairs(value: unknown): boolean { return Array.isArray(value) && value.length > 0 && value.every((pair) => isRecord(pair) && hasOnlyKeys(pair, ['leftItemKey', 'rightItemKey']) && isStableKey(pair.leftItemKey) && isStableKey(pair.rightItemKey)) && new Set(value.map((pair) => (pair as { leftItemKey: string }).leftItemKey)).size === value.length }
function isResult(value: unknown): value is LearnerAssessmentAttemptResult { return isRecord(value) && hasOnlyKeys(value, ['correctCount', 'totalCount', 'percentage']) && typeof value.correctCount === 'number' && Number.isInteger(value.correctCount) && value.correctCount >= 0 && typeof value.totalCount === 'number' && Number.isInteger(value.totalCount) && value.totalCount > 0 && value.correctCount <= value.totalCount && typeof value.percentage === 'number' && value.percentage >= 0 && value.percentage <= 100 }
function isUUID(value: unknown): value is string { return typeof value === 'string' && uuid.test(value) }
function isStableKey(value: unknown): value is string { return typeof value === 'string' && value.length <= 160 && key.test(value) }
function isPositiveInteger(value: unknown): value is number { return Number.isSafeInteger(value) && (value as number) > 0 }
function isDate(value: unknown): value is string { return typeof value === 'string' && !Number.isNaN(Date.parse(value)) }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }
function hasOnlyKeys(value: Record<string, unknown>, keys: string[]): boolean { return Object.keys(value).every((item) => keys.includes(item)) }

export type LearnerAttemptSession = {
  responses: Ref<LearnerAssessmentAttemptResponse[]>
  attempt: Ref<LearnerAssessmentAttempt | undefined>
  pending: Ref<'save' | 'submit' | undefined>
  error: Ref<string | undefined>
  conflict: Ref<boolean>
  dirty: ComputedRef<boolean>
  submitted: ComputedRef<boolean>
  save: () => Promise<boolean>
  submit: () => Promise<boolean>
  reload: () => Promise<void>
  dispose: () => void
}

export function createLearnerAttemptSession(context: { courseId: string; version: string; assessmentKey: string }, client: APIClient): LearnerAttemptSession {
  const responses = ref<LearnerAssessmentAttemptResponse[]>([])
  const saved = ref<LearnerAssessmentAttemptResponse[]>([])
  const attempt = ref<LearnerAssessmentAttempt>()
  const pending = ref<'save' | 'submit'>()
  const error = ref<string>()
  const conflict = ref(false)
  let active = true
  let generation = 0
  const dirty = computed(() => JSON.stringify(responses.value) !== JSON.stringify(saved.value))
  const submitted = computed(() => attempt.value?.state === 'SUBMITTED')
  const current = () => active
  function accept(value: LearnerAssessmentAttempt) { attempt.value = value; saved.value = value.responses; if (value.state === 'SUBMITTED') responses.value = value.responses; error.value = undefined; conflict.value = false }
  async function recoverConflict() {
    if (!attempt.value) return
    const local = responses.value
    try { const value = await getLearnerAttempt(attempt.value.attemptId, client); if (current()) { accept(value); if (value.state === 'IN_PROGRESS') responses.value = local; conflict.value = true } } catch { if (current()) error.value = 'Your saved progress is unavailable. Please try again.' }
  }
  async function save(): Promise<boolean> {
    if (pending.value || submitted.value || !current()) return false
    pending.value = 'save'; error.value = undefined; const version = ++generation
    try {
      if (!attempt.value) { const created = await createLearnerAttempt(context.courseId, context.version, context.assessmentKey, client); if (!current() || version !== generation) return false; accept(created) }
      if (dirty.value && attempt.value) { const updated = await updateLearnerAttempt(attempt.value.attemptId, attempt.value.revision, responses.value, client); if (!current() || version !== generation) return false; accept(updated) }
      return current() && version === generation
    } catch (cause) { if (current() && version === generation) { if (cause instanceof APIProblemError && cause.status === 409) { await recoverConflict() } else error.value = cause instanceof APIProblemError && cause.status === 404 ? 'Progress is unavailable.' : 'Progress could not be saved. Please try again.' } return false } finally { if (current() && version === generation) pending.value = undefined }
  }
  async function submit(): Promise<boolean> {
    if (pending.value || submitted.value || !current()) return false
    if (!attempt.value || dirty.value) { const savedNow = await save(); if (!savedNow || !attempt.value) return false }
    pending.value = 'submit'; error.value = undefined; const version = ++generation
    try { const value = await submitLearnerAttempt(attempt.value.attemptId, attempt.value.revision, client); if (!current() || version !== generation) return false; accept(value); return true
    } catch (cause) { if (current() && version === generation) { if (cause instanceof APIProblemError && cause.status === 409) await recoverConflict(); else error.value = cause instanceof APIProblemError && cause.status === 400 ? 'Please complete each question before submitting.' : cause instanceof APIProblemError && cause.status === 404 ? 'Progress is unavailable.' : 'Answers could not be submitted. Please try again.' } return false
    } finally { if (current() && version === generation) pending.value = undefined }
  }
  async function reload() { if (!attempt.value || pending.value || !current()) return; const version = ++generation; try { const value = await getLearnerAttempt(attempt.value.attemptId, client); if (current() && version === generation) accept(value) } catch { if (current() && version === generation) error.value = 'Your saved progress is unavailable. Please try again.' } }
  return { responses, attempt, pending, error, conflict, dirty, submitted, save, submit, reload, dispose: () => { active = false; generation += 1 } }
}
