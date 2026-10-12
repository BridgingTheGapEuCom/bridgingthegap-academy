import { apiClient, type APIClient } from '../api/client'
import type { components } from '../api/generated'
import {
  InvalidCourseRouteError,
  InvalidPublishedCourseResponseError,
  isCourseVersion,
  isPublishedCourseID,
} from './courses'

export type LearnerCertificate = components['schemas']['LearnerCertificate']
export type PublicCertificate = components['schemas']['PublicCertificate']

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i
const publicKeys = ['certificateId', 'status', 'issuedAt', 'revokedAt', 'achievement', 'issuer']
const learnerKeys = [...publicKeys, 'verificationPath', 'openBadgePath']

export function certificatePath(id: string) {
  return `/certificates/${encodeURIComponent(id)}`
}

export function verificationPath(id: string) {
  return `/verify/certificates/${encodeURIComponent(id)}`
}

export async function issueCertificate(courseId: string, version: string, client: APIClient = apiClient): Promise<LearnerCertificate> {
  if (!isPublishedCourseID(courseId) || !isCourseVersion(version)) throw new InvalidCourseRouteError()
  return validateLearnerCertificate(await client.request<unknown>(
    `/api/courses/by-id/${encodeURIComponent(courseId)}/versions/${encodeURIComponent(version)}/certificate`,
    { method: 'POST', cache: 'no-store' },
  ))
}

export async function getLearnerCertificate(id: string, client: APIClient = apiClient): Promise<LearnerCertificate> {
  assertCertificateID(id)
  return validateLearnerCertificate(await client.request<unknown>(
    `/api/learner/certificates/${encodeURIComponent(id)}`,
    { cache: 'no-store' },
  ))
}

export async function getPublicCertificate(id: string, client: APIClient = apiClient): Promise<PublicCertificate> {
  assertCertificateID(id)
  return validatePublicCertificate(await client.request<unknown>(
    `/api/public/certificates/${encodeURIComponent(id)}`,
    { cache: 'no-store' },
  ))
}

function assertCertificateID(id: string) {
  if (!uuid.test(id)) throw new InvalidCourseRouteError()
}

function validateLearnerCertificate(value: unknown): LearnerCertificate {
  if (!validCore(value, learnerKeys)
    || typeof value.verificationPath !== 'string'
    || value.verificationPath !== verificationPath(value.certificateId)
    || (Object.hasOwn(value, 'openBadgePath') && typeof value.openBadgePath !== 'string')) {
    throw new InvalidPublishedCourseResponseError()
  }
  return value as LearnerCertificate
}

function validatePublicCertificate(value: unknown): PublicCertificate {
  if (!validCore(value, publicKeys)) throw new InvalidPublishedCourseResponseError()
  return value as PublicCertificate
}

function validCore(value: unknown, keys: string[]): value is Record<string, unknown> & {
  certificateId: string
  status: 'ACTIVE' | 'REVOKED'
  issuedAt: string
  revokedAt: string | null
} {
  if (!record(value) || !only(value, keys) || !uuid.test(String(value.certificateId))
    || (value.status !== 'ACTIVE' && value.status !== 'REVOKED') || !date(value.issuedAt)
    || !achievement(value.achievement) || !issuer(value.issuer)) return false
  return value.status === 'ACTIVE' ? value.revokedAt === null : date(value.revokedAt)
}

function achievement(value: unknown) {
  return record(value)
    && only(value, ['courseId', 'title', 'version', 'language', 'criteria'])
    && isPublishedCourseID(String(value.courseId))
    && text(value.title, 240)
    && isCourseVersion(String(value.version))
    && text(value.language, 64)
    && text(value.criteria, 4000)
}

function issuer(value: unknown) {
  return record(value) && only(value, ['id', 'name']) && text(value.id, 256) && text(value.name, 256)
}

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function only(value: Record<string, unknown>, keys: string[]) {
  return Object.keys(value).every((key) => keys.includes(key))
}

function text(value: unknown, maxLength: number) {
  return typeof value === 'string' && value.trim() === value && value.length > 0 && value.length <= maxLength
}

function date(value: unknown) {
  return typeof value === 'string' && !Number.isNaN(Date.parse(value))
}
