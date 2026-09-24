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

export function packageErrorMessage(error: unknown): string {
  if (!(error instanceof APIProblemError)) return 'We couldn’t complete this request right now. Please try again.'
  switch (error.problem?.code) {
    case 'package_too_large': return 'This package is too large for this Academy installation.'
    case 'unsupported_package_format': return 'This is not a supported course package.'
    case 'unsupported_package_version': return 'This package format version is not supported by this Academy installation.'
    case 'checksum_mismatch': return 'This package appears to be damaged or changed.'
    case 'invalid_course': return 'The package contains invalid course content.'
    case 'invalid_assessment': return 'The package contains an invalid assessment.'
    case 'invalid_asset': return 'The package contains an invalid asset.'
    case 'invalid_translation': return 'The package contains an invalid translation.'
    case 'invalid_archive': return 'This file is not a valid course package archive.'
    default: return 'We couldn’t validate this package. Choose a course package and try again.'
  }
}

export function formatBytes(value: number): string {
  if (!Number.isSafeInteger(value) || value < 0) return 'Unknown size'
  if (value < 1024) return `${value} bytes`
  const units = ['KB', 'MB', 'GB']
  let amount = value / 1024
  let unit = 0
  while (amount >= 1024 && unit < units.length - 1) { amount /= 1024; unit += 1 }
  return `${amount.toFixed(amount >= 10 ? 0 : 1)} ${units[unit]}`
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
