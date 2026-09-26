import { useAuth, type AuthService } from '../auth/auth'
import type { components } from '../api/generated'

export type AuthoringDraft = components['schemas']['AuthoringDraft']
export type AuthoringDraftCreate = components['schemas']['AuthoringDraftCreateRequest']
export type AuthoringDraftSummary = components['schemas']['AuthoringDraftSummary']
export type AuthoringDraftList = components['schemas']['AuthoringDraftList']
export type AuthoringContentLicense = components['schemas']['ContentLicense']
export type AuthoringStructure = components['schemas']['AuthoringStructure']
export type AuthoringModuleSummary = components['schemas']['AuthoringModuleSummary']
export type AuthoringLessonSummary = components['schemas']['AuthoringLessonSummary']
export type AuthoringLessonDetail = components['schemas']['AuthoringLessonDetail']
export type AuthoringLessonContent = components['schemas']['LessonContent']
export type AuthoringLessonContentMutation = components['schemas']['AuthoringLessonContentMutationResponse']
export type AuthoringAsset = components['schemas']['AuthoringAsset']
export type AuthoringAssetSummary = components['schemas']['AuthoringAssetSummary']
export type AuthoringAssetList = components['schemas']['AuthoringAssetList']
// These are deliberately Authoring-only representations. Detail responses
// contain answer definitions so an authorized author can edit them; learner
// code must never use this type or helper.
export type AuthoringAssessmentOption = components['schemas']['AuthoringAssessmentOption']
export type AuthoringAssessmentMatchingItem = components['schemas']['AuthoringAssessmentMatchingItem']
export type AuthoringAssessmentMatchingPair = components['schemas']['AuthoringAssessmentMatchingPair']
export type AuthoringAssessmentQuestion = components['schemas']['AuthoringAssessmentQuestion']
export type AuthoringAssessmentCreate = components['schemas']['AuthoringAssessmentCreateRequest']
export type AuthoringAssessmentUpdate = components['schemas']['AuthoringAssessmentUpdateRequest']
export type AuthoringAssessmentDetail = components['schemas']['AuthoringAssessmentDetail']
export type AuthoringAssessmentSummary = components['schemas']['AuthoringAssessmentSummary']
export type AuthoringAssessmentList = components['schemas']['AuthoringAssessmentList']
export type AuthoringActiveMember = components['schemas']['AuthoringActiveMember']
export type AuthoringActiveMemberList = components['schemas']['AuthoringActiveMemberList']
export type AuthoringMemberAdd = components['schemas']['AuthoringMemberAddRequest']
export type AuthoringMemberRoleChange = components['schemas']['AuthoringMemberRoleRequest']
export type AuthoringMemberMutation = components['schemas']['AuthoringMemberMutationResponse']
export type AuthoringReview = components['schemas']['AuthoringReview']
export type AuthoringReviewList = components['schemas']['AuthoringReviewList']
export type AuthoringReviewDetail = components['schemas']['AuthoringReviewDetail']
export type AuthoringReviewSubmissionDetail = components['schemas']['AuthoringReviewSubmissionDetail']
export type AuthoringReviewPublicationStatus = components['schemas']['AuthoringReviewPublicationStatus']
export type AuthoringReviewSubmit = components['schemas']['AuthoringReviewSubmitRequest']
export type AuthoringReviewDecision = components['schemas']['AuthoringReviewDecisionRequest']
export type AuthoringReviewSnapshot = components['schemas']['AuthoringReviewSnapshot']
export type AuthoringReviewSnapshotModule = components['schemas']['AuthoringReviewSnapshotModule']
export type AuthoringReviewSnapshotLesson = components['schemas']['AuthoringReviewSnapshotLesson']
export type AuthoringPublicationRequest = components['schemas']['AuthoringPublicationRequest']
export type AuthoringPublication = components['schemas']['AuthoringPublication']
export type PublicationValidationIssue = components['schemas']['PublicationValidationIssue']
export type AuthoringCourseWidget = components['schemas']['AuthoringCourseWidget']
export type AuthoringCourseWidgetList = components['schemas']['AuthoringCourseWidgetList']

export type AuthoringModuleCreate = components['schemas']['AuthoringModuleCreateRequest']
export type AuthoringModuleUpdate = { expectedModuleRevision: number; title?: string; description?: string }
export type AuthoringModuleMutation = components['schemas']['AuthoringModuleMutationResponse']
export type AuthoringOrderResponse = components['schemas']['AuthoringModuleOrderResponse']
export type AuthoringLessonCreate = components['schemas']['AuthoringLessonCreateRequest']
export type AuthoringLessonOrderModule = components['schemas']['AuthoringLessonOrderModule']
export type AuthoringLessonMetadataPatch = {
  expectedLessonRevision: number
  title?: string
  description?: string
  objectives?: string[]
  estimatedDurationMinutes?: number | null
}
export type AuthoringLessonPrerequisitesPatch = components['schemas']['AuthoringLessonPrerequisitesRequest']

export type AuthoringDraftMetadataPatch = {
  expectedRevision: number
  intendedVersion?: string
  sourceLanguage?: string
  title?: string
  description?: string
  objectives?: string[]
  changelog?: string
  license?: AuthoringContentLicense
}

const draftIDPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/

export class InvalidAuthoringDraftIDError extends Error {
  constructor() {
    super('Invalid Authoring draft ID')
    this.name = 'InvalidAuthoringDraftIDError'
  }
}

export class InvalidAuthoringDraftListResponseError extends Error {
  constructor() {
    super('Invalid Authoring draft list response')
    this.name = 'InvalidAuthoringDraftListResponseError'
  }
}

export class InvalidAuthoringDraftResponseError extends Error {
  constructor() {
    super('Invalid Authoring draft response')
    this.name = 'InvalidAuthoringDraftResponseError'
  }
}

export class InvalidAuthoringReviewResponseError extends Error {
  constructor() {
    super('Invalid Authoring Review response')
    this.name = 'InvalidAuthoringReviewResponseError'
  }
}

export class InvalidAuthoringPublicationResponseError extends Error {
  constructor() {
    super('Invalid Authoring publication response')
    this.name = 'InvalidAuthoringPublicationResponseError'
  }
}

export class InvalidAuthoringAssetResponseError extends Error {
  constructor() {
    super('Invalid Authoring Asset response')
    this.name = 'InvalidAuthoringAssetResponseError'
  }
}

export class InvalidAuthoringAssessmentResponseError extends Error {
  constructor() {
    super('Invalid Authoring Assessment response')
    this.name = 'InvalidAuthoringAssessmentResponseError'
  }
}

// The route value is bounded before it becomes an API path. Authorization is
// deliberately not decided here: the private API remains the authority.
export function isAuthoringDraftID(value: string): boolean {
  return value.length === 36 && draftIDPattern.test(value)
}

function assertAuthoringID(value: string): void {
  if (!isAuthoringDraftID(value)) throw new InvalidAuthoringDraftIDError()
}

// Discovery remains server-authoritative: the private endpoint filters the
// current actor's active memberships and never accepts browser-side roles.
export async function listAuthoringDrafts(client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringDraftList> {
  const response = await client.request<unknown>('/api/authoring/drafts', { cache: 'no-store' })
  if (!isAuthoringDraftList(response)) throw new InvalidAuthoringDraftListResponseError()
  return response
}

// Creation accepts only Draft metadata. The authenticated API session supplies
// creator identity and CSRF; the server owns Course identity and membership.
export async function createAuthoringDraft(input: AuthoringDraftCreate, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringDraft> {
  const response = await client.request<unknown>('/api/authoring/drafts', {
    ...jsonRequest('POST', input),
    cache: 'no-store',
  })
  if (!isAuthoringDraft(response)) throw new InvalidAuthoringDraftResponseError()
  return response
}

export async function getAuthoringDraft(draftID: string, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringDraft> {
  if (!isAuthoringDraftID(draftID)) throw new InvalidAuthoringDraftIDError()
  return client.request<AuthoringDraft>(`/api/authoring/drafts/${encodeURIComponent(draftID)}`)
}

export async function updateAuthoringDraft(draftID: string, patch: AuthoringDraftMetadataPatch, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringDraft> {
  assertAuthoringID(draftID)
  return client.request<AuthoringDraft>(`/api/authoring/drafts/${encodeURIComponent(draftID)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  })
}

export async function getAuthoringStructure(draftID: string, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringStructure> {
  assertAuthoringID(draftID)
  return client.request<AuthoringStructure>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/structure`)
}

// Membership remains an Authoring-only, opaque identity projection. The API
// owns authorization and active/revoked filtering; callers never derive it
// from mutation responses or browser-side role assumptions.
export async function getAuthoringDraftMembers(draftID: string, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringActiveMemberList> {
  assertAuthoringID(draftID)
  return client.request<AuthoringActiveMemberList>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/members`)
}

// Review overview reads only immutable cycle metadata. Exact snapshots remain
// separate and are fetched only by the explicit Draft-scoped detail route.
export async function getAuthoringDraftReviewHistory(draftID: string, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringReviewList> {
  assertAuthoringID(draftID)
  return client.request<AuthoringReviewList>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/reviews`)
}

export async function getAuthoringActiveDraftReview(draftID: string, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringReview> {
  assertAuthoringID(draftID)
  return client.request<AuthoringReview>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/reviews/active`)
}

export async function getAuthoringLatestDraftReview(draftID: string, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringReview> {
  assertAuthoringID(draftID)
  return client.request<AuthoringReview>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/reviews/latest`)
}

// A Review snapshot is historical evidence. It is read only through both its
// enclosing Draft and its exact Review ID; it is never reconstructed from the
// current mutable Draft.
export async function getAuthoringDraftReview(draftID: string, reviewID: string, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringReviewDetail> {
  assertAuthoringID(draftID)
  assertAuthoringID(reviewID)
  const response = await client.request<unknown>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/reviews/${encodeURIComponent(reviewID)}`, { cache: 'no-store' })
  if (!isAuthoringReviewDetail(response)) throw new InvalidAuthoringReviewResponseError()
  return response
}

// The server freezes the canonical snapshot from the authoritative Draft. The
// frontend sends only the Draft revision it intends to submit and does not use
// the snapshot response until a later explicit Review-detail experience.
export async function submitAuthoringDraftReview(draftID: string, input: AuthoringReviewSubmit, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringReviewSubmissionDetail> {
  assertAuthoringID(draftID)
  return client.request<AuthoringReviewSubmissionDetail>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/reviews`, jsonRequest('POST', input))
}

// Decisions are scoped by both the Draft and the immutable Review cycle. The
// only browser-supplied concurrency value is the authoritative Review revision.
export async function approveAuthoringReview(draftID: string, reviewID: string, input: AuthoringReviewDecision, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringReview> {
  assertAuthoringID(draftID)
  assertAuthoringID(reviewID)
  return client.request<AuthoringReview>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/reviews/${encodeURIComponent(reviewID)}/approve`, jsonRequest('POST', input))
}

export async function requestAuthoringReviewChanges(draftID: string, reviewID: string, input: AuthoringReviewDecision, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringReview> {
  assertAuthoringID(draftID)
  assertAuthoringID(reviewID)
  return client.request<AuthoringReview>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/reviews/${encodeURIComponent(reviewID)}/request-changes`, jsonRequest('POST', input))
}

// Publishing uses the immutable Review's own revision for CAS. The server
// derives publisher attribution, time, and the CourseVersion from its frozen
// snapshot; none of those values are browser input.
export async function publishAuthoringDraftReview(draftID: string, reviewID: string, input: AuthoringPublicationRequest, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringPublication> {
  assertAuthoringID(draftID)
  assertAuthoringID(reviewID)
  const response = await client.request<unknown>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/reviews/${encodeURIComponent(reviewID)}/publish`, {
    ...jsonRequest('POST', input),
    cache: 'no-store',
  })
  // A successful response is still untrusted network data. It must describe
  // the exact Review and revision the user selected; publication metadata is
  // otherwise rendered only after the authoritative Review read is refreshed.
  if (!isAuthoringPublication(response)
    || response.reviewId !== reviewID
    || response.reviewRevision !== input.expectedReviewRevision) {
    throw new InvalidAuthoringPublicationResponseError()
  }
  return response
}

export async function addAuthoringMember(draftID: string, input: AuthoringMemberAdd, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringMemberMutation> {
  assertAuthoringID(draftID)
  assertAuthoringID(input.userId)
  return client.request<AuthoringMemberMutation>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/members`, jsonRequest('POST', input))
}

export async function changeAuthoringMemberRole(draftID: string, userID: string, input: AuthoringMemberRoleChange, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringMemberMutation> {
  assertAuthoringID(draftID); assertAuthoringID(userID)
  return client.request<AuthoringMemberMutation>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/members/${encodeURIComponent(userID)}`, jsonRequest('PATCH', input))
}

export async function revokeAuthoringMember(draftID: string, userID: string, expectedDraftRevision: number, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringMemberMutation> {
  assertAuthoringID(draftID); assertAuthoringID(userID)
  return client.request<AuthoringMemberMutation>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/members/${encodeURIComponent(userID)}`, jsonRequest('DELETE', { expectedDraftRevision }))
}

export async function createAuthoringModule(draftID: string, input: AuthoringModuleCreate, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringModuleMutation> {
  assertAuthoringID(draftID)
  return client.request<AuthoringModuleMutation>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/modules`, jsonRequest('POST', input))
}

export async function updateAuthoringModule(draftID: string, moduleID: string, input: AuthoringModuleUpdate, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringModuleMutation> {
  assertAuthoringID(draftID); assertAuthoringID(moduleID)
  return client.request<AuthoringModuleMutation>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/modules/${encodeURIComponent(moduleID)}`, jsonRequest('PATCH', input))
}

export async function reorderAuthoringModules(draftID: string, expectedDraftRevision: number, moduleIDs: string[], client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringOrderResponse> {
  assertAuthoringID(draftID); moduleIDs.forEach(assertAuthoringID)
  return client.request<AuthoringOrderResponse>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/modules/order`, jsonRequest('PUT', { expectedDraftRevision, moduleIds: moduleIDs }))
}

export async function deleteAuthoringModule(draftID: string, moduleID: string, expectedDraftRevision: number, expectedModuleRevision: number, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringOrderResponse> {
  assertAuthoringID(draftID); assertAuthoringID(moduleID)
  return client.request<AuthoringOrderResponse>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/modules/${encodeURIComponent(moduleID)}`, jsonRequest('DELETE', { expectedDraftRevision, expectedModuleRevision }))
}

export async function createAuthoringLesson(draftID: string, moduleID: string, input: AuthoringLessonCreate, client: Pick<AuthService, 'request'> = useAuth()): Promise<components['schemas']['AuthoringLessonMutationResponse']> {
  assertAuthoringID(draftID); assertAuthoringID(moduleID)
  return client.request<components['schemas']['AuthoringLessonMutationResponse']>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/modules/${encodeURIComponent(moduleID)}/lessons`, jsonRequest('POST', input))
}

export async function reorderAuthoringLessons(draftID: string, expectedDraftRevision: number, modules: AuthoringLessonOrderModule[], client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringOrderResponse> {
  assertAuthoringID(draftID)
  modules.forEach(({ moduleId, lessonIds }) => { assertAuthoringID(moduleId); lessonIds.forEach(assertAuthoringID) })
  return client.request<AuthoringOrderResponse>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/lessons/order`, jsonRequest('PUT', { expectedDraftRevision, modules }))
}

export async function deleteAuthoringLesson(draftID: string, lessonID: string, expectedDraftRevision: number, expectedLessonRevision: number, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringOrderResponse> {
  assertAuthoringID(draftID); assertAuthoringID(lessonID)
  return client.request<AuthoringOrderResponse>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/lessons/${encodeURIComponent(lessonID)}`, jsonRequest('DELETE', { expectedDraftRevision, expectedLessonRevision }))
}

export async function getAuthoringLesson(draftID: string, lessonID: string, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringLessonDetail> {
  assertAuthoringID(draftID); assertAuthoringID(lessonID)
  return client.request<AuthoringLessonDetail>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/lessons/${encodeURIComponent(lessonID)}`)
}

export async function updateAuthoringLesson(draftID: string, lessonID: string, input: AuthoringLessonMetadataPatch, client: Pick<AuthService, 'request'> = useAuth()): Promise<components['schemas']['AuthoringLessonMutationResponse']> {
  assertAuthoringID(draftID); assertAuthoringID(lessonID)
  return client.request<components['schemas']['AuthoringLessonMutationResponse']>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/lessons/${encodeURIComponent(lessonID)}`, jsonRequest('PATCH', input))
}

export async function replaceAuthoringLessonPrerequisites(draftID: string, lessonID: string, input: AuthoringLessonPrerequisitesPatch, client: Pick<AuthService, 'request'> = useAuth()): Promise<components['schemas']['AuthoringLessonMutationResponse']> {
  assertAuthoringID(draftID); assertAuthoringID(lessonID)
  return client.request<components['schemas']['AuthoringLessonMutationResponse']>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/lessons/${encodeURIComponent(lessonID)}/prerequisites`, jsonRequest('PUT', input))
}

export async function replaceAuthoringLessonContent(draftID: string, lessonID: string, input: components['schemas']['AuthoringLessonContentUpdateRequest'], client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringLessonContentMutation> {
  assertAuthoringID(draftID); assertAuthoringID(lessonID)
  return client.request<AuthoringLessonContentMutation>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/lessons/${encodeURIComponent(lessonID)}/content`, jsonRequest('PUT', input))
}

export async function listAuthoringCourseWidgets(draftID: string, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringCourseWidgetList> {
  assertAuthoringID(draftID)
  return client.request<AuthoringCourseWidgetList>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/plugins/course-widgets`, { cache: 'no-store' })
}

// The server owns asset identity, ownership, MIME detection, integrity, and
// lifecycle. This client deliberately sends one file part and no metadata.
export async function uploadAuthoringAsset(draftID: string, file: File, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringAsset> {
  assertAuthoringID(draftID)
  const body = new FormData()
  body.append('file', file)
  const response = await client.request<unknown>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/assets`, {
    method: 'POST', body, cache: 'no-store',
  })
  if (!isAuthoringAsset(response)) throw new InvalidAuthoringAssetResponseError()
  return response
}
export async function listAuthoringDraftAssets(draftID: string, limit = 20, offset = 0, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringAssetList> {
  assertAuthoringID(draftID)
  if (!Number.isSafeInteger(limit) || limit < 1 || limit > 100 || !Number.isSafeInteger(offset) || offset < 0) throw new InvalidAuthoringAssetResponseError()
  const response = await client.request<unknown>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/assets?limit=${limit}&offset=${offset}`, { cache: 'no-store' })
  if (!isAuthoringAssetList(response)) throw new InvalidAuthoringAssetResponseError()
  return response
}

export async function listAuthoringDraftAssessments(draftID: string, limit = 20, offset = 0, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringAssessmentList> {
  assertAuthoringID(draftID)
  if (!isPage(limit, offset)) throw new InvalidAuthoringAssessmentResponseError()
  const response = await client.request<unknown>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/assessments?limit=${limit}&offset=${offset}`, { cache: 'no-store' })
  if (!isAuthoringAssessmentList(response)) throw new InvalidAuthoringAssessmentResponseError()
  return response
}

export async function createAuthoringAssessment(draftID: string, input: AuthoringAssessmentCreate, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringAssessmentDetail> {
  assertAuthoringID(draftID)
  const response = await client.request<unknown>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/assessments`, { ...jsonRequest('POST', input), cache: 'no-store' })
  if (!isAuthoringAssessmentDetail(response)) throw new InvalidAuthoringAssessmentResponseError()
  return response
}

export async function getAuthoringAssessment(draftID: string, assessmentID: string, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringAssessmentDetail> {
  assertAuthoringID(draftID); assertAuthoringID(assessmentID)
  const response = await client.request<unknown>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/assessments/${encodeURIComponent(assessmentID)}`, { cache: 'no-store' })
  if (!isAuthoringAssessmentDetail(response) || response.assessmentKey !== assessmentID) throw new InvalidAuthoringAssessmentResponseError()
  return response
}

export async function replaceAuthoringAssessment(draftID: string, assessmentID: string, input: AuthoringAssessmentUpdate, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringAssessmentDetail> {
  assertAuthoringID(draftID); assertAuthoringID(assessmentID)
  const response = await client.request<unknown>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/assessments/${encodeURIComponent(assessmentID)}`, { ...jsonRequest('PUT', input), cache: 'no-store' })
  if (!isAuthoringAssessmentDetail(response) || response.assessmentKey !== assessmentID || response.revision <= input.expectedRevision) throw new InvalidAuthoringAssessmentResponseError()
  return response
}

function jsonRequest(method: 'POST' | 'PATCH' | 'PUT' | 'DELETE', body: unknown) {
  return { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }
}

function isAuthoringDraftList(value: unknown): value is AuthoringDraftList {
  if (!isRecord(value) || !Array.isArray(value.drafts)) return false
  return value.drafts.every(isAuthoringDraftSummary)
}

function isAuthoringDraftSummary(value: unknown): value is AuthoringDraftSummary {
  return isRecord(value)
    && typeof value.id === 'string'
    && isAuthoringDraftID(value.id)
    && typeof value.title === 'string'
    && value.title.length > 0
    && value.title.length <= 240
    && typeof value.intendedVersion === 'string'
    && isAuthoringVersion(value.intendedVersion)
    && (value.status === 'ACTIVE' || value.status === 'ABANDONED')
    && isDateTime(value.updatedAt)
}

function isAuthoringDraft(value: unknown): value is AuthoringDraft {
  return isRecord(value)
    && typeof value.id === 'string' && isAuthoringDraftID(value.id)
    && typeof value.course_id === 'string' && isAuthoringDraftID(value.course_id)
    && typeof value.intended_version === 'string' && isAuthoringVersion(value.intended_version)
    && typeof value.source_language === 'string' && value.source_language.length > 0 && value.source_language.length <= 64
    && typeof value.title === 'string' && value.title.trim().length > 0 && value.title.length <= 240
    && typeof value.description === 'string' && value.description.trim().length > 0 && value.description.length <= 20000
    && Array.isArray(value.objectives) && value.objectives.length >= 1 && value.objectives.length <= 100 && value.objectives.every((objective) => typeof objective === 'string' && objective.trim().length > 0 && objective.length <= 1000)
    && typeof value.changelog === 'string' && value.changelog.trim().length > 0 && value.changelog.length <= 20000
    && isRecord(value.license)
    && (value.status === 'ACTIVE' || value.status === 'ABANDONED')
    && typeof value.revision === 'number' && Number.isInteger(value.revision) && value.revision >= 1
    && isDateTime(value.created_at) && isDateTime(value.updated_at)
}

function isAuthoringReviewDetail(value: unknown): value is AuthoringReviewDetail {
  return isRecord(value)
    && isRecord(value.review)
    && typeof value.review.id === 'string' && isAuthoringDraftID(value.review.id)
    && typeof value.review.draftId === 'string' && isAuthoringDraftID(value.review.draftId)
    && (value.review.status === 'IN_REVIEW' || value.review.status === 'APPROVED' || value.review.status === 'CHANGES_REQUESTED')
    && typeof value.review.reviewRevision === 'number' && Number.isSafeInteger(value.review.reviewRevision) && value.review.reviewRevision > 0
    && isRecord(value.snapshot) && typeof value.snapshot.schemaVersion === 'number' && Array.isArray(value.snapshot.modules)
    && isAuthoringReviewPublicationStatus(value.publication)
}

function isAuthoringReviewPublicationStatus(value: unknown): value is AuthoringReviewPublicationStatus {
  if (!isRecord(value) || typeof value.canPublish !== 'boolean' || typeof value.publishable !== 'boolean' || !Array.isArray(value.issues)) return false
  if (!value.issues.every((issue) => isRecord(issue) && typeof issue.code === 'string' && typeof issue.path === 'string' && typeof issue.message === 'string')) return false
  return value.published === null || (isRecord(value.published)
    && typeof value.published.courseId === 'string' && isAuthoringDraftID(value.published.courseId)
    && typeof value.published.courseVersion === 'string' && isAuthoringVersion(value.published.courseVersion)
    && isDateTime(value.published.publishedAt))
}

function isAuthoringPublication(value: unknown): value is AuthoringPublication {
  return isRecord(value)
    && typeof value.reviewId === 'string' && isAuthoringDraftID(value.reviewId)
    && typeof value.reviewRevision === 'number' && Number.isSafeInteger(value.reviewRevision) && value.reviewRevision > 0
    && typeof value.courseId === 'string' && isAuthoringDraftID(value.courseId)
    && typeof value.courseVersion === 'string' && isAuthoringVersion(value.courseVersion)
    && typeof value.courseVersionId === 'string' && isAuthoringDraftID(value.courseVersionId)
    && isDateTime(value.publishedAt)
}

function isAuthoringAsset(value: unknown): value is AuthoringAsset {
  return isRecord(value)
    && typeof value.assetKey === 'string' && isAuthoringDraftID(value.assetKey)
    && typeof value.filename === 'string' && isSafeAssetFilename(value.filename)
    && typeof value.mediaType === 'string' && isCanonicalMediaType(value.mediaType)
    && typeof value.byteSize === 'number' && Number.isSafeInteger(value.byteSize) && value.byteSize > 0
    && value.status === 'AVAILABLE'
}
function isAuthoringAssetList(value: unknown): value is AuthoringAssetList {
  return isRecord(value) && Array.isArray(value.items) && value.items.every((item) => isRecord(item) && typeof item.assetKey === 'string' && isAuthoringDraftID(item.assetKey) && typeof item.filename === 'string' && isSafeAssetFilename(item.filename) && typeof item.mediaType === 'string' && isCanonicalMediaType(item.mediaType) && typeof item.byteSize === 'number' && Number.isSafeInteger(item.byteSize) && item.byteSize > 0 && isDateTime(item.createdAt))
    && typeof value.limit === 'number' && Number.isSafeInteger(value.limit) && value.limit >= 1 && value.limit <= 100
    && typeof value.offset === 'number' && Number.isSafeInteger(value.offset) && value.offset >= 0
    && typeof value.total === 'number' && Number.isSafeInteger(value.total) && value.total >= 0
}

function isAuthoringAssessmentList(value: unknown): value is AuthoringAssessmentList {
  return isRecord(value) && Array.isArray(value.items) && value.items.every(isAuthoringAssessmentSummary)
    && isPage(value.limit, value.offset) && typeof value.total === 'number' && Number.isSafeInteger(value.total) && value.total >= 0
}

function isAuthoringAssessmentSummary(value: unknown): value is AuthoringAssessmentSummary {
  return isRecord(value) && typeof value.assessmentKey === 'string' && isAuthoringDraftID(value.assessmentKey)
    && isAssessmentTitle(value.title) && typeof value.questionCount === 'number' && Number.isSafeInteger(value.questionCount) && value.questionCount >= 0
    && isRevision(value.revision) && isDateTime(value.updatedAt)
}

function isAuthoringAssessmentDetail(value: unknown): value is AuthoringAssessmentDetail {
  return isRecord(value) && typeof value.assessmentKey === 'string' && isAuthoringDraftID(value.assessmentKey)
    && isAssessmentTitle(value.title) && isRevision(value.revision) && isDateTime(value.createdAt) && isDateTime(value.updatedAt)
    && Array.isArray(value.questions) && value.questions.length <= 100 && value.questions.every(isAuthoringAssessmentQuestion)
    && unique(value.questions.map((question) => question.stableKey)) && sequential(value.questions.map((question) => question.position))
}

function isAuthoringAssessmentQuestion(value: unknown): value is AuthoringAssessmentQuestion {
  if (!isRecord(value) || !isStableAssessmentKey(value.stableKey) || !isNonEmptyText(value.prompt, 10_000) || !isPosition(value.position)
    || !Array.isArray(value.options) || !Array.isArray(value.correctOptionKeys) || !Array.isArray(value.leftItems) || !Array.isArray(value.rightItems) || !Array.isArray(value.correctPairs)) return false
  const options = value.options as unknown[]; const correct = value.correctOptionKeys as unknown[]; const leftItems = value.leftItems as unknown[]; const rightItems = value.rightItems as unknown[]; const pairs = value.correctPairs as unknown[]
  if (!options.every(isAuthoringAssessmentOption) || !leftItems.every(isAuthoringAssessmentItem) || !rightItems.every(isAuthoringAssessmentItem)
    || !pairs.every(isAuthoringAssessmentPair) || !correct.every(isStableAssessmentKey)) return false
  const safeOptions = options as AuthoringAssessmentOption[]; const safeCorrect = correct as string[]; const safeLeft = leftItems as AuthoringAssessmentMatchingItem[]; const safeRight = rightItems as AuthoringAssessmentMatchingItem[]; const safePairs = pairs as AuthoringAssessmentMatchingPair[]
  if (!unique(safeOptions.map((option) => option.stableKey)) || !unique(safeLeft.map((item) => item.stableKey)) || !unique(safeRight.map((item) => item.stableKey))
    || !sequential(safeOptions.map((option) => option.position)) || !sequential(safeLeft.map((item) => item.position)) || !sequential(safeRight.map((item) => item.position))) return false
  if (value.type === 'SINGLE_CHOICE') return safeOptions.length >= 2 && safeLeft.length === 0 && safeRight.length === 0 && safePairs.length === 0
    && safeCorrect.length === 1 && safeOptions.some((option) => option.stableKey === safeCorrect[0])
  if (value.type === 'MULTIPLE_CHOICE') return safeOptions.length >= 2 && safeLeft.length === 0 && safeRight.length === 0 && safePairs.length === 0
    && safeCorrect.length > 0 && unique(safeCorrect) && safeCorrect.every((key) => safeOptions.some((option) => option.stableKey === key))
  if (value.type === 'MATCHING') return safeOptions.length === 0 && safeCorrect.length === 0 && safeLeft.length > 0 && safeLeft.length === safeRight.length
    && safePairs.length === safeLeft.length && unique(safePairs.map((pair) => pair.leftKey)) && unique(safePairs.map((pair) => pair.rightKey))
    && safePairs.every((pair) => safeLeft.some((item) => item.stableKey === pair.leftKey) && safeRight.some((item) => item.stableKey === pair.rightKey))
  return false
}

function isAuthoringAssessmentOption(value: unknown): value is AuthoringAssessmentOption { return isRecord(value) && isStableAssessmentKey(value.stableKey) && isNonEmptyText(value.text, 4000) && isPosition(value.position) }
function isAuthoringAssessmentItem(value: unknown): value is AuthoringAssessmentMatchingItem { return isRecord(value) && isStableAssessmentKey(value.stableKey) && isNonEmptyText(value.text, 4000) && isPosition(value.position) }
function isAuthoringAssessmentPair(value: unknown): value is AuthoringAssessmentMatchingPair { return isRecord(value) && isStableAssessmentKey(value.leftKey) && isStableAssessmentKey(value.rightKey) }
function isAssessmentTitle(value: unknown): boolean { return isNonEmptyText(value, 240) }
function isNonEmptyText(value: unknown, max: number): value is string { return typeof value === 'string' && value.trim().length > 0 && value.length <= max }
function isStableAssessmentKey(value: unknown): value is string { return typeof value === 'string' && value.length >= 1 && value.length <= 160 && /^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(value) }
function isPosition(value: unknown): boolean { return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0 }
function isRevision(value: unknown): boolean { return typeof value === 'number' && Number.isSafeInteger(value) && value >= 1 }
function isPage(limit: unknown, offset: unknown): boolean { return typeof limit === 'number' && Number.isSafeInteger(limit) && limit >= 1 && limit <= 100 && typeof offset === 'number' && Number.isSafeInteger(offset) && offset >= 0 }
function unique(values: string[]): boolean { return new Set(values).size === values.length }
function sequential(positions: number[]): boolean { return positions.every((position, index) => position === index) }

function isSafeAssetFilename(value: string): boolean {
  return value.length > 0 && value.length <= 255 && value.trim() === value
    && value !== '.' && value !== '..' && !/[\\/\u0000-\u001f\u007f]/.test(value)
}

function isCanonicalMediaType(value: string): boolean {
  return value.length >= 3 && value.length <= 127
    && /^[a-z0-9][a-z0-9!#$&^_.+-]*\/[a-z0-9][a-z0-9!#$&^_.+-]*$/.test(value)
}

function isAuthoringVersion(value: string): boolean {
  return /^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$/.test(value)
}

function isDateTime(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && !Number.isNaN(Date.parse(value))
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function authoringDraftPath(draftID: string, section: 'overview' | 'structure' | 'members' | 'review' = 'overview'): string {
  return `/authoring/drafts/${encodeURIComponent(draftID)}/${section}`
}

export function authoringDraftReviewPath(draftID: string, reviewID: string): string {
  assertAuthoringID(draftID)
  assertAuthoringID(reviewID)
  return `/authoring/drafts/${encodeURIComponent(draftID)}/reviews/${encodeURIComponent(reviewID)}`
}

export function authoringDraftLessonPath(draftID: string, lessonID: string): string {
  assertAuthoringID(draftID); assertAuthoringID(lessonID)
  return `/authoring/drafts/${encodeURIComponent(draftID)}/lessons/${encodeURIComponent(lessonID)}`
}

export function authoringDraftAssessmentsPath(draftID: string): string {
  assertAuthoringID(draftID)
  return `/authoring/drafts/${encodeURIComponent(draftID)}/assessments`
}

export function authoringDraftAssessmentPath(draftID: string, assessmentID: string): string {
  assertAuthoringID(draftID); assertAuthoringID(assessmentID)
  return `${authoringDraftAssessmentsPath(draftID)}/${encodeURIComponent(assessmentID)}`
}
