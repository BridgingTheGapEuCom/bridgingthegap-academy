import { useAuth, type AuthService } from '../auth/auth'
import type { components } from '../api/generated'

export type AuthoringDraft = components['schemas']['AuthoringDraft']
export type AuthoringDraftSummary = components['schemas']['AuthoringDraftSummary']
export type AuthoringDraftList = components['schemas']['AuthoringDraftList']
export type AuthoringContentLicense = components['schemas']['ContentLicense']
export type AuthoringStructure = components['schemas']['AuthoringStructure']
export type AuthoringModuleSummary = components['schemas']['AuthoringModuleSummary']
export type AuthoringLessonSummary = components['schemas']['AuthoringLessonSummary']
export type AuthoringLessonDetail = components['schemas']['AuthoringLessonDetail']
export type AuthoringLessonContent = components['schemas']['LessonContent']
export type AuthoringLessonContentMutation = components['schemas']['AuthoringLessonContentMutationResponse']
export type AuthoringActiveMember = components['schemas']['AuthoringActiveMember']
export type AuthoringActiveMemberList = components['schemas']['AuthoringActiveMemberList']
export type AuthoringMemberAdd = components['schemas']['AuthoringMemberAddRequest']
export type AuthoringMemberRoleChange = components['schemas']['AuthoringMemberRoleRequest']
export type AuthoringMemberMutation = components['schemas']['AuthoringMemberMutationResponse']
export type AuthoringReview = components['schemas']['AuthoringReview']
export type AuthoringReviewList = components['schemas']['AuthoringReviewList']
export type AuthoringReviewDetail = components['schemas']['AuthoringReviewDetail']
export type AuthoringReviewSubmit = components['schemas']['AuthoringReviewSubmitRequest']
export type AuthoringReviewDecision = components['schemas']['AuthoringReviewDecisionRequest']
export type AuthoringReviewSnapshot = components['schemas']['AuthoringReviewSnapshot']
export type AuthoringReviewSnapshotModule = components['schemas']['AuthoringReviewSnapshotModule']
export type AuthoringReviewSnapshotLesson = components['schemas']['AuthoringReviewSnapshotLesson']
export type AuthoringPublicationRequest = components['schemas']['AuthoringPublicationRequest']
export type AuthoringPublication = components['schemas']['AuthoringPublication']
export type PublicationValidationIssue = components['schemas']['PublicationValidationIssue']

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
  return client.request<AuthoringDraftList>('/api/authoring/drafts')
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
  return client.request<AuthoringReviewDetail>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/reviews/${encodeURIComponent(reviewID)}`)
}

// The server freezes the canonical snapshot from the authoritative Draft. The
// frontend sends only the Draft revision it intends to submit and does not use
// the snapshot response until a later explicit Review-detail experience.
export async function submitAuthoringDraftReview(draftID: string, input: AuthoringReviewSubmit, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringReviewDetail> {
  assertAuthoringID(draftID)
  return client.request<AuthoringReviewDetail>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/reviews`, jsonRequest('POST', input))
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
  return client.request<AuthoringPublication>(`/api/authoring/drafts/${encodeURIComponent(draftID)}/reviews/${encodeURIComponent(reviewID)}/publish`, jsonRequest('POST', input))
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

function jsonRequest(method: 'POST' | 'PATCH' | 'PUT' | 'DELETE', body: unknown) {
  return { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }
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
