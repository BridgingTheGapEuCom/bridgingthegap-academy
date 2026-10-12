import { computed, ref, type ComputedRef, type Ref } from 'vue'
import { APIProblemError, type APIClient } from '../api/client'
import type { components } from '../api/generated'
import { isCourseVersion, isPublishedCourseID, InvalidCourseRouteError, InvalidPublishedCourseResponseError } from './courses'

export type LearnerCourseProgress = components['schemas']['LearnerCourseProgress']
export type LearnerProgressError = 'load' | 'save' | 'unavailable' | 'conflict'

const lessonKey = /^[a-z0-9]+(?:-[a-z0-9]+)*$/

export async function getLearnerCourseProgress(courseId: string, version: string, client: APIClient): Promise<LearnerCourseProgress> {
  validateContext(courseId, version)
  return requestProgress(client, `/api/courses/by-id/${encodeURIComponent(courseId)}/versions/${encodeURIComponent(version)}/progress`, { cache: 'no-store' })
}

export async function completeLearnerCourseLesson(courseId: string, version: string, key: string, expectedRevision: number, client: APIClient): Promise<LearnerCourseProgress> {
  validateContext(courseId, version)
  if (!lessonKey.test(key) || !Number.isSafeInteger(expectedRevision) || expectedRevision < 1) throw new InvalidCourseRouteError()
  return requestProgress(client, `/api/courses/by-id/${encodeURIComponent(courseId)}/versions/${encodeURIComponent(version)}/progress/lessons/${encodeURIComponent(key)}`, {
    method: 'PUT', cache: 'no-store', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ expectedRevision }),
  })
}

async function requestProgress(client: APIClient, path: string, options: Parameters<APIClient['request']>[1]): Promise<LearnerCourseProgress> {
  const value = await client.request<unknown>(path, options)
  if (!isLearnerCourseProgress(value)) throw new InvalidPublishedCourseResponseError()
  return value
}

function isLearnerCourseProgress(value: unknown): value is LearnerCourseProgress {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false
  const record = value as Record<string, unknown>
  return Object.keys(record).every((item) => ['revision', 'completedLessonKeys', 'updatedAt'].includes(item))
    && Number.isSafeInteger(record.revision) && (record.revision as number) > 0
    && Array.isArray(record.completedLessonKeys) && record.completedLessonKeys.every((item) => typeof item === 'string' && lessonKey.test(item))
    && new Set(record.completedLessonKeys).size === record.completedLessonKeys.length
    && typeof record.updatedAt === 'string' && !Number.isNaN(Date.parse(record.updatedAt))
}

function validateContext(courseId: string, version: string) {
  if (!isPublishedCourseID(courseId) || !isCourseVersion(version)) throw new InvalidCourseRouteError()
}

export type LearnerProgressSession = {
  progress: Ref<LearnerCourseProgress | undefined>
  pending: Ref<boolean>
  error: Ref<LearnerProgressError | undefined>
  completedCount: ComputedRef<number>
  load: () => Promise<boolean>
  complete: (key: string) => Promise<boolean>
  dispose: () => void
}

export function createLearnerProgressSession(context: { courseId: string; version: string }, client: APIClient): LearnerProgressSession {
  const progress = ref<LearnerCourseProgress>()
  const pending = ref(false)
  const error = ref<LearnerProgressError>()
  const completedCount = computed(() => progress.value?.completedLessonKeys.length ?? 0)
  let active = true
  let generation = 0
  async function load() {
    const request = ++generation
    pending.value = true
    error.value = undefined
    try {
      const value = await getLearnerCourseProgress(context.courseId, context.version, client)
      if (!active || request !== generation) return false
      progress.value = value
      return true
    } catch {
      if (active && request === generation) error.value = 'load'
      return false
    } finally {
      if (active && request === generation) pending.value = false
    }
  }
  async function complete(key: string) {
    if (!progress.value || pending.value || progress.value.completedLessonKeys.includes(key)) return Boolean(progress.value?.completedLessonKeys.includes(key))
    const request = ++generation
    pending.value = true
    error.value = undefined
    try {
      const value = await completeLearnerCourseLesson(context.courseId, context.version, key, progress.value.revision, client)
      if (!active || request !== generation) return false
      progress.value = value
      return true
    } catch (cause) {
      if (active && request === generation) error.value = cause instanceof APIProblemError && cause.status === 409 ? 'conflict' : cause instanceof APIProblemError && cause.status === 404 ? 'unavailable' : 'save'
      return false
    } finally {
      if (active && request === generation) pending.value = false
    }
  }
  return { progress, pending, error, completedCount, load, complete, dispose: () => { active = false; generation += 1 } }
}
