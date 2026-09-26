import { cleanup, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { shallowRef } from 'vue'
import { APIProblemError } from '../api/client'
import { authoringDraftContextKey } from '../authoring/draftContext'
import type { AuthoringPublication, AuthoringReviewDetail } from '../authoring/authoring'
import AuthoringDraftReviewSnapshotPage from './AuthoringDraftReviewSnapshotPage.vue'

const getAuthoringDraftReviewMock = vi.hoisted(() => vi.fn())
const approveAuthoringReviewMock = vi.hoisted(() => vi.fn())
const requestAuthoringReviewChangesMock = vi.hoisted(() => vi.fn())
const publishAuthoringDraftReviewMock = vi.hoisted(() => vi.fn())
vi.mock('../authoring/authoring', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../authoring/authoring')>()),
  getAuthoringDraftReview: getAuthoringDraftReviewMock,
  approveAuthoringReview: approveAuthoringReviewMock,
  requestAuthoringReviewChanges: requestAuthoringReviewChangesMock,
  publishAuthoringDraftReview: publishAuthoringDraftReviewMock,
}))

const draftID = '11111111-1111-4111-8111-111111111111'
const secondDraftID = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'
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
    publication: {
      canPublish: status === 'APPROVED',
      publishable: status === 'APPROVED',
      issues: status === 'APPROVED' ? [] : [{ code: 'review_not_approved', path: 'review.status', message: 'Only an approved Review can be published.' }],
      published: null,
    },
  } as unknown as AuthoringReviewDetail
}

function detailForDraft(targetDraftID: string, id: string, title: string): AuthoringReviewDetail {
  const value = detail(id, title)
  return {
    ...value,
    review: { ...value.review, draftId: targetDraftID },
    snapshot: { ...value.snapshot, draft: { ...value.snapshot.draft, id: targetDraftID } },
  }
}

function publishedDetail(
  id = firstReviewID,
  title = 'Frozen integration foundations',
  reviewRevision = 2,
): AuthoringReviewDetail {
  const value = detail(id, title, 'APPROVED', reviewRevision)
  value.publication = {
    canPublish: true,
    publishable: true,
    issues: [],
    published: { courseId: '55555555-5555-4555-8555-555555555555', courseVersion: '1.0.0', publishedAt: '2026-09-16T14:30:00Z' },
  }
  return value
}

function publicationResult(reviewID = firstReviewID, reviewRevision = 5): AuthoringPublication {
  return {
    reviewId: reviewID,
    reviewRevision,
    courseId: '55555555-5555-4555-8555-555555555555',
    courseVersion: '1.0.0',
    courseVersionId: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',
    publishedAt: '2026-09-16T14:30:00Z',
  }
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
  return { ...result, router, draft, markDraftUnavailable }
}

describe('AuthoringDraftReviewSnapshotPage', () => {
  beforeEach(() => {
    getAuthoringDraftReviewMock.mockReset().mockResolvedValue(detail())
    approveAuthoringReviewMock.mockReset()
    requestAuthoringReviewChangesMock.mockReset()
    publishAuthoringDraftReviewMock.mockReset()
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
    expect(screen.queryByText(actor)).toBeNull()
    expect(screen.getByText('Creative Commons Attribution 4.0')).toBeTruthy()
    const modules = screen.getAllByRole('heading', { level: 3 })
    expect(modules.map((heading) => heading.textContent)).toEqual(['Publication', 'Publish this Review', 'Draft metadata', 'Snapshot structure', 'First Module', 'Second Module'])
    expect(screen.getAllByRole('heading', { level: 4 }).map((heading) => heading.textContent)).toContain('Before lesson')
    expect(screen.getByText('Before lesson (before)')).toBeTruthy()
  })

  it('keeps every historical snapshot level unchanged when current Draft context changes', async () => {
    const page = await renderPage()
    expect(await screen.findByText('Frozen integration foundations')).toBeTruthy()
    expect(screen.getByText('Before lesson')).toBeTruthy()
    expect(screen.getByText('<script>never execute</script>')).toBeTruthy()
    page.draft.value = {
      ...currentDraft,
      revision: 99,
      title: 'Changed current Draft title',
      description: 'Changed current Draft description',
    }
    await waitFor(() => expect(screen.getByText('Frozen integration foundations')).toBeTruthy())
    expect(screen.getByText('Before lesson')).toBeTruthy()
    expect(screen.getByText('<script>never execute</script>')).toBeTruthy()
    expect(screen.queryByText('Changed current Draft title')).toBeNull()
    expect(screen.queryByText('Changed current Draft description')).toBeNull()
    expect(getAuthoringDraftReviewMock).toHaveBeenCalledTimes(1)
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

  it('does not let a slow prior Draft response cross into another Draft Review route', async () => {
    let resolveFirst: (value: AuthoringReviewDetail) => void = () => undefined
    const second = detailForDraft(secondDraftID, secondReviewID, 'Second Draft frozen snapshot')
    getAuthoringDraftReviewMock.mockImplementationOnce(() => new Promise<AuthoringReviewDetail>((resolve) => { resolveFirst = resolve }))
      .mockResolvedValueOnce(second)
    const page = await renderPage()
    await waitFor(() => expect(getAuthoringDraftReviewMock).toHaveBeenCalledWith(draftID, firstReviewID))
    page.draft.value = { ...currentDraft, id: secondDraftID, title: 'Second current Draft' }
    await page.router.push(`/authoring/drafts/${secondDraftID}/reviews/${secondReviewID}`)
    expect(await screen.findByText('Second Draft frozen snapshot')).toBeTruthy()
    expect(getAuthoringDraftReviewMock).toHaveBeenCalledWith(secondDraftID, secondReviewID)
    resolveFirst(detail(firstReviewID, 'Old Draft frozen snapshot'))
    await waitFor(() => expect(screen.queryByText('Old Draft frozen snapshot')).toBeNull())
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

  it('uses the server-authoritative publication capability for an exact approved Review', async () => {
    await renderPage()
    expect(await screen.findByRole('button', { name: 'Publish course version' })).toBeTruthy()
    expect(screen.getByRole('heading', { level: 4, name: 'Ready for publication' })).toBeTruthy()

    cleanup()
    const withoutCapability = detail()
    withoutCapability.publication = { ...withoutCapability.publication, canPublish: false }
    getAuthoringDraftReviewMock.mockResolvedValueOnce(withoutCapability)
    await renderPage()
    await screen.findByText('Frozen integration foundations')
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Publish course version' })).toBeNull())
    expect(screen.getByText('This Review is ready for publication.')).toBeTruthy()
  })

  it('does not offer publishing for an in-review or changes-requested cycle', async () => {
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail(firstReviewID, 'Frozen integration foundations', 'IN_REVIEW', 5))
    await renderPage()
    await screen.findByText('In review')
    expect(screen.queryByRole('button', { name: 'Publish course version' })).toBeNull()
    expect(screen.getByText('Publication is unavailable because this Review is not approved. Current status: in review.')).toBeTruthy()

    cleanup()
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail(firstReviewID, 'Frozen integration foundations', 'CHANGES_REQUESTED', 6))
    await renderPage()
    await screen.findByText('Changes requested')
    expect(screen.queryByRole('button', { name: 'Publish course version' })).toBeNull()
    expect(screen.getByText('Publication is unavailable because this Review is not approved. Current status: changes requested.')).toBeTruthy()
  })

  it('renders authoritative publication validation issues without enabling publication', async () => {
    const blocked = detail()
    blocked.publication = {
      canPublish: true,
      publishable: false,
      issues: [
        { code: 'unresolved_asset_reference', path: 'modules[media].lessons[intro].content.blocks[diagram]', message: 'The image reference must be resolved before publication.' },
        { code: 'unresolved_assessment_reference', path: 'modules[checks].lessons[review].content.blocks[knowledge-check]', message: 'The knowledge check reference must be resolved before publication.' },
      ],
      published: null,
    }
    getAuthoringDraftReviewMock.mockResolvedValueOnce(blocked)
    await renderPage()
    expect(await screen.findByRole('heading', { level: 4, name: 'Publication is blocked' })).toBeTruthy()
    expect(screen.getByText('The image reference must be resolved before publication.')).toBeTruthy()
    expect(screen.getByText('The knowledge check reference must be resolved before publication.')).toBeTruthy()
    expect(screen.getAllByText(/Location:/)).toHaveLength(2)
    expect(screen.queryByRole('button', { name: 'Publish course version' })).toBeNull()
  })

  it('identifies only an exact recorded publication and links to its public immutable version', async () => {
    getAuthoringDraftReviewMock.mockResolvedValueOnce(publishedDetail())
    await renderPage()
    expect(await screen.findByRole('heading', { level: 4, name: 'Published course version' })).toBeTruthy()
    expect(screen.getByText('Published: this exact Review produced course version 1.0.0.')).toBeTruthy()
    const link = screen.getByRole('link', { name: 'View published course version 1.0.0' })
    expect(link.getAttribute('href')).toBe('/courses/by-id/55555555-5555-4555-8555-555555555555/versions/1.0.0')
    expect(screen.queryByRole('button', { name: 'Publish course version' })).toBeNull()
    expect(screen.queryByText(/courseVersionId|publishedBy/i)).toBeNull()
  })

  it('publishes only the exact current Review revision and refreshes its authoritative state', async () => {
    const approved = detail(firstReviewID, 'Frozen integration foundations', 'APPROVED', 5)
    getAuthoringDraftReviewMock.mockResolvedValueOnce(approved).mockResolvedValueOnce(publishedDetail(firstReviewID, 'Refetched frozen snapshot', 5))
    publishAuthoringDraftReviewMock.mockResolvedValueOnce(publicationResult())
    await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    expect(publishAuthoringDraftReviewMock).toHaveBeenCalledWith(draftID, firstReviewID, { expectedReviewRevision: 5 })
    expect(await screen.findByText('Refetched frozen snapshot')).toBeTruthy()
    expect(getAuthoringDraftReviewMock).toHaveBeenCalledTimes(2)
    expect(screen.getByRole('heading', { level: 4, name: 'Published course version' })).toBeTruthy()
    expect(screen.queryByRole('heading', { level: 4, name: 'Course version published' })).toBeNull()
    expect(screen.queryByText('Course version 1.0.0 was published successfully.')).toBeNull()
    expect(screen.getByText('16 Sept 2026, 14:30')).toBeTruthy()
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('heading', { level: 4, name: 'Published course version' })))
  })

  it('guards duplicate publication while the exact Review request is pending', async () => {
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail()).mockResolvedValueOnce(publishedDetail())
    let resolvePublication: (value: unknown) => void = () => undefined
    publishAuthoringDraftReviewMock.mockImplementationOnce(() => new Promise((resolve) => { resolvePublication = resolve }))
    await renderPage()
    const publish = await screen.findByRole('button', { name: 'Publish course version' })
    await publish.click()
    expect(screen.getByText('Publishing course version…')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Publishing…' })).toHaveProperty('disabled', true)
    expect(screen.getByRole('button', { name: 'Publishing…' }).getAttribute('aria-busy')).toBe('true')
    await screen.getByRole('button', { name: 'Publishing…' }).click()
    expect(publishAuthoringDraftReviewMock).toHaveBeenCalledTimes(1)
    resolvePublication(publicationResult())
    await waitFor(() => expect(getAuthoringDraftReviewMock).toHaveBeenCalledTimes(2))
  })

  it('refreshes the exact Review after stale or non-approved publication conflicts without retrying', async () => {
    const approved = detail(firstReviewID, 'Frozen integration foundations', 'APPROVED', 5)
    const changed = detail(firstReviewID, 'Frozen integration foundations', 'CHANGES_REQUESTED', 6)
    getAuthoringDraftReviewMock.mockResolvedValueOnce(approved).mockResolvedValueOnce(changed)
    publishAuthoringDraftReviewMock.mockRejectedValueOnce(new APIProblemError(409, {
      type: 'https://academy.example/problems/review-revision-conflict', title: 'Review revision conflict', status: 409,
      instance: '/api/authoring/drafts/example/reviews/example/publish', request_id: 'request-id', code: 'review_revision_conflict',
    }, undefined))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    expect(await screen.findByText('Changes requested')).toBeTruthy()
    expect(screen.getByText('This Review changed before publication completed. The latest Review state has been loaded; publication was not retried.')).toBeTruthy()
    expect(publishAuthoringDraftReviewMock).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('button', { name: 'Publish course version' })).toBeNull()
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('heading', { level: 4, name: 'Review state changed' })))
  })

  it('refreshes the exact Review after a non-approved publication conflict without retrying', async () => {
    const approved = detail(firstReviewID, 'Frozen integration foundations', 'APPROVED', 5)
    const inReview = detail(firstReviewID, 'Frozen integration foundations', 'IN_REVIEW', 6)
    getAuthoringDraftReviewMock.mockResolvedValueOnce(approved).mockResolvedValueOnce(inReview)
    publishAuthoringDraftReviewMock.mockRejectedValueOnce(new APIProblemError(409, {
      type: 'https://academy.example/problems/review-not-approved', title: 'Review not approved', status: 409,
      instance: '/api/authoring/drafts/example/reviews/example/publish', request_id: 'request-id', code: 'review_not_approved',
    }, undefined))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    expect(await screen.findByText('In review')).toBeTruthy()
    expect(screen.getByText('This Review is no longer approved for publication. The current Review state has been loaded.')).toBeTruthy()
    expect(publishAuthoringDraftReviewMock).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('button', { name: 'Publish course version' })).toBeNull()
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('heading', { level: 4, name: 'Review state changed' })))
  })

  it('replaces stale readiness with authoritative blocking validation after a publication conflict', async () => {
    const issues = [
      { code: 'unresolved_asset_reference', path: 'modules[0].lessons[0].content.blocks[1]', message: 'Asset unavailable.' },
      { code: 'unresolved_assessment_reference', path: 'modules[0].lessons[1].content.blocks[2]', message: 'Assessment unavailable.' },
    ]
    const blocked = detail()
    blocked.publication = { canPublish: true, publishable: false, issues, published: null }
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail()).mockResolvedValueOnce(blocked)
    publishAuthoringDraftReviewMock.mockRejectedValueOnce(new APIProblemError(422, {
      type: 'https://academy.example/problems/publication-validation-failed', title: 'Publication validation failed', status: 422,
      instance: '/api/authoring/drafts/example/reviews/example/publish', request_id: 'request-id', code: 'publication_validation_failed', issues,
    }, undefined))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    expect(await screen.findByRole('heading', { level: 4, name: 'Publication is blocked' })).toBeTruthy()
    expect(screen.getByText('Asset unavailable.')).toBeTruthy()
    expect(screen.getByText('Assessment unavailable.')).toBeTruthy()
    expect(screen.getByText('Frozen integration foundations')).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Publish course version' })).toBeNull()
    expect(screen.queryByRole('heading', { level: 4, name: 'Publication validation issues' })).toBeNull()
    expect(screen.getAllByRole('list', { name: 'Publication validation issues' })).toHaveLength(1)
    expect(getAuthoringDraftReviewMock).toHaveBeenCalledTimes(2)
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('heading', { level: 4, name: 'Publication is blocked' })))
  })

  it('presents an exact replay as the same single successful publication outcome', async () => {
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail()).mockResolvedValueOnce(publishedDetail())
    publishAuthoringDraftReviewMock.mockResolvedValue(publicationResult(firstReviewID, 2))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    expect(await screen.findByRole('heading', { level: 4, name: 'Published course version' })).toBeTruthy()
    expect(publishAuthoringDraftReviewMock).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('button', { name: 'Publish course version' })).toBeNull()
    expect(screen.queryByRole('heading', { level: 4, name: 'Course version published' })).toBeNull()
    expect(screen.queryByText('Course version 1.0.0 was published successfully.')).toBeNull()
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('heading', { level: 4, name: 'Published course version' })))
  })

  it('clears obsolete validation feedback when a new publication attempt starts', async () => {
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail()).mockResolvedValueOnce(detail()).mockResolvedValueOnce(publishedDetail())
    publishAuthoringDraftReviewMock.mockRejectedValueOnce(new APIProblemError(422, {
      type: 'https://academy.example/problems/publication-validation-failed', title: 'Publication validation failed', status: 422,
      instance: '/publish', request_id: 'request-id', code: 'publication_validation_failed',
      issues: [{ code: 'unresolved_asset_reference', path: 'modules[0]', message: 'Old issue.' }],
    }, undefined))
    let resolveRetry: (value: AuthoringPublication) => void = () => undefined
    publishAuthoringDraftReviewMock.mockImplementationOnce(() => new Promise<AuthoringPublication>((resolve) => { resolveRetry = resolve }))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    expect(await screen.findByText('Old issue.')).toBeTruthy()
    await screen.getByRole('button', { name: 'Publish course version' }).click()
    expect(screen.queryByText('Old issue.')).toBeNull()
    expect(screen.getByText('Publishing course version…')).toBeTruthy()
    resolveRetry(publicationResult(firstReviewID, 2))
    expect(await screen.findByRole('heading', { level: 4, name: 'Published course version' })).toBeTruthy()
    expect(screen.queryByText('Old issue.')).toBeNull()
  })

  it('does not fabricate Published state when the mutation succeeds but the authoritative projection does not confirm it', async () => {
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail()).mockResolvedValueOnce(detail())
    publishAuthoringDraftReviewMock.mockResolvedValueOnce(publicationResult(firstReviewID, 2))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    expect(await screen.findByRole('heading', { level: 4, name: 'Publication status could not be confirmed' })).toBeTruthy()
    expect(screen.getByText(/refreshed Review did not confirm Published/)).toBeTruthy()
    expect(screen.queryByRole('heading', { level: 4, name: 'Published course version' })).toBeNull()
    expect(screen.queryByText('16 Sept 2026, 14:30')).toBeNull()
  })

  it('recovers from a failed post-publication refresh without leaving the action stuck pending', async () => {
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail())
      .mockRejectedValueOnce(new Error('refresh unavailable'))
      .mockResolvedValueOnce(publishedDetail())
    publishAuthoringDraftReviewMock.mockResolvedValueOnce(publicationResult(firstReviewID, 2))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    expect(await screen.findByRole('heading', { level: 2, name: 'Review unavailable' })).toBeTruthy()
    expect(screen.queryByText('Publishing course version…')).toBeNull()
    await screen.getByRole('button', { name: 'Try again' }).click()
    expect(await screen.findByRole('heading', { level: 4, name: 'Published course version' })).toBeTruthy()
    expect(screen.queryByText('Publishing course version…')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Publish course version' })).toBeNull()
  })

  it('lets a refreshed exact Published projection supersede an obsolete mutation conflict', async () => {
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail()).mockResolvedValueOnce(publishedDetail())
    publishAuthoringDraftReviewMock.mockRejectedValueOnce(new APIProblemError(409, {
      type: 'https://academy.example/problems/course-version-already-exists', title: 'Course version already exists', status: 409,
      instance: '/publish', request_id: 'request-id', code: 'course_version_already_exists',
    }, undefined))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    expect(await screen.findByRole('heading', { level: 4, name: 'Published course version' })).toBeTruthy()
    expect(screen.queryByRole('heading', { level: 4, name: 'Course version already exists' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Publish course version' })).toBeNull()
  })

  it('distinguishes a foreign Course version conflict without exposing hidden provenance', async () => {
    publishAuthoringDraftReviewMock.mockRejectedValueOnce(new APIProblemError(409, {
      type: 'https://academy.example/problems/course-version-already-exists', title: 'secret-review-id database constraint', status: 409,
      instance: '/publish', request_id: 'request-id', code: 'course_version_already_exists',
    }, undefined))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    expect(await screen.findByRole('heading', { level: 4, name: 'Course version already exists' })).toBeTruthy()
    expect(screen.getByText('This semantic version already exists for another publication. Choose a new version in a later Draft and Review cycle before publishing.')).toBeTruthy()
    expect(screen.queryByText(/secret-review-id|database constraint/i)).toBeNull()
    expect(getAuthoringDraftReviewMock).toHaveBeenCalledTimes(2)
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('heading', { level: 4, name: 'Course version already exists' })))
  })

  it('presents a safe publication consistency conflict without provenance details', async () => {
    publishAuthoringDraftReviewMock.mockRejectedValueOnce(new APIProblemError(409, {
      type: 'https://academy.example/problems/publication-conflict', title: 'provenance mismatch with review abc', status: 409,
      instance: '/publish', request_id: 'request-id', code: 'publication_conflict',
    }, undefined))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    expect(await screen.findByRole('heading', { level: 4, name: 'Publication conflict' })).toBeTruthy()
    expect(screen.getByText('Publication could not be completed because the stored publication state is inconsistent. Review the current state before trying again.')).toBeTruthy()
    expect(screen.queryByText(/review abc|provenance mismatch/i)).toBeNull()
  })

  it('keeps operational errors sanitized and the retry action reachable', async () => {
    publishAuthoringDraftReviewMock.mockRejectedValueOnce(new APIProblemError(500, {
      type: 'about:blank', title: 'SQLSTATE 23505 duplicate key publication_review_id_key', status: 500,
      instance: '/publish', request_id: 'request-id',
    }, undefined))
    await renderPage()
    const publish = await screen.findByRole('button', { name: 'Publish course version' })
    publish.focus()
    await publish.click()
    expect(await screen.findByRole('heading', { level: 4, name: 'Publication failed' })).toBeTruthy()
    expect(screen.getByText('We couldn’t publish this Review right now. Please try again.')).toBeTruthy()
    expect(screen.queryByText(/SQLSTATE|duplicate key|publication_review_id_key/i)).toBeNull()
    expect(document.activeElement).toBe(publish)
    expect(screen.getByRole('button', { name: 'Publish course version' })).toBeTruthy()
  })

  it('does not let a late publication response affect a newly selected Review route', async () => {
    const first = detail(firstReviewID, 'First frozen snapshot', 'APPROVED', 5)
    const second = detail(secondReviewID, 'Second frozen snapshot', 'APPROVED', 7)
    getAuthoringDraftReviewMock.mockResolvedValueOnce(first).mockResolvedValueOnce(second)
    let resolvePublication: (value: unknown) => void = () => undefined
    publishAuthoringDraftReviewMock.mockImplementationOnce(() => new Promise((resolve) => { resolvePublication = resolve }))
    const { router } = await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    await router.push(`/authoring/drafts/${draftID}/reviews/${secondReviewID}`)
    expect(await screen.findByText('Second frozen snapshot')).toBeTruthy()
    resolvePublication(publicationResult(firstReviewID, 5))
    await waitFor(() => expect(screen.queryByText('First frozen snapshot')).toBeNull())
    expect(getAuthoringDraftReviewMock).toHaveBeenCalledTimes(2)
    expect(screen.queryByRole('heading', { level: 4, name: 'Course version published' })).toBeNull()
    expect(document.activeElement?.id).not.toBe('authoring-review-publication-feedback-title')
  })

  it('does not let a late post-publication refresh overwrite a newly selected Review', async () => {
    const first = detail(firstReviewID, 'First frozen snapshot', 'APPROVED', 5)
    const second = detail(secondReviewID, 'Second frozen snapshot', 'APPROVED', 7)
    let resolveFirstRefresh: (value: AuthoringReviewDetail) => void = () => undefined
    getAuthoringDraftReviewMock.mockResolvedValueOnce(first)
      .mockImplementationOnce(() => new Promise<AuthoringReviewDetail>((resolve) => { resolveFirstRefresh = resolve }))
      .mockResolvedValueOnce(second)
    publishAuthoringDraftReviewMock.mockResolvedValueOnce(publicationResult(firstReviewID, 5))
    const { router } = await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    await waitFor(() => expect(getAuthoringDraftReviewMock).toHaveBeenCalledTimes(2))
    await router.push(`/authoring/drafts/${draftID}/reviews/${secondReviewID}`)
    expect(await screen.findByText('Second frozen snapshot')).toBeTruthy()
    resolveFirstRefresh(publishedDetail(firstReviewID, 'First published snapshot', 5))
    await waitFor(() => expect(screen.queryByText('First published snapshot')).toBeNull())
    expect(screen.queryByRole('heading', { level: 4, name: 'Published course version' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Publish course version' })).toBeTruthy()
    expect(document.activeElement?.id).not.toBe('authoring-review-published-title')
  })

  it('does not let a late validation failure render, announce, or focus on a new Review route', async () => {
    const first = detail(firstReviewID, 'First frozen snapshot', 'APPROVED', 5)
    const second = detail(secondReviewID, 'Second frozen snapshot', 'APPROVED', 7)
    getAuthoringDraftReviewMock.mockResolvedValueOnce(first).mockResolvedValueOnce(second)
    let rejectPublication: (reason: unknown) => void = () => undefined
    publishAuthoringDraftReviewMock.mockImplementationOnce(() => new Promise((_, reject) => { rejectPublication = reject }))
    const { router } = await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    await router.push(`/authoring/drafts/${draftID}/reviews/${secondReviewID}`)
    expect(await screen.findByText('Second frozen snapshot')).toBeTruthy()
    rejectPublication(new APIProblemError(422, {
      type: 'https://academy.example/problems/publication-validation-failed', title: 'Publication validation failed', status: 422,
      instance: '/publish', request_id: 'request-id', code: 'publication_validation_failed',
      issues: [{ code: 'unresolved_asset_reference', path: 'old-review', message: 'Old Review issue.' }],
    }, undefined))
    await waitFor(() => expect(screen.queryByText('Old Review issue.')).toBeNull())
    expect(screen.queryByRole('heading', { level: 4, name: 'Publication validation issues' })).toBeNull()
    expect(document.activeElement?.id).not.toBe('authoring-review-publication-feedback-title')
  })

  it('clears rendered publication feedback when the exact Review route changes', async () => {
    const first = detail(firstReviewID, 'First frozen snapshot', 'APPROVED', 5)
    const second = detail(secondReviewID, 'Second frozen snapshot', 'APPROVED', 7)
    getAuthoringDraftReviewMock.mockResolvedValueOnce(first).mockResolvedValueOnce(first).mockResolvedValueOnce(second)
    publishAuthoringDraftReviewMock.mockRejectedValueOnce(new APIProblemError(422, {
      type: 'https://academy.example/problems/publication-validation-failed', title: 'Publication validation failed', status: 422,
      instance: '/publish', request_id: 'request-id', code: 'publication_validation_failed',
      issues: [{ code: 'unresolved_asset_reference', path: 'first-review', message: 'First Review issue.' }],
    }, undefined))
    const { router } = await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    expect(await screen.findByText('First Review issue.')).toBeTruthy()
    await router.push(`/authoring/drafts/${draftID}/reviews/${secondReviewID}`)
    expect(await screen.findByText('Second frozen snapshot')).toBeTruthy()
    expect(screen.queryByText('First Review issue.')).toBeNull()
    expect(screen.queryByRole('heading', { level: 4, name: 'Publication validation issues' })).toBeNull()
  })

  it('keeps a hidden publication denial opaque', async () => {
    publishAuthoringDraftReviewMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    const page = await renderPage()
    await (await screen.findByRole('button', { name: 'Publish course version' })).click()
    await waitFor(() => expect(page.markDraftUnavailable).toHaveBeenCalledTimes(1))
    expect(screen.queryByText(/authoring.publish|permission|maintainer/i)).toBeNull()
  })

  it('approves using the exact authoritative Review revision then reloads the frozen Review', async () => {
    const inReview = detail(firstReviewID, 'Frozen integration foundations', 'IN_REVIEW', 5)
    const approved = detail(firstReviewID, 'Frozen integration foundations', 'APPROVED', 6)
    getAuthoringDraftReviewMock.mockResolvedValueOnce(inReview).mockResolvedValueOnce(approved)
    approveAuthoringReviewMock.mockResolvedValueOnce(approved.review)
    await renderPage()
    const approve = await screen.findByRole('button', { name: 'Approve' })
    approve.focus()
    await approve.click()
    expect(approveAuthoringReviewMock).toHaveBeenCalledWith(draftID, firstReviewID, { expectedReviewRevision: 5 })
    expect(await screen.findByText('Approved Review is not a published Course.')).toBeTruthy()
    expect(screen.getByText('Frozen integration foundations')).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Request changes' })).toBeNull()
    expect(getAuthoringDraftReviewMock).toHaveBeenCalledTimes(2)
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('heading', { level: 2, name: 'Reviewing Draft revision 4' })))
    expect(screen.getByRole('status').textContent).toContain('Review approved')
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
    expect(screen.getByRole('status').textContent).toContain('Changes requested')
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

  it('maps the same structured policy conflict for request changes', async () => {
    getAuthoringDraftReviewMock.mockResolvedValueOnce(detail(firstReviewID, 'Frozen integration foundations', 'IN_REVIEW', 5))
    requestAuthoringReviewChangesMock.mockRejectedValueOnce(new APIProblemError(409, {
      type: 'https://academy.example/problems/independent-reviewer-required', title: 'Independent reviewer required', status: 409,
      instance: '/api/authoring/drafts/example/reviews/example/request-changes', request_id: 'request-id', code: 'independent_reviewer_required',
    }, undefined))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Request changes' })).click()
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

  it('does not let a late conflict reload overwrite a newly selected Review', async () => {
    const first = detail(firstReviewID, 'First frozen snapshot', 'IN_REVIEW', 5)
    const second = detail(secondReviewID, 'Second frozen snapshot', 'IN_REVIEW', 7)
    let resolveReload: (value: AuthoringReviewDetail) => void = () => undefined
    getAuthoringDraftReviewMock.mockResolvedValueOnce(first)
      .mockImplementationOnce(() => new Promise<AuthoringReviewDetail>((resolve) => { resolveReload = resolve }))
      .mockResolvedValueOnce(second)
    approveAuthoringReviewMock.mockRejectedValueOnce(new APIProblemError(409, undefined, undefined))
    const { router } = await renderPage()
    await (await screen.findByRole('button', { name: 'Approve' })).click()
    await screen.findByText('This Review changed before your decision was saved. Reload the latest Review state.')
    await screen.getByRole('button', { name: 'Reload review' }).click()
    await router.push(`/authoring/drafts/${draftID}/reviews/${secondReviewID}`)
    expect(await screen.findByText('Second frozen snapshot')).toBeTruthy()
    resolveReload(detail(firstReviewID, 'Stale reloaded snapshot', 'APPROVED', 6))
    await waitFor(() => expect(screen.queryByText('Stale reloaded snapshot')).toBeNull())
    expect(screen.getByText('Second frozen snapshot')).toBeTruthy()
  })
})
