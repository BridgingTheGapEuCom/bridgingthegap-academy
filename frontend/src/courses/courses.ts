import { apiClient, type APIClient } from '../api/client'
import type { components } from '../api/generated'

export type CourseList = components['schemas']['CourseList']
export type CourseSummary = components['schemas']['CourseSummary']
export type CourseDetail = components['schemas']['CourseDetail']
export type CourseVersionSummary = components['schemas']['CourseVersionSummary']
export type LessonSummary = components['schemas']['LessonSummary']

const courseSlugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/

// This mirrors the public API's conservative slug contract so malformed route
// values never become arbitrary or unbounded API paths.
export function isCourseSlug(slug: string): boolean {
  return slug.length >= 3 && slug.length <= 96 && courseSlugPattern.test(slug)
}

export class InvalidCourseSlugError extends Error {
  constructor() {
    super('Invalid course slug')
    this.name = 'InvalidCourseSlugError'
  }
}

export async function listCourses(client: APIClient = apiClient): Promise<CourseList> {
  return client.request<CourseList>('/api/courses')
}

export async function getCourse(slug: string, client: APIClient = apiClient): Promise<CourseDetail> {
  if (!isCourseSlug(slug)) throw new InvalidCourseSlugError()
  return client.request<CourseDetail>(`/api/courses/${encodeURIComponent(slug)}`)
}

export function lessonPath(slug: string, version: string, lessonKey: string): string {
  return `/courses/${encodeURIComponent(slug)}/versions/${encodeURIComponent(version)}/lessons/${encodeURIComponent(lessonKey)}`
}

export function formatDuration(minutes: number | null | undefined): string | undefined {
  if (!Number.isSafeInteger(minutes) || !minutes || minutes < 1) return undefined
  const hours = Math.floor(minutes / 60)
  const remainingMinutes = minutes % 60
  if (hours === 0) return `${minutes} min`
  if (remainingMinutes === 0) return `${hours} hr`
  return `${hours} hr ${remainingMinutes} min`
}
