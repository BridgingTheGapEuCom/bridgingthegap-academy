import { apiClient, type APIClient } from '../api/client'
import type { components } from '../api/generated'

export type CourseList = components['schemas']['CourseList']
export type CourseSummary = components['schemas']['CourseSummary']
export type CourseDetail = components['schemas']['CourseDetail']
export type LessonDetail = components['schemas']['LessonDetail']
export type CourseVersionSummary = components['schemas']['CourseVersionSummary']
export type LessonSummary = components['schemas']['LessonSummary']
export type PublishedCourseCatalogPage = components['schemas']['PublishedCourseCatalogPage']
export type PublishedCourseCatalogItem = components['schemas']['PublishedCourseCatalogItem']
export type PublishedCourseVersionDetail = components['schemas']['PublishedCourseVersionDetail']
export type PublishedAssessmentLearnerView = components['schemas']['PublishedAssessmentLearnerView']

export const publishedCatalogPageSize = 20
export const maxPublishedCatalogOffset = 2_147_483_647

const courseSlugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/
const courseVersionPattern = /^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$/
const courseIDPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i

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

export class InvalidCourseRouteError extends Error {
  constructor() {
    super('Invalid course route')
    this.name = 'InvalidCourseRouteError'
  }
}

export class InvalidPublishedCourseResponseError extends Error {
  constructor() {
    super('Invalid published course response')
    this.name = 'InvalidPublishedCourseResponseError'
  }
}

export function isCourseVersion(version: string): boolean {
  return version.length <= 29 && courseVersionPattern.test(version)
}

export function isLessonKey(key: string): boolean {
  return isCourseSlug(key)
}

export function isPublishedCourseID(courseID: string): boolean {
  return courseIDPattern.test(courseID)
}

export async function listCourses(client: APIClient = apiClient): Promise<CourseList> {
  return client.request<CourseList>('/api/courses')
}

export type PublishedCatalogQuery = {
  limit?: number
  offset?: number
  language?: string
}

export async function listPublishedCourseCatalog(query: PublishedCatalogQuery = {}, client: APIClient = apiClient): Promise<PublishedCourseCatalogPage> {
  const limit = query.limit ?? publishedCatalogPageSize
  const offset = query.offset ?? 0
  if (!Number.isSafeInteger(limit) || limit < 1 || limit > 100 || !Number.isSafeInteger(offset) || offset < 0 || offset > maxPublishedCatalogOffset) {
    throw new InvalidCourseRouteError()
  }
  const parameters = new URLSearchParams({ limit: String(limit), offset: String(offset) })
  if (query.language) parameters.set('language', query.language)
  const page = await client.request<unknown>(`/api/courses/catalog?${parameters.toString()}`, { cache: 'no-store' })
  if (!isPublishedCourseCatalogPage(page, limit, offset)) throw new InvalidPublishedCourseResponseError()
  return page
}

export function publishedCoursePath(courseID: string): string {
  return `/courses/by-id/${encodeURIComponent(courseID)}`
}

export function publishedCourseVersionPath(courseID: string, version: string): string {
  return `/courses/by-id/${encodeURIComponent(courseID)}/versions/${encodeURIComponent(version)}`
}

export async function getLatestPublishedCourse(courseID: string, client: APIClient = apiClient): Promise<PublishedCourseVersionDetail> {
  if (!isPublishedCourseID(courseID)) throw new InvalidCourseRouteError()
  const course = await client.request<unknown>(`/api/courses/by-id/${encodeURIComponent(courseID)}/latest`, { cache: 'no-store' })
  if (!isPublishedCourseVersionDetail(course)) throw new InvalidPublishedCourseResponseError()
  return course
}

export async function getPublishedCourseVersionByID(courseID: string, version: string, client: APIClient = apiClient): Promise<PublishedCourseVersionDetail> {
  if (!isPublishedCourseID(courseID) || !isCourseVersion(version)) throw new InvalidCourseRouteError()
  const course = await client.request<unknown>(`/api/courses/by-id/${encodeURIComponent(courseID)}/versions/${encodeURIComponent(version)}`, { cache: 'no-store' })
  if (!isPublishedCourseVersionDetail(course)) throw new InvalidPublishedCourseResponseError()
  return course
}

export function publishedLessonKeyFromRoute(value: unknown): string | undefined {
  return typeof value === 'string' && isLessonKey(value) ? value : undefined
}

export function catalogOffsetFromRoute(value: unknown): number | undefined {
  if (value === undefined) return 0
  if (typeof value !== 'string' || !/^(0|[1-9][0-9]*)$/.test(value)) return undefined
  const offset = Number(value)
  return Number.isSafeInteger(offset) && offset <= maxPublishedCatalogOffset ? offset : undefined
}

export async function getCourse(slug: string, client: APIClient = apiClient): Promise<CourseDetail> {
  if (!isCourseSlug(slug)) throw new InvalidCourseSlugError()
  return client.request<CourseDetail>(`/api/courses/${encodeURIComponent(slug)}`)
}

export async function getCourseVersion(slug: string, version: string, client: APIClient = apiClient): Promise<CourseDetail> {
  if (!isCourseSlug(slug) || !isCourseVersion(version)) throw new InvalidCourseRouteError()
  return client.request<CourseDetail>(`/api/courses/${encodeURIComponent(slug)}/versions/${encodeURIComponent(version)}`)
}

export async function getLesson(slug: string, version: string, lessonKey: string, client: APIClient = apiClient): Promise<LessonDetail> {
  if (!isCourseSlug(slug) || !isCourseVersion(version) || !isLessonKey(lessonKey)) throw new InvalidCourseRouteError()
  return client.request<LessonDetail>(`/api/courses/${encodeURIComponent(slug)}/versions/${encodeURIComponent(version)}/lessons/${encodeURIComponent(lessonKey)}`)
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

function isPublishedCourseCatalogPage(value: unknown, limit: number, offset: number): value is PublishedCourseCatalogPage {
  if (!isRecord(value)
    || value.limit !== limit
    || value.offset !== offset
    || !isNonNegativeInteger(value.total)
    || !Array.isArray(value.items)
    || value.items.length > limit) return false
  return value.items.every(isPublishedCourseCatalogItem)
}

function isPublishedCourseCatalogItem(value: unknown): value is PublishedCourseCatalogItem {
  return isRecord(value)
    && typeof value.courseId === 'string'
    && isPublishedCourseID(value.courseId)
    && typeof value.version === 'string'
    && isCourseVersion(value.version)
    && typeof value.title === 'string'
    && typeof value.description === 'string'
    && typeof value.sourceLanguage === 'string'
    && isPublishedContentLicense(value.license)
    && isPublishedContributors(value.contributors)
    && isDateTime(value.publishedAt)
}

function isPublishedCourseVersionDetail(value: unknown): value is PublishedCourseVersionDetail {
  if (!isRecord(value)
    || typeof value.courseId !== 'string'
    || !isPublishedCourseID(value.courseId)
    || typeof value.version !== 'string'
    || !isCourseVersion(value.version)
    || typeof value.title !== 'string'
    || typeof value.description !== 'string'
    || !isStringArray(value.objectives)
    || typeof value.sourceLanguage !== 'string'
    || typeof value.changelog !== 'string'
    || !isPublishedContentLicense(value.license)
    || !isPublishedContributors(value.contributors)
    || !isDateTime(value.publishedAt)
    || !Array.isArray(value.modules)
    || !isPublishedAssessmentLearnerViews(value.assessments)) return false

  const moduleKeys = new Set<string>()
  const lessonKeys = new Set<string>()
  return value.modules.every((module, moduleIndex) => {
    if (!isRecord(module)
      || typeof module.stableKey !== 'string'
      || !isLessonKey(module.stableKey)
      || moduleKeys.has(module.stableKey)
      || typeof module.title !== 'string'
      || typeof module.description !== 'string'
      || module.position !== moduleIndex
      || !Array.isArray(module.lessons)) return false
    moduleKeys.add(module.stableKey)
    return module.lessons.every((lesson, lessonIndex) => {
      if (!isRecord(lesson)
        || typeof lesson.stableKey !== 'string'
        || !isLessonKey(lesson.stableKey)
        || lessonKeys.has(lesson.stableKey)
        || typeof lesson.title !== 'string'
        || typeof lesson.description !== 'string'
        || !isStringArray(lesson.objectives)
        || !(lesson.estimatedDurationMinutes === null || (isNonNegativeInteger(lesson.estimatedDurationMinutes) && lesson.estimatedDurationMinutes > 0))
        || lesson.position !== lessonIndex
        || !isStringArray(lesson.prerequisiteStableKeys)
        || !Object.prototype.hasOwnProperty.call(lesson, 'content')) return false
      lessonKeys.add(lesson.stableKey)
      return true
    })
  })
}

function isPublishedAssessmentLearnerViews(value: unknown): value is PublishedAssessmentLearnerView[] {
  if (!Array.isArray(value)) return false
  const assessmentKeys = new Set<string>()
  return value.every((assessment) => {
    if (!hasOnlyKeys(assessment, ['assessmentKey', 'questions'])
      || typeof assessment.assessmentKey !== 'string'
      || !isPublishedCourseID(assessment.assessmentKey)
      || assessmentKeys.has(assessment.assessmentKey)
      || !Array.isArray(assessment.questions)) return false
    assessmentKeys.add(assessment.assessmentKey)
    const questionKeys = new Set<string>()
    return assessment.questions.every((question, position) => {
      if (!hasOnlyKeys(question, ['stableKey', 'type', 'prompt', 'position', 'options', 'leftItems', 'rightItems'])
        || !isAssessmentStableKey(question.stableKey)
        || questionKeys.has(question.stableKey)
        || (question.type !== 'SINGLE_CHOICE' && question.type !== 'MULTIPLE_CHOICE' && question.type !== 'MATCHING')
        || !isAssessmentText(question.prompt, 10000)
        || question.position !== position
        || !Array.isArray(question.options)
        || !Array.isArray(question.leftItems)
        || !Array.isArray(question.rightItems)) return false
      questionKeys.add(question.stableKey)
      if (question.type === 'MATCHING') {
        return question.options.length === 0
          && question.leftItems.length > 0
          && question.leftItems.length === question.rightItems.length
          && isPublishedAssessmentItems(question.leftItems)
          && isPublishedAssessmentItems(question.rightItems)
      }
      return question.leftItems.length === 0
        && question.rightItems.length === 0
        && question.options.length >= 2
        && isPublishedAssessmentOptions(question.options)
    })
  })
}

function isPublishedAssessmentOptions(value: unknown[]): boolean {
  const keys = new Set<string>()
  return value.every((option, position) => {
    if (!hasOnlyKeys(option, ['stableKey', 'text', 'position'])
      || !isAssessmentStableKey(option.stableKey)
      || !isAssessmentText(option.text, 4000)
      || option.position !== position) return false
    if (keys.has(option.stableKey)) return false
    keys.add(option.stableKey)
    return true
  })
}

function isPublishedAssessmentItems(value: unknown[]): boolean {
  const keys = new Set<string>()
  return value.every((item, position) => {
    if (!hasOnlyKeys(item, ['stableKey', 'text', 'position'])
      || !isAssessmentStableKey(item.stableKey)
      || !isAssessmentText(item.text, 4000)
      || item.position !== position) return false
    if (keys.has(item.stableKey)) return false
    keys.add(item.stableKey)
    return true
  })
}

function isAssessmentStableKey(value: unknown): value is string {
  return typeof value === 'string' && value.length >= 1 && value.length <= 160 && /^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(value)
}

function isAssessmentText(value: unknown, maximum: number): value is string {
  return typeof value === 'string' && value.length > 0 && value.length <= maximum && value.trim() === value
}

function isPublishedContentLicense(value: unknown): value is PublishedCourseCatalogItem['license'] {
  return isRecord(value)
    && (value.kind === 'STANDARD' || value.kind === 'ALL_RIGHTS_RESERVED' || value.kind === 'CUSTOM')
    && typeof value.identifier === 'string'
    && typeof value.displayName === 'string'
    && typeof value.url === 'string'
    && typeof value.customText === 'string'
}

function isPublishedContributors(value: unknown): value is PublishedCourseCatalogItem['contributors'] {
  return Array.isArray(value) && value.every((contributor) => isRecord(contributor)
    && typeof contributor.displayName === 'string'
    && (contributor.role === 'AUTHOR' || contributor.role === 'MAINTAINER')
    && isNonNegativeInteger(contributor.order))
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function hasOnlyKeys(value: unknown, keys: string[]): value is Record<string, unknown> {
  return isRecord(value) && Object.keys(value).every((key) => keys.includes(key))
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === 'string')
}

function isNonNegativeInteger(value: unknown): value is number {
  return Number.isSafeInteger(value) && (value as number) >= 0
}

function isDateTime(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && !Number.isNaN(Date.parse(value))
}
