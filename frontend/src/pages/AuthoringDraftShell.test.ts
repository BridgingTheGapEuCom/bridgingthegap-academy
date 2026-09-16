import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { nextTick, shallowRef } from 'vue'
import { APIProblemError } from '../api/client'

type AuthenticationState =
  | { status: 'bootstrapping' }
  | { status: 'unauthenticated' }
  | { status: 'authenticated'; userId: string; expiresAt: string }
  | { status: 'unavailable' }

const authMock = vi.hoisted(() => ({
  state: { value: { status: 'authenticated' } as AuthenticationState },
  request: vi.fn(),
  bootstrapSession: vi.fn(),
}))
const getAuthoringDraftMock = vi.hoisted(() => vi.fn())
const updateAuthoringDraftMock = vi.hoisted(() => vi.fn())
const createAuthoringModuleMock = vi.hoisted(() => vi.fn())
const getAuthoringStructureMock = vi.hoisted(() => vi.fn())

vi.mock('../auth/auth', () => ({ useAuth: () => authMock }))
vi.mock('../authoring/authoring', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../authoring/authoring')>()),
  getAuthoringDraft: getAuthoringDraftMock,
  updateAuthoringDraft: updateAuthoringDraftMock,
  getAuthoringStructure: getAuthoringStructureMock,
  createAuthoringModule: createAuthoringModuleMock,
}))

import AuthoringDraftShell from './AuthoringDraftShell.vue'
import AuthoringDraftOverviewPage from './AuthoringDraftOverviewPage.vue'
import AuthoringDraftStructurePage from './AuthoringDraftStructurePage.vue'
import AuthoringDraftMembersPage from './AuthoringDraftMembersPage.vue'

const firstID = '11111111-1111-4111-8111-111111111111'
const secondID = '22222222-2222-4222-8222-222222222222'
const draft = (id = firstID, title = 'Integration foundations') => ({
  id,
  course_id: '33333333-3333-4333-8333-333333333333',
  intended_version: '1.0.0',
  source_language: 'en',
  title,
  description: 'A mutable working version.',
  objectives: ['Explain ownership'],
  changelog: 'Initial Draft.',
  license: { kind: 'STANDARD', display_name: 'Creative Commons Attribution 4.0' },
  status: 'ACTIVE' as const,
  revision: 3,
  created_at: '2026-09-15T10:00:00Z',
  updated_at: '2026-09-15T11:00:00Z',
})

function routerFor() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<p>Home</p>' } },
      { path: '/login', component: { template: '<p>Login</p>' } },
      {
        path: '/authoring/drafts/:draftId', component: AuthoringDraftShell,
        children: [
          { path: '', redirect: (to) => ({ name: 'overview', params: { draftId: to.params.draftId } }) },
          { path: 'overview', name: 'overview', component: AuthoringDraftOverviewPage },
          { path: 'structure', name: 'structure', component: AuthoringDraftStructurePage },
          { path: 'members', name: 'members', component: AuthoringDraftMembersPage },
          { path: 'review', name: 'review', component: { template: '<h2>Review</h2>' } },
        ],
      },
    ],
  })
}

async function renderShell(path = `/authoring/drafts/${firstID}`) {
  const router = routerFor()
  await router.push(path)
  await router.isReady()
  return { ...render({ template: '<RouterView />' }, { global: { plugins: [router] } }), router }
}

describe('AuthoringDraftShell', () => {
  beforeEach(() => {
    authMock.state = shallowRef<AuthenticationState>({ status: 'authenticated', userId: '44444444-4444-4444-8444-444444444444', expiresAt: '2026-09-15T12:00:00Z' })
    authMock.bootstrapSession.mockReset()
    getAuthoringDraftMock.mockReset()
    updateAuthoringDraftMock.mockReset()
    getAuthoringStructureMock.mockReset()
    createAuthoringModuleMock.mockReset()
    getAuthoringDraftMock.mockResolvedValue(draft())
    getAuthoringStructureMock.mockResolvedValue({ modules: [] })
  })

  afterEach(cleanup)

  it('loads Draft context before rendering shell navigation and exposes the current section semantically', async () => {
    let resolveDraft: (value: ReturnType<typeof draft>) => void = () => undefined
    getAuthoringDraftMock.mockImplementationOnce(() => new Promise<ReturnType<typeof draft>>((resolve) => { resolveDraft = resolve }))
    await renderShell()
    expect(screen.getByRole('status').textContent).toContain('Loading draft')
    resolveDraft(draft())
    await screen.findByRole('heading', { level: 1, name: 'Integration foundations' })
    expect(screen.getAllByText('Intended version')[0]?.nextElementSibling?.textContent).toBe('1.0.0')
    const overview = screen.getByRole('link', { name: 'Overview' })
    expect(overview.getAttribute('aria-current')).toBe('page')
    expect(screen.getByRole('heading', { level: 2, name: 'Overview' })).toBeTruthy()
    expect(screen.getByRole('navigation', { name: 'Draft sections' })).toBeTruthy()
  })

  it('navigates between the shell sections using real keyboard-operable links', async () => {
    const { router } = await renderShell()
    await screen.findByRole('heading', { level: 1, name: 'Integration foundations' })
    const structure = screen.getByRole('link', { name: 'Structure' })
    structure.focus()
    await fireEvent.click(structure)
    await waitFor(() => expect(router.currentRoute.value.path).toBe(`/authoring/drafts/${firstID}/structure`))
    expect(screen.getByRole('heading', { level: 2, name: 'Structure' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Structure' }).getAttribute('aria-current')).toBe('page')

    const review = screen.getByRole('link', { name: 'Review' })
    review.focus()
    await fireEvent.click(review)
    await waitFor(() => expect(router.currentRoute.value.path).toBe(`/authoring/drafts/${firstID}/review`))
    expect(screen.getByRole('link', { name: 'Review' }).getAttribute('aria-current')).toBe('page')
  })

  it('keeps unavailable and hidden Draft responses distinct without revealing access details', async () => {
    getAuthoringDraftMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    await renderShell()
    expect(await screen.findByRole('heading', { level: 1, name: 'Draft unavailable' })).toBeTruthy()
    expect(document.body.textContent).not.toContain('membership')

    cleanup()
    getAuthoringDraftMock.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(draft())
    await renderShell()
    await screen.findByRole('heading', { level: 1, name: 'Authoring unavailable' })
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Integration foundations' })).toBeTruthy()
  })

  it('redirects unauthenticated Draft routes to sign in without loading the Draft', async () => {
    authMock.state.value = { status: 'unauthenticated' }
    const { router } = await renderShell()
    await waitFor(() => expect(router.currentRoute.value.path).toBe('/login'))
    expect(getAuthoringDraftMock).not.toHaveBeenCalled()
  })

  it('does not retain a late response from a previous Draft route', async () => {
    let resolveFirst: (value: ReturnType<typeof draft>) => void = () => undefined
    getAuthoringDraftMock.mockImplementationOnce(() => new Promise<ReturnType<typeof draft>>((resolve) => { resolveFirst = resolve }))
    getAuthoringDraftMock.mockResolvedValueOnce(draft(secondID, 'Second Draft'))
    const { router } = await renderShell(`/authoring/drafts/${firstID}/overview`)
    await waitFor(() => expect(getAuthoringDraftMock).toHaveBeenCalledTimes(1))
    await router.push(`/authoring/drafts/${secondID}/overview`)
    await screen.findByRole('heading', { level: 1, name: 'Second Draft' })
    resolveFirst(draft(firstID, 'Old Draft'))
    await waitFor(() => expect(screen.queryByRole('heading', { level: 1, name: 'Old Draft' })).toBeNull())
  })

  it('edits only changed metadata with the authoritative revision and updates the shell after save', async () => {
    const updated = { ...draft(), title: 'Updated foundations', revision: 4, updated_at: '2026-09-15T12:00:00Z' }
    updateAuthoringDraftMock.mockResolvedValue(updated)
    await renderShell()
    const title = await screen.findByRole('textbox', { name: /Title/ })
    await fireEvent.update(title, 'Updated foundations')
    const save = screen.getByRole('button', { name: 'Save changes' })
    expect(save.hasAttribute('disabled')).toBe(false)
    await fireEvent.click(save)
    await waitFor(() => expect(updateAuthoringDraftMock).toHaveBeenCalledWith(firstID, { expectedRevision: 3, title: 'Updated foundations' }))
    expect(screen.getByRole('heading', { level: 1, name: 'Updated foundations' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Save changes' }).hasAttribute('disabled')).toBe(true)
  })

  it('preserves user input on a stale revision and reloads only when explicitly requested', async () => {
    updateAuthoringDraftMock.mockRejectedValueOnce(new APIProblemError(409, undefined, undefined))
    getAuthoringDraftMock.mockResolvedValueOnce(draft()).mockResolvedValueOnce({ ...draft(), title: 'Latest Draft', revision: 4 })
    await renderShell()
    const title = await screen.findByRole('textbox', { name: /Title/ })
    await fireEvent.update(title, 'My local title')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(await screen.findByText(/Draft changed elsewhere/)).toBeTruthy()
    expect((title as HTMLInputElement).value).toBe('My local title')
    expect(updateAuthoringDraftMock).toHaveBeenCalledTimes(1)
    await fireEvent.click(screen.getByRole('button', { name: 'Reload latest Draft' }))
    await waitFor(() => expect((screen.getByRole('textbox', { name: /Title/ }) as HTMLInputElement).value).toBe('Latest Draft'))
  })

  it('keeps Save disabled for unchanged or reverted metadata and reports client validation accessibly', async () => {
    await renderShell()
    const title = await screen.findByRole('textbox', { name: /Title/ })
    const save = screen.getByRole('button', { name: 'Save changes' })
    expect(save.hasAttribute('disabled')).toBe(true)
    await fireEvent.update(title, 'Temporary title')
    await fireEvent.update(title, 'Integration foundations')
    expect(save.hasAttribute('disabled')).toBe(true)
    await fireEvent.update(title, '')
    await fireEvent.click(save)
    expect(await screen.findByText('Enter a title.')).toBeTruthy()
    expect(title.getAttribute('aria-invalid')).toBe('true')
    expect(updateAuthoringDraftMock).not.toHaveBeenCalled()
  })

  it('keeps local values for an operational save failure and hides a Draft after an opaque mutation 404', async () => {
    updateAuthoringDraftMock.mockRejectedValueOnce(new Error('offline'))
    await renderShell()
    const title = await screen.findByRole('textbox', { name: /Title/ })
    await fireEvent.update(title, 'Local title')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    expect((await screen.findByRole('alert')).textContent).toContain('We couldn’t save this Draft right now.')
    expect((title as HTMLInputElement).value).toBe('Local title')

    cleanup()
    updateAuthoringDraftMock.mockRejectedValueOnce(new APIProblemError(404, undefined, undefined))
    await renderShell()
    await fireEvent.update(await screen.findByRole('textbox', { name: /Title/ }), 'Hidden Draft')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Draft unavailable' })).toBeTruthy()
  })
  it.each(['success', 'hidden-404'])('ignores a late metadata %s after switching Drafts', async (outcome) => {
    let resolveSave!: (value: ReturnType<typeof draft>) => void
    let rejectSave!: (error: Error) => void
    updateAuthoringDraftMock.mockImplementationOnce(() => new Promise((resolve, reject) => { resolveSave = resolve; rejectSave = reject }))
    getAuthoringDraftMock.mockResolvedValueOnce(draft()).mockResolvedValueOnce(draft(secondID, 'Second Draft'))
    const { router } = await renderShell()
    await fireEvent.update(await screen.findByRole('textbox', { name: /Title/ }), 'Old local edits')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await router.push(`/authoring/drafts/${secondID}/overview`)
    await screen.findByRole('heading', { level: 1, name: 'Second Draft' })
    if (outcome === 'success') resolveSave({ ...draft(), title: 'Old response', revision: 4 })
    else rejectSave(new APIProblemError(404, undefined, undefined))
    await nextTick()
    expect(screen.getByRole('heading', { level: 1, name: 'Second Draft' })).toBeTruthy()
    expect(screen.queryByText('Draft unavailable')).toBeNull()
    expect(screen.queryByDisplayValue('Old response')).toBeNull()
  })

  it('clears private Draft context when another account becomes authenticated and ignores the old save', async () => {
    let resolveSave!: (value: ReturnType<typeof draft>) => void
    updateAuthoringDraftMock.mockImplementationOnce(() => new Promise((resolve) => { resolveSave = resolve }))
    getAuthoringDraftMock.mockResolvedValueOnce(draft()).mockResolvedValueOnce({ ...draft(), title: 'New account Draft', revision: 8 })
    await renderShell()
    await fireEvent.update(await screen.findByRole('textbox', { name: /Title/ }), 'Private old edits')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    authMock.state.value = { status: 'authenticated', userId: '55555555-5555-4555-8555-555555555555', expiresAt: 'later' }
    await screen.findByRole('heading', { level: 1, name: 'New account Draft' })
    resolveSave({ ...draft(), title: 'Previous account response', revision: 9 })
    await nextTick()
    expect(screen.getByRole('heading', { level: 1, name: 'New account Draft' })).toBeTruthy()
    expect(screen.queryByDisplayValue('Private old edits')).toBeNull()
  })

  it('shares authoritative Draft revisions across metadata and structure mutations', async () => {
    updateAuthoringDraftMock.mockResolvedValueOnce({ ...draft(), title: 'Saved Draft', revision: 10 })
      .mockResolvedValueOnce({ ...draft(), title: 'Saved again', revision: 21 })
    createAuthoringModuleMock.mockResolvedValue({ draftRevision: 20 })
    const { router } = await renderShell()
    await fireEvent.update(await screen.findByRole('textbox', { name: /Title/ }), 'Saved Draft')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await screen.findByRole('heading', { level: 1, name: 'Saved Draft' })
    await router.push(`/authoring/drafts/${firstID}/structure`)
    await fireEvent.update(await screen.findByRole('textbox', { name: /Module stable key/ }), 'new-module')
    await fireEvent.update(screen.getByRole('textbox', { name: /Module title/ }), 'New Module')
    await fireEvent.click(screen.getByRole('button', { name: 'Create module' }))
    await screen.findByText('Module created.')
    expect(createAuthoringModuleMock.mock.calls[0]![1].expectedDraftRevision).toBe(10)
    await router.push(`/authoring/drafts/${firstID}/overview`)
    await fireEvent.update(await screen.findByRole('textbox', { name: /Title/ }), 'Saved again')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(updateAuthoringDraftMock).toHaveBeenLastCalledWith(firstID, { expectedRevision: 20, title: 'Saved again' }))
  })

})
