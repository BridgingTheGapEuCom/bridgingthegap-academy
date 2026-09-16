import { cleanup, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { shallowRef } from 'vue'
import { APIProblemError } from '../api/client'
import { authoringDraftContextKey } from '../authoring/draftContext'
import AuthoringDraftReviewPage from './AuthoringDraftReviewPage.vue'

type AuthenticationState =
  | { status: 'bootstrapping' }
  | { status: 'unauthenticated' }
  | { status: 'authenticated'; userId: string; expiresAt: string }
  | { status: 'unavailable' }

const authMock = vi.hoisted(() => ({ state: { value: { status: 'authenticated' } as AuthenticationState } }))
const getAuthoringDraftReviewHistoryMock = vi.hoisted(() => vi.fn())
const getAuthoringActiveDraftReviewMock = vi.hoisted(() => vi.fn())
const getAuthoringLatestDraftReviewMock = vi.hoisted(() => vi.fn())
const getAuthoringDraftMock = vi.hoisted(() => vi.fn())
const submitAuthoringDraftReviewMock = vi.hoisted(() => vi.fn())

vi.mock('../auth/auth', () => ({ useAuth: () => authMock }))
vi.mock('../authoring/authoring', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../authoring/authoring')>()),
  getAuthoringDraftReviewHistory: getAuthoringDraftReviewHistoryMock,
  getAuthoringActiveDraftReview: getAuthoringActiveDraftReviewMock,
  getAuthoringLatestDraftReview: getAuthoringLatestDraftReviewMock,
  getAuthoringDraft: getAuthoringDraftMock,
  submitAuthoringDraftReview: submitAuthoringDraftReviewMock,
}))

const firstID = '11111111-1111-4111-8111-111111111111'
const secondID = '22222222-2222-4222-8222-222222222222'
const actorA = '33333333-3333-4333-8333-333333333333'
const actorB = '44444444-4444-4444-8444-444444444444'

const draft = (id = firstID) => ({
  id,
  course_id: '55555555-5555-4555-8555-555555555555',
  intended_version: '1.0.0', source_language: 'en', title: id === firstID ? 'First Draft' : 'Second Draft',
  description: '', objectives: [], changelog: '', license: { kind: 'STANDARD' as const, display_name: 'Standard license' },
  status: 'ACTIVE' as const, revision: 4, created_at: '2026-09-15T10:00:00Z', updated_at: '2026-09-15T11:00:00Z',
})

const review = (id: string, status: 'IN_REVIEW' | 'APPROVED' | 'CHANGES_REQUESTED', revision: number, submittedBy = actorA) => ({
  id,
  draftId: firstID,
  draftRevision: revision,
  snapshotSchemaVersion: 1 as const,
  status,
  reviewRevision: 1,
  submittedBy,
  submittedAt: '2026-09-15T12:00:00Z',
  decidedBy: status === 'IN_REVIEW' ? null : actorB,
  decidedAt: status === 'IN_REVIEW' ? null : '2026-09-15T13:00:00Z',
})

async function renderPage(initialDraft = draft()) {
  const currentDraft = shallowRef(initialDraft)
  const markDraftUnavailable = vi.fn()
  const replaceDraft = vi.fn((nextDraft) => { currentDraft.value = nextDraft })
  const result = render(AuthoringDraftReviewPage, {
    global: { provide: { [authoringDraftContextKey as symbol]: { draft: currentDraft, markDraftUnavailable, replaceDraft } } },
  })
  return { ...result, currentDraft, markDraftUnavailable }
}

describe('AuthoringDraftReviewPage', () => {
  beforeEach(() => {
    authMock.state = shallowRef<AuthenticationState>({ status: 'authenticated', userId: actorA, expiresAt: '2026-09-15T16:00:00Z' })
    getAuthoringDraftReviewHistoryMock.mockReset().mockResolvedValue({ reviews: [] })
    getAuthoringActiveDraftReviewMock.mockReset().mockRejectedValue(new APIProblemError(404, undefined, undefined))
    getAuthoringLatestDraftReviewMock.mockReset().mockRejectedValue(new APIProblemError(404, undefined, undefined))
    getAuthoringDraftMock.mockReset().mockResolvedValue(draft())
    submitAuthoringDraftReviewMock.mockReset()
  })

  afterEach(cleanup)

  it('shows the derived Editing state for an accessible Draft with no Review cycles', async () => {
    await renderPage()
    expect(await screen.findByText('Editing')).toBeTruthy()
    expect(screen.getByText(/No Review cycle has been submitted yet/)).toBeTruthy()
    expect(screen.getByText('No Review cycles yet.')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Submit for review' })).toBeTruthy()
    expect(getAuthoringDraftReviewHistoryMock).toHaveBeenCalledWith(firstID)
    expect(getAuthoringActiveDraftReviewMock).toHaveBeenCalledWith(firstID)
    expect(getAuthoringLatestDraftReviewMock).toHaveBeenCalledWith(firstID)
  })

  it('renders active Review metadata without fetching a snapshot', async () => {
    const active = review('66666666-6666-4666-8666-666666666666', 'IN_REVIEW', 4)
    getAuthoringDraftReviewHistoryMock.mockResolvedValue({ reviews: [active] })
    getAuthoringActiveDraftReviewMock.mockResolvedValue(active)
    getAuthoringLatestDraftReviewMock.mockResolvedValue(active)
    await renderPage()
    expect((await screen.findAllByText('In review')).length).toBeGreaterThan(0)
    expect(screen.getAllByText('Draft revision under review')[0]?.nextElementSibling?.textContent).toBe('4')
    expect(screen.getAllByText(actorA).length).toBeGreaterThan(0)
    expect(screen.getByRole('list', { name: 'Review history' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Submit for review' })).toBeNull()
  })

  it.each([
    ['APPROVED', 'Approved'],
    ['CHANGES_REQUESTED', 'Changes requested'],
  ] as const)('renders latest terminal %s state accurately', async (status, label) => {
    const latest = review('66666666-6666-4666-8666-666666666666', status, 3)
    getAuthoringDraftReviewHistoryMock.mockResolvedValue({ reviews: [latest] })
    getAuthoringLatestDraftReviewMock.mockResolvedValue(latest)
    await renderPage()
    expect((await screen.findAllByText(label)).length).toBeGreaterThan(0)
    expect(screen.getByRole('button', { name: 'Submit for review' })).toBeTruthy()
    if (status === 'APPROVED') {
      expect(screen.getByText(/approved, but the Draft has not been published/i)).toBeTruthy()
      expect(screen.queryByText('Published')).toBeNull()
    }
  })

  it('preserves newest-first history metadata without loading review snapshots', async () => {
    const latest = review('66666666-6666-4666-8666-666666666666', 'CHANGES_REQUESTED', 5, actorB)
    const earlier = review('77777777-7777-4777-8777-777777777777', 'APPROVED', 3)
    getAuthoringDraftReviewHistoryMock.mockResolvedValue({ reviews: [latest, earlier] })
    getAuthoringLatestDraftReviewMock.mockResolvedValue(latest)
    await renderPage()
    const history = await screen.findByRole('list', { name: 'Review history' })
    expect(history.textContent).toContain('Changes requested')
    expect(history.textContent).toContain('Approved')
    expect(history.querySelectorAll('li')).toHaveLength(2)
    expect(getAuthoringDraftReviewHistoryMock).toHaveBeenCalledTimes(1)
    expect(getAuthoringActiveDraftReviewMock).toHaveBeenCalledTimes(1)
    expect(getAuthoringLatestDraftReviewMock).toHaveBeenCalledTimes(1)
  })

  it('treats an active-only 404 as no active Review, but keeps scoped history 404 opaque', async () => {
    const latest = review('66666666-6666-4666-8666-666666666666', 'APPROVED', 3)
    getAuthoringDraftReviewHistoryMock.mockResolvedValueOnce({ reviews: [latest] })
    getAuthoringLatestDraftReviewMock.mockResolvedValueOnce(latest)
    const ready = await renderPage()
    expect((await screen.findAllByText('Approved')).length).toBeGreaterThan(0)
    expect(ready.markDraftUnavailable).not.toHaveBeenCalled()

    cleanup()
    getAuthoringDraftReviewHistoryMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    const hidden = await renderPage()
    await waitFor(() => expect(hidden.markDraftUnavailable).toHaveBeenCalledTimes(1))
  })

  it('shows a recoverable operational failure without treating it as an empty history', async () => {
    getAuthoringDraftReviewHistoryMock.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ reviews: [] })
    await renderPage()
    expect(await screen.findByText('We couldn’t load Review information right now.')).toBeTruthy()
    await screen.getByRole('button', { name: 'Try again' }).click()
    expect(await screen.findByText('No Review cycles yet.')).toBeTruthy()
  })

  it('does not display a late Review response after the Draft context changes', async () => {
    let resolveFirst: (value: { reviews: ReturnType<typeof review>[] }) => void = () => undefined
    getAuthoringDraftReviewHistoryMock.mockImplementationOnce(() => new Promise((resolve) => { resolveFirst = resolve }))
    getAuthoringDraftReviewHistoryMock.mockResolvedValueOnce({ reviews: [] })
    const { currentDraft } = await renderPage()
    await waitFor(() => expect(getAuthoringDraftReviewHistoryMock).toHaveBeenCalledTimes(1))
    currentDraft.value = draft(secondID)
    await screen.findByText('No Review cycles yet.')
    resolveFirst({ reviews: [review('66666666-6666-4666-8666-666666666666', 'IN_REVIEW', 4)] })
    await waitFor(() => expect(screen.queryByText('In review')).toBeNull())
  })

  it('submits the current authoritative Draft revision and refreshes Review state without a local merge', async () => {
    const submitted = review('66666666-6666-4666-8666-666666666666', 'IN_REVIEW', 4)
    getAuthoringDraftReviewHistoryMock.mockResolvedValueOnce({ reviews: [] }).mockResolvedValueOnce({ reviews: [submitted] })
    getAuthoringActiveDraftReviewMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined)).mockResolvedValueOnce(submitted)
    getAuthoringLatestDraftReviewMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined)).mockResolvedValueOnce(submitted)
    submitAuthoringDraftReviewMock.mockResolvedValue({ review: submitted, snapshot: {} })
    await renderPage()
    await (await screen.findByRole('button', { name: 'Submit for review' })).click()
    expect(submitAuthoringDraftReviewMock).toHaveBeenCalledWith(firstID, { expectedDraftRevision: 4 })
    expect(await screen.findByText('Draft submitted for review.')).toBeTruthy()
    expect((await screen.findAllByText('In review')).length).toBeGreaterThan(0)
    expect(screen.queryByRole('button', { name: 'Submit for review' })).toBeNull()
    expect(getAuthoringDraftReviewHistoryMock).toHaveBeenCalledTimes(2)
  })

  it('uses a newer shared Draft revision and prevents duplicate pending submissions', async () => {
    let resolveSubmission: (value: unknown) => void = () => undefined
    submitAuthoringDraftReviewMock.mockImplementationOnce(() => new Promise((resolve) => { resolveSubmission = resolve }))
    const { currentDraft } = await renderPage()
    currentDraft.value = { ...draft(), revision: 9 }
    const submit = await screen.findByRole('button', { name: 'Submit for review' })
    await submit.click()
    await submit.click()
    expect(screen.getByRole('button', { name: 'Submitting…' })).toBeTruthy()
    expect(screen.getByText('Submitting the current Draft revision for review…')).toBeTruthy()
    expect(submitAuthoringDraftReviewMock).toHaveBeenCalledTimes(1)
    expect(submitAuthoringDraftReviewMock).toHaveBeenCalledWith(firstID, { expectedDraftRevision: 9 })
    resolveSubmission({ review: review('66666666-6666-4666-8666-666666666666', 'IN_REVIEW', 9), snapshot: {} })
    await screen.findByText('No Review cycles yet.')
  })

  it('does not let a late submission response affect a newly selected Draft', async () => {
    let resolveSubmission: (value: unknown) => void = () => undefined
    submitAuthoringDraftReviewMock.mockImplementationOnce(() => new Promise((resolve) => { resolveSubmission = resolve }))
    const { currentDraft } = await renderPage()
    await (await screen.findByRole('button', { name: 'Submit for review' })).click()
    currentDraft.value = draft(secondID)
    expect(await screen.findByRole('button', { name: 'Submit for review' })).toBeTruthy()
    resolveSubmission({ review: review('66666666-6666-4666-8666-666666666666', 'IN_REVIEW', 4), snapshot: {} })
    await waitFor(() => expect(screen.queryByText('In review')).toBeNull())
  })

  it('preserves authoritative state after a submission conflict until explicit reload', async () => {
    submitAuthoringDraftReviewMock.mockRejectedValueOnce(new APIProblemError(409, undefined, undefined))
    getAuthoringDraftMock.mockResolvedValueOnce({ ...draft(), revision: 8 })
    await renderPage()
    await (await screen.findByRole('button', { name: 'Submit for review' })).click()
    expect(await screen.findByText(/Review or Draft changed before submission/)).toBeTruthy()
    expect(getAuthoringDraftReviewHistoryMock).toHaveBeenCalledTimes(1)
    await screen.getByRole('button', { name: 'Reload review state' }).click()
    await waitFor(() => expect(getAuthoringDraftMock).toHaveBeenCalledWith(firstID))
    expect(await screen.findByText('The latest Draft and Review state have been loaded.')).toBeTruthy()
    expect(submitAuthoringDraftReviewMock).toHaveBeenCalledTimes(1)
  })

  it('keeps the empty authoritative state when submission fails operationally', async () => {
    submitAuthoringDraftReviewMock.mockRejectedValueOnce(new Error('offline'))
    await renderPage()
    await (await screen.findByRole('button', { name: 'Submit for review' })).click()
    expect(await screen.findByText(/couldn’t submit this Draft for review right now/)).toBeTruthy()
    expect(screen.getByText('No Review cycles yet.')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Submit for review' })).toBeTruthy()
  })

  it('keeps a hidden submission 404 opaque', async () => {
    submitAuthoringDraftReviewMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    const page = await renderPage()
    await (await screen.findByRole('button', { name: 'Submit for review' })).click()
    await waitFor(() => expect(page.markDraftUnavailable).toHaveBeenCalledTimes(1))
  })
})
