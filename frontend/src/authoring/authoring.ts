import { useAuth, type AuthService } from '../auth/auth'
import type { components } from '../api/generated'

export type AuthoringDraft = components['schemas']['AuthoringDraft']
export type AuthoringContentLicense = components['schemas']['ContentLicense']

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

export async function getAuthoringDraft(draftID: string, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringDraft> {
  if (!isAuthoringDraftID(draftID)) throw new InvalidAuthoringDraftIDError()
  return client.request<AuthoringDraft>(`/api/authoring/drafts/${encodeURIComponent(draftID)}`)
}

export async function updateAuthoringDraft(draftID: string, patch: AuthoringDraftMetadataPatch, client: Pick<AuthService, 'request'> = useAuth()): Promise<AuthoringDraft> {
  if (!isAuthoringDraftID(draftID)) throw new InvalidAuthoringDraftIDError()
  return client.request<AuthoringDraft>(`/api/authoring/drafts/${encodeURIComponent(draftID)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  })
}

export function authoringDraftPath(draftID: string, section: 'overview' | 'structure' | 'members' = 'overview'): string {
  return `/authoring/drafts/${encodeURIComponent(draftID)}/${section}`
}
