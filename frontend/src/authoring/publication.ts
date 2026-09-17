import { APIProblemError } from '../api/client'
import type { AuthoringPublication, PublicationValidationIssue } from './authoring'

export type AuthoringPublicationConflictCode = 'review_revision_conflict' | 'review_not_approved' | 'course_version_already_exists' | 'publication_conflict'

export type AuthoringPublicationState =
  | { kind: 'idle' }
  | { kind: 'submitting' }
  | { kind: 'success'; result: AuthoringPublication }
  | { kind: 'validation-failure'; issues: PublicationValidationIssue[] }
  | { kind: 'conflict'; code: AuthoringPublicationConflictCode | undefined }
  | { kind: 'operational-failure' }

export type AuthoringPublicationFailure = Extract<AuthoringPublicationState,
  | { kind: 'validation-failure' }
  | { kind: 'conflict' }
  | { kind: 'operational-failure' }
>

const publicationConflictCodes = new Set<NonNullable<Extract<AuthoringPublicationFailure, { kind: 'conflict' }>['code']>>([
  'review_revision_conflict',
  'review_not_approved',
  'course_version_already_exists',
  'publication_conflict',
])

function isPublicationValidationIssue(value: unknown): value is PublicationValidationIssue {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return false
  const issue = value as Record<string, unknown>
  return typeof issue.code === 'string' && issue.code.length > 0 && issue.code.length <= 128
    && typeof issue.path === 'string' && issue.path.length <= 4096
    && typeof issue.message === 'string' && issue.message.length > 0 && issue.message.length <= 20000
}

// Keep structured publication validation intact for the accessible issue UI.
// This is deliberately transport-error classification, not authorization or
// publication policy logic.
export function classifyAuthoringPublicationFailure(error: unknown): AuthoringPublicationFailure {
  if (!(error instanceof APIProblemError)) return { kind: 'operational-failure' }
  if (error.status === 422 && error.problem?.code === 'publication_validation_failed') {
    const issues = (error.problem as { issues?: unknown }).issues
    if (Array.isArray(issues) && issues.length > 0 && issues.every(isPublicationValidationIssue)) {
      return { kind: 'validation-failure', issues }
    }
  }
  if (error.status === 409) {
    const code = error.problem?.code
    return { kind: 'conflict', code: typeof code === 'string' && publicationConflictCodes.has(code as never) ? code as Extract<AuthoringPublicationFailure, { kind: 'conflict' }>['code'] : undefined }
  }
  return { kind: 'operational-failure' }
}
