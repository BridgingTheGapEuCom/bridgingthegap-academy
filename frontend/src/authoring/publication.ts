import { APIProblemError } from '../api/client'
import type { AuthoringActiveMemberList, AuthoringPublication, PublicationValidationIssue } from './authoring'

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

// This controls only whether the client offers the action. It intentionally
// mirrors the currently visible membership contract in one place and is never
// an authorization boundary; the server's authoring.publish check decides it.
export function canDisplayAuthoringPublication(members: AuthoringActiveMemberList, actorUserID: string): boolean {
  return members.members.some((member) => member.userId === actorUserID && member.role === 'MAINTAINER')
}

const publicationConflictCodes = new Set<NonNullable<Extract<AuthoringPublicationFailure, { kind: 'conflict' }>['code']>>([
  'review_revision_conflict',
  'review_not_approved',
  'course_version_already_exists',
  'publication_conflict',
])

// Keep structured publication validation intact for the accessible issue UI.
// This is deliberately transport-error classification, not authorization or
// publication policy logic.
export function classifyAuthoringPublicationFailure(error: unknown): AuthoringPublicationFailure {
  if (!(error instanceof APIProblemError)) return { kind: 'operational-failure' }
  if (error.status === 422 && error.problem?.code === 'publication_validation_failed') {
    const issues = (error.problem as { issues?: unknown }).issues
    if (Array.isArray(issues)) return { kind: 'validation-failure', issues: issues as PublicationValidationIssue[] }
  }
  if (error.status === 409) {
    const code = error.problem?.code
    return { kind: 'conflict', code: typeof code === 'string' && publicationConflictCodes.has(code as never) ? code as Extract<AuthoringPublicationFailure, { kind: 'conflict' }>['code'] : undefined }
  }
  return { kind: 'operational-failure' }
}
