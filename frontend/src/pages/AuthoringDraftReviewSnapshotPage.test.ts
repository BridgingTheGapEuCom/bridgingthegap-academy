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
const approveAuthoringReviewMock = vi.hoisted(() => vi.fn())
const requestAuthoringReviewChangesMock = vi.hoisted(() => vi.fn())
vi.mock('../auth/auth', () => ({ useAuth: () => authMock }))
vi.mock('../authoring/authoring', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../authoring/authoring')>()),
  getAuthoringDraftReview: getAuthoringDraftReviewMock,
  approveAuthoringReview: approveAuthoringReviewMock,
  requestAuthoringReviewChanges: requestAuthoringReviewChangesMock,
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

function detail(
  id = firstReviewID,
  title = 'Frozen integration foundations',
  status: 'IN_REVIEW' | 'APPROVED' | 'CHANGES_REQUESTED' = 'APPROVED',
  reviewRevision = 2,
): AuthoringReviewDetail {
  return {
    review: {
      id, draftId: draftID, draftRevision: 4, snapshotSchemaVersion: 1, status, reviewRevision,
      submittedBy: actor, submittedAt: '2026-09-15T12:00:00Z',
      decidedBy: status === 'IN_REVIEW' ? null : actor,
      decidedAt: status === 'IN_REVIEW' ? null : '2026-09-15T13:00:00Z',
    },
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
    approveAuthoringReviewMock.mockReset()
    requestAuthoringReviewChangesMock.mockReset()
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
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Request changes' })).toBeNull()
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

  it('offers both terminal decisions only for an in-review cycle', async () => {
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail(firstReviewID, 'Frozen integration foundations', 'IN_REVIEW', 5))
    await renderPage()
    expect(await screen.findByRole('button', { name: 'Approve' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Request changes' })).toBeTruthy()

    cleanup()
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail(firstReviewID, 'Frozen integration foundations', 'CHANGES_REQUESTED', 6))
    await renderPage()
    expect(await screen.findByText('Changes requested')).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Request changes' })).toBeNull()
  })

  it('approves using the exact authoritative Review revision then reloads the frozen Review', async () => {
    const inReview = detail(firstReviewID, 'Frozen integration foundations', 'IN_REVIEW', 5)
    const approved = detail(firstReviewID, 'Frozen integration foundations', 'APPROVED', 6)
    getAuthoringDraftReviewMock.mockResolvedValueOnce(inReview).mockResolvedValueOnce(approved)
    approveAuthoringReviewMock.mockResolvedValueOnce(approved.review)
    await renderPage()
    await (await screen.findByRole('button', { name: 'Approve' })).click()
    expect(approveAuthoringReviewMock).toHaveBeenCalledWith(draftID, firstReviewID, { expectedReviewRevision: 5 })
    expect(await screen.findByText('Approved Review is not a published Course.')).toBeTruthy()
    expect(screen.getByText('Frozen integration foundations')).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Request changes' })).toBeNull()
    expect(getAuthoringDraftReviewMock).toHaveBeenCalledTimes(2)
  })

  it('requests changes using the exact authoritative Review revision and preserves the snapshot', async () => {
    const inReview = detail(firstReviewID, 'Frozen integration foundations', 'IN_REVIEW', 5)
    const requested = detail(firstReviewID, 'Frozen integration foundations', 'CHANGES_REQUESTED', 6)
    getAuthoringDraftReviewMock.mockResolvedValueOnce(inReview).mockResolvedValueOnce(requested)
    requestAuthoringReviewChangesMock.mockResolvedValueOnce(requested.review)
    await renderPage()
    await (await screen.findByRole('button', { name: 'Request changes' })).click()
    expect(requestAuthoringReviewChangesMock).toHaveBeenCalledWith(draftID, firstReviewID, { expectedReviewRevision: 5 })
    expect(await screen.findByText('Changes requested')).toBeTruthy()
    expect(screen.getByText('Frozen integration foundations')).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull()
  })

  it('guards competing local decisions while one request is pending', async () => {
    const inReview = detail(firstReviewID, 'Frozen integration foundations', 'IN_REVIEW', 5)
    getAuthoringDraftReviewMock.mockResolvedValueOnce(inReview).mockResolvedValueOnce(detail(firstReviewID, 'Frozen integration foundations', 'APPROVED', 6))
    let resolveApprove: (value: unknown) => void = () => undefined
    approveAuthoringReviewMock.mockImplementationOnce(() => new Promise((resolve) => { resolveApprove = resolve }))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Approve' })).click()
    await screen.getByRole('button', { name: 'Request changes' }).click()
    expect(screen.getByText('Approving this Review…')).toBeTruthy()
    expect(approveAuthoringReviewMock).toHaveBeenCalledTimes(1)
    expect(requestAuthoringReviewChangesMock).not.toHaveBeenCalled()
    resolveApprove(detail().review)
    expect(await screen.findByText('Approved Review is not a published Course.')).toBeTruthy()
  })

  it('maps the structured independent-review policy conflict without frontend identity inference', async () => {
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail(firstReviewID, 'Frozen integration foundations', 'IN_REVIEW', 5))
    approveAuthoringReviewMock.mockRejectedValueOnce(new APIProblemError(409, {
      type: 'https://academy.example/problems/independent-reviewer-required', title: 'Independent reviewer required', status: 409,
      instance: '/api/authoring/drafts/example/reviews/example/approve', request_id: 'request-id', code: 'independent_reviewer_required',
    }, undefined))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Approve' })).click()
    expect(await screen.findByText('This Review must be decided by someone other than the person who submitted it.')).toBeTruthy()
    expect(screen.queryByText(/changed before your decision/)).toBeNull()
    expect(getAuthoringDraftReviewMock).toHaveBeenCalledTimes(1)
  })

  it('keeps generic Review conflicts distinct and reloads only the exact Review on demand', async () => {
    const inReview = detail(firstReviewID, 'Frozen integration foundations', 'IN_REVIEW', 5)
    const approved = detail(firstReviewID, 'Frozen integration foundations', 'APPROVED', 6)
    getAuthoringDraftReviewMock.mockResolvedValueOnce(inReview).mockResolvedValueOnce(approved)
    approveAuthoringReviewMock.mockRejectedValueOnce(new APIProblemError(409, undefined, undefined))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Approve' })).click()
    expect(await screen.findByText('This Review changed before your decision was saved. Reload the latest Review state.')).toBeTruthy()
    expect(screen.queryByText(/someone other than the person/)).toBeNull()
    await screen.getByRole('button', { name: 'Reload review' }).click()
    expect(await screen.findByText('Approved Review is not a published Course.')).toBeTruthy()
    expect(approveAuthoringReviewMock).toHaveBeenCalledTimes(1)
    expect(getAuthoringDraftReviewMock).toHaveBeenCalledTimes(2)
  })

  it('preserves the in-review snapshot after an operational decision failure', async () => {
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail(firstReviewID, 'Frozen integration foundations', 'IN_REVIEW', 5))
    requestAuthoringReviewChangesMock.mockRejectedValueOnce(new APIProblemError(500, undefined, undefined))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Request changes' })).click()
    expect(await screen.findByText('We couldn’t save this Review decision right now. Please try again.')).toBeTruthy()
    expect(screen.getByText('Frozen integration foundations')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Approve' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Request changes' })).toBeTruthy()
  })

  it('keeps a hidden decision denial opaque', async () => {
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail(firstReviewID, 'Frozen integration foundations', 'IN_REVIEW', 5))
    approveAuthoringReviewMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    const page = await renderPage()
    await (await screen.findByRole('button', { name: 'Approve' })).click()
    await waitFor(() => expect(page.markDraftUnavailable).toHaveBeenCalledTimes(1))
  })

  it('does not let a late decision response affect a newly selected Review route', async () => {
    const first = detail(firstReviewID, 'First frozen snapshot', 'IN_REVIEW', 5)
    const second = detail(secondReviewID, 'Second frozen snapshot', 'IN_REVIEW', 7)
    getAuthoringDraftReviewMock.mockResolvedValueOnce(first).mockResolvedValueOnce(second)
    let resolveApprove: (value: unknown) => void = () => undefined
    approveAuthoringReviewMock.mockImplementationOnce(() => new Promise((resolve) => { resolveApprove = resolve }))
    const { router } = await renderPage()
    await (await screen.findByRole('button', { name: 'Approve' })).click()
    await router.push(`/authoring/drafts/${draftID}/reviews/${secondReviewID}`)
    expect(await screen.findByText('Second frozen snapshot')).toBeTruthy()
    resolveApprove(detail(firstReviewID, 'First frozen snapshot', 'APPROVED', 6).review)
    await waitFor(() => expect(screen.queryByText('First frozen snapshot')).toBeNull())
    expect(screen.getByRole('button', { name: 'Approve' })).toBeTruthy()
  })
})
