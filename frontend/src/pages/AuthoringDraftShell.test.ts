import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { shallowRef } from 'vue'
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

vi.mock('../auth/auth', () => ({ useAuth: () => authMock }))
vi.mock('../authoring/authoring', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../authoring/authoring')>()),
  getAuthoringDraft: getAuthoringDraftMock,
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
    getAuthoringDraftMock.mockResolvedValue(draft())
  })

  afterEach(cleanup)

  it('loads Draft context before rendering shell navigation and exposes the current section semantically', async () => {
    let resolveDraft: (value: ReturnType<typeof draft>) => void = () => undefined
    getAuthoringDraftMock.mockImplementationOnce(() => new Promise<ReturnType<typeof draft>>((resolve) => { resolveDraft = resolve }))
    await renderShell()
    expect(screen.getByRole('status').textContent).toContain('Loading draft')
    resolveDraft(draft())
    await screen.findByRole('heading', { level: 1, name: 'Integration foundations' })
    expect(screen.getByText('Intended version').nextElementSibling?.textContent).toBe('1.0.0')
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
})
