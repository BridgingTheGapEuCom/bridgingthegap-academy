import { APIProblemError, type APIClient } from '../api/client'
import { useAuth } from '../auth/auth'
import type { components } from '../api/generated'

export type CoursePackagePreviewResponse = components['schemas']['CoursePackagePreviewResponse']
export type CoursePackageImportResult = components['schemas']['CoursePackageImportResult']

export class InvalidCoursePackageResponseError extends Error {
  constructor() { super('Invalid course package response'); this.name = 'InvalidCoursePackageResponseError' }
}

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i
const semVer = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/

export async function previewCoursePackage(file: File, client: Pick<APIClient, 'request'> = useAuth()): Promise<CoursePackagePreviewResponse> {
  const value = await client.request<unknown>('/api/portability/imports/preview', { method: 'POST', body: file, headers: { 'Content-Type': 'application/zip' } })
  if (!isPreview(value)) throw new InvalidCoursePackageResponseError()
  return value
}

export async function executeCoursePackage(token: string, client: Pick<APIClient, 'request'> = useAuth()): Promise<CoursePackageImportResult> {
  if (!/^[0-9a-f]{64}$/i.test(token)) throw new InvalidCoursePackageResponseError()
  const value = await client.request<unknown>(`/api/portability/imports/${encodeURIComponent(token)}/execute`, { method: 'POST', body: null })
  if (!isImportResult(value)) throw new InvalidCoursePackageResponseError()
  return value
}

export type CoursePackageFailure = 'expired' | 'conflict' | 'validation' | 'unavailable'
export function classifyCoursePackageFailure(error: unknown): CoursePackageFailure {
  if (!(error instanceof APIProblemError)) return 'unavailable'
  if (error.status === 404) return 'expired'
  if (error.status === 409 && error.problem?.code === 'package_source_version_conflict') return 'conflict'
  if (error.status === 400 || error.status === 422) return 'validation'
  return 'unavailable'
}

export type CoursePackageValidationError = 'tooLarge' | 'unsupportedFormat' | 'unsupportedVersion' | 'checksum' | 'invalidCourse' | 'invalidAssessment' | 'invalidAsset' | 'invalidTranslation' | 'invalidArchive' | 'validation' | 'unavailable'
export function coursePackageValidationError(error: unknown): CoursePackageValidationError {
  if (!(error instanceof APIProblemError)) return 'unavailable'
  switch (error.problem?.code) {
    case 'package_too_large': return 'tooLarge'
    case 'unsupported_package_format': return 'unsupportedFormat'
    case 'unsupported_package_version': return 'unsupportedVersion'
    case 'checksum_mismatch': return 'checksum'
    case 'invalid_course': return 'invalidCourse'
    case 'invalid_assessment': return 'invalidAssessment'
    case 'invalid_asset': return 'invalidAsset'
    case 'invalid_translation': return 'invalidTranslation'
    case 'invalid_archive': return 'invalidArchive'
    default: return 'validation'
  }
}

function record(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }
function strings(value: unknown): value is string[] { return Array.isArray(value) && value.every((x) => typeof x === 'string' && x.length > 0) }
function isPreview(value: unknown): value is CoursePackagePreviewResponse {
  if (!record(value) || typeof value.previewToken !== 'string' || !/^[0-9a-f]{64}$/i.test(value.previewToken) || !record(value.preview)) return false
  const p = value.preview
  return typeof p.format === 'string' && typeof p.formatVersion === 'number' && Number.isSafeInteger(p.formatVersion)
    && typeof p.title === 'string' && p.title.length > 0 && typeof p.version === 'string' && semVer.test(p.version)
    && typeof p.language === 'string' && p.language.length > 0 && record(p.license) && typeof p.license.kind === 'string' && typeof p.license.displayName === 'string'
    && Array.isArray(p.attribution) && p.attribution.every((x) => record(x) && typeof x.displayName === 'string' && typeof x.role === 'string' && Number.isSafeInteger(x.order))
    && ['moduleCount', 'lessonCount', 'assessmentCount', 'assetCount', 'assetBytes'].every((key) => typeof p[key] === 'number' && Number.isSafeInteger(p[key]) && (p[key] as number) >= 0)
    && strings(p.translationLanguages)
}
function isImportResult(value: unknown): value is CoursePackageImportResult {
  return record(value) && typeof value.importId === 'string' && uuid.test(value.importId) && typeof value.courseId === 'string' && uuid.test(value.courseId)
    && typeof value.courseVersionId === 'string' && uuid.test(value.courseVersionId) && typeof value.semVer === 'string' && semVer.test(value.semVer)
    && (value.status === 'IMPORTED' || value.status === 'REPLAYED') && strings(value.translationLanguages)
}
