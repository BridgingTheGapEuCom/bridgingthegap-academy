import { cleanup, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { shallowRef } from 'vue'
import { APIProblemError } from '../api/client'
import { authoringDraftContextKey } from '../authoring/draftContext'
import type { AuthoringReviewDetail } from '../authoring/authoring'
import AuthoringDraftReviewSnapshotPage from './AuthoringDraftReviewSnapshotPage.vue'

type AuthenticationState =
  | { status: 'bootstrapping' }
  | { status: 'unauthenticated' }
  | { status: 'authenticated'; userId: string; expiresAt: string }
  | { status: 'unavailable' }

const authMock = vi.hoisted(() => ({ state: { value: { status: 'authenticated' } as AuthenticationState } }))
const getAuthoringDraftReviewMock = vi.hoisted(() => vi.fn())
vi.mock('../auth/auth', () => ({ useAuth: () => authMock }))
vi.mock('../authoring/authoring', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../authoring/authoring')>()),
  getAuthoringDraftReview: getAuthoringDraftReviewMock,
}))

const draftID = '11111111-1111-4111-8111-111111111111'
const firstReviewID = '22222222-2222-4222-8222-222222222222'
const secondReviewID = '33333333-3333-4333-8333-333333333333'
const actor = '44444444-4444-4444-8444-444444444444'

const currentDraft = {
  id: draftID, course_id: '55555555-5555-4555-8555-555555555555', intended_version: '9.0.0', source_language: 'en', title: 'Current mutable Draft',
  description: '', objectives: [], changelog: '', license: { kind: 'STANDARD' as const, display_name: 'Standard license' }, status: 'ACTIVE' as const,
  revision: 9, created_at: '2026-09-15T10:00:00Z', updated_at: '2026-09-15T11:00:00Z',
}

function detail(id = firstReviewID, title = 'Frozen integration foundations'): AuthoringReviewDetail {
  return {
    review: { id, draftId: draftID, draftRevision: 4, snapshotSchemaVersion: 1, status: 'APPROVED', reviewRevision: 2, submittedBy: actor, submittedAt: '2026-09-15T12:00:00Z', decidedBy: actor, decidedAt: '2026-09-15T13:00:00Z' },
    snapshot: {
      schemaVersion: 1,
      draft: { id: draftID, revision: 4, courseId: '55555555-5555-4555-8555-555555555555', intendedVersion: '1.0.0', sourceLanguage: 'en', title, description: 'Historical description', objectives: ['Understand snapshots'], changelog: 'Frozen before Review.', license: { Kind: 'STANDARD', Identifier: 'CC-BY-4.0', DisplayName: 'Creative Commons Attribution 4.0', URL: '', CustomText: '' } },
      modules: [
        { id: '66666666-6666-4666-8666-666666666666', stableKey: 'first', title: 'First Module', description: '', position: 0, lessons: [
          { id: '77777777-7777-4777-8777-777777777777', stableKey: 'before', title: 'Before lesson', description: '', objectives: [], estimatedDurationMinutes: 10, position: 0, prerequisiteStableKeys: [], content: { schemaVersion: 1, blocks: [{ key: 'historical-text', type: 'TEXT', payload: { content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: '<script>never execute</script>', marks: [] }] }] } } }] } },
          { id: '88888888-8888-4888-8888-888888888888', stableKey: 'after', title: 'After lesson', description: '', objectives: ['Apply the model'], estimatedDurationMinutes: null, position: 1, prerequisiteStableKeys: ['before'], content: { schemaVersion: 1, blocks: [
            { key: 'image', type: 'IMAGE', payload: { asset: { assetKey: 'diagram' }, decorative: false, altText: 'Historical diagram' } },
            { key: 'check', type: 'KNOWLEDGE_CHECK', payload: { assessmentKey: 'opaque-check' } },
            { key: 'unknown', type: 'FUTURE_WIDGET', payload: { script: 'unsafe' } },
          ] } },
        ] },
        { id: '99999999-9999-4999-8999-999999999999', stableKey: 'second', title: 'Second Module', description: 'Captured second.', position: 1, lessons: [] },
      ],
    },
  } as unknown as AuthoringReviewDetail
}

async function renderPage(path = `/authoring/drafts/${draftID}/reviews/${firstReviewID}`) {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/authoring/drafts/:draftId/review', name: 'review-overview', component: { template: '<h2>Review overview</h2>' } },
    { path: '/authoring/drafts/:draftId/reviews/:reviewId', name: 'snapshot', component: AuthoringDraftReviewSnapshotPage },
  ] })
  await router.push(path)
  await router.isReady()
  const draft = shallowRef(currentDraft)
  const markDraftUnavailable = vi.fn()
  const result = render({ template: '<RouterView />' }, {
    global: { plugins: [router], provide: { [authoringDraftContextKey as symbol]: { draft, replaceDraft: vi.fn(), markDraftUnavailable } } },
  })
  return { ...result, router, markDraftUnavailable }
}

describe('AuthoringDraftReviewSnapshotPage', () => {
  beforeEach(() => {
    authMock.state = shallowRef<AuthenticationState>({ status: 'authenticated', userId: actor, expiresAt: '2026-09-15T16:00:00Z' })
    getAuthoringDraftReviewMock.mockReset().mockResolvedValue(detail())
  })
  afterEach(cleanup)

  it('loads only the exact Review snapshot and renders its historical ordering and metadata', async () => {
    await renderPage()
    expect(await screen.findByRole('heading', { level: 2, name: 'Reviewing Draft revision 4' })).toBeTruthy()
    expect(getAuthoringDraftReviewMock).toHaveBeenCalledWith(draftID, firstReviewID)
    expect(screen.getByText('Frozen integration foundations')).toBeTruthy()
    expect(screen.queryByText('Current mutable Draft')).toBeNull()
    expect(screen.getByText('Approved Review is not a published Course.')).toBeTruthy()
    expect(screen.queryByText('Published')).toBeNull()
    expect(screen.getAllByText(actor).length).toBeGreaterThan(0)
    expect(screen.getByText('Creative Commons Attribution 4.0')).toBeTruthy()
    const modules = screen.getAllByRole('heading', { level: 3 })
    expect(modules.map((heading) => heading.textContent)).toEqual(['Draft metadata', 'Snapshot structure', 'First Module', 'Second Module'])
    expect(screen.getAllByRole('heading', { level: 4 }).map((heading) => heading.textContent)).toContain('Before lesson')
    expect(screen.getByText('Before lesson (before)')).toBeTruthy()
  })

  it('uses the canonical safe renderer for historical content and deferred block fallbacks', async () => {
    await renderPage()
    expect(await screen.findByText('<script>never execute</script>')).toBeTruthy()
    expect(document.querySelector('script')).toBeNull()
    expect(screen.getByText('Image asset unavailable')).toBeTruthy()
    expect(screen.getByText(/Interactive knowledge checks will be available/)).toBeTruthy()
    expect(screen.getByText('This lesson block cannot be displayed safely.')).toBeTruthy()
    expect(document.querySelector('[src]')).toBeNull()
  })

  it('does not let a slow prior exact Review response overwrite a new Review route', async () => {
    let resolveFirst: (value: AuthoringReviewDetail) => void = () => undefined
    getAuthoringDraftReviewMock.mockImplementationOnce(() => new Promise<AuthoringReviewDetail>((resolve) => { resolveFirst = resolve }))
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail(secondReviewID, 'Second frozen snapshot'))
    const { router } = await renderPage()
    await waitFor(() => expect(getAuthoringDraftReviewMock).toHaveBeenCalledWith(draftID, firstReviewID))
    await router.push(`/authoring/drafts/${draftID}/reviews/${secondReviewID}`)
    expect(await screen.findByText('Second frozen snapshot')).toBeTruthy()
    resolveFirst(detail(firstReviewID, 'Old frozen snapshot'))
    await waitFor(() => expect(screen.queryByText('Old frozen snapshot')).toBeNull())
  })

  it('keeps hidden and operational failures separate without substituting current Draft content', async () => {
    getAuthoringDraftReviewMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    const hidden = await renderPage()
    await waitFor(() => expect(hidden.markDraftUnavailable).toHaveBeenCalledTimes(1))
    expect(screen.queryByText('Current mutable Draft')).toBeNull()

    cleanup()
    getAuthoringDraftReviewMock.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(detail())
    await renderPage()
    expect(await screen.findByRole('heading', { level: 2, name: 'Review unavailable' })).toBeTruthy()
    await screen.getByRole('button', { name: 'Try again' }).click()
    expect(await screen.findByText('Frozen integration foundations')).toBeTruthy()
  })

  it('fails safely for a future snapshot schema without substituting the current Draft', async () => {
    getAuthoringDraftReviewMock.mockResolvedValueOnce({ ...detail(), snapshot: { ...detail().snapshot, schemaVersion: 2 } } as unknown as AuthoringReviewDetail)
    await renderPage()
    expect(await screen.findByRole('heading', { level: 2, name: 'Review snapshot format unavailable' })).toBeTruthy()
    expect(screen.queryByText('Current mutable Draft')).toBeNull()
  })
})
