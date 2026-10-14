import { cleanup, fireEvent, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { shallowRef } from 'vue'

type AuthenticationState =
  | { status: 'bootstrapping' }
  | { status: 'unauthenticated' }
  | { status: 'authenticated'; userId: string; expiresAt: string }
  | { status: 'unavailable' }

const authMock = vi.hoisted(() => ({ state: { value: { status: 'authenticated' } as AuthenticationState }, bootstrapSession: vi.fn() }))
const listAuthoringDraftsMock = vi.hoisted(() => vi.fn())

vi.mock('../auth/auth', () => ({ useAuth: () => authMock }))
vi.mock('../authoring/authoring', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../authoring/authoring')>()),
  listAuthoringDrafts: listAuthoringDraftsMock,
}))

import AuthoringHomePage from './AuthoringHomePage.vue'
import { pseudoLocalize } from '../i18n/pseudo'
import { renderWithI18n } from '../test/i18n'

const draftID = '11111111-1111-4111-8111-111111111111'
const secondDraftID = '33333333-3333-4333-8333-333333333333'
const drafts = () => ({ drafts: [
  { id: draftID, title: 'Integration foundations', intendedVersion: '1.0.0', status: 'ACTIVE' as const, updatedAt: '2026-09-15T11:00:00Z' },
  { id: secondDraftID, title: 'Messaging patterns', intendedVersion: '1.1.0', status: 'ABANDONED' as const, updatedAt: '2026-09-15T10:00:00Z' },
] })

function routerFor() {
  return createRouter({ history: createMemoryHistory(), routes: [
    { path: '/authoring', component: AuthoringHomePage },
    { path: '/authoring/new', component: { template: '<p>Create Draft page</p>' } },
    { path: '/authoring/drafts/:draftId/overview', component: { template: '<p>Draft workspace</p>' } },
    { path: '/login', component: { template: '<p>Login</p>' } },
  ] })
}

async function renderPage(locale: 'en' | 'en-XA' = 'en') {
  const router = routerFor()
  await router.push('/authoring')
  await router.isReady()
  return { ...renderWithI18n({ template: '<RouterView />' }, { global: { plugins: [router] } }, locale), router }
}

describe('AuthoringHomePage', () => {
  beforeEach(() => {
    authMock.state = shallowRef<AuthenticationState>({ status: 'authenticated', userId: '22222222-2222-4222-8222-222222222222', expiresAt: '2026-09-15T12:00:00Z' })
    authMock.bootstrapSession.mockReset()
    listAuthoringDraftsMock.mockReset()
    listAuthoringDraftsMock.mockResolvedValue(drafts())
  })
  afterEach(cleanup)

  it('renders authoritative Drafts in server order with useful semantic links', async () => {
    await renderPage()
    const list = await screen.findByRole('list')
    expect(list.textContent).toContain('Integration foundations')
    expect(list.textContent).toContain('Messaging patterns')
    expect(list.textContent?.indexOf('Integration foundations')).toBeLessThan(list.textContent?.indexOf('Messaging patterns') ?? 0)
    expect(screen.getByText('Active Draft')).toBeTruthy()
    expect(screen.getByText('Abandoned Draft')).toBeTruthy()
    expect(screen.getAllByText('Last updated')).toHaveLength(2)
    expect(screen.getByRole('link', { name: 'Open Draft: Integration foundations' }).getAttribute('href')).toBe(`/authoring/drafts/${draftID}/overview`)
    expect(listAuthoringDraftsMock).toHaveBeenCalledTimes(1)
  })

  it('uses an informative empty state with the same real creation path', async () => {
    listAuthoringDraftsMock.mockResolvedValueOnce({ drafts: [] })
    const { router } = await renderPage()
    expect(await screen.findByRole('heading', { level: 3, name: 'No course Drafts yet' })).toBeTruthy()
    expect(screen.queryByRole('link', { name: /Open Draft/ })).toBeNull()
    await fireEvent.click(screen.getByRole('button', { name: 'Create Draft' }))
    await waitFor(() => expect(router.currentRoute.value.path).toBe('/authoring/new'))
  })

  it('offers the creation route alongside existing Draft discovery', async () => {
    const { router } = await renderPage()
    await screen.findByRole('link', { name: 'Open Draft: Integration foundations' })
    await fireEvent.click(screen.getByRole('button', { name: 'Create Draft' }))
    await waitFor(() => expect(router.currentRoute.value.path).toBe('/authoring/new'))
  })

  it('keeps an accessible loading state until Draft discovery resolves', async () => {
    let resolveDrafts: (value: ReturnType<typeof drafts>) => void = () => undefined
    listAuthoringDraftsMock.mockImplementationOnce(() => new Promise<ReturnType<typeof drafts>>((resolve) => { resolveDrafts = resolve }))
    await renderPage()
    expect(screen.getByRole('status').textContent).toContain('Loading your Drafts')
    resolveDrafts(drafts())
    expect(await screen.findByRole('link', { name: 'Open Draft: Integration foundations' })).toBeTruthy()
  })

  it('keeps operational failures distinct from an empty list and retries explicitly', async () => {
    listAuthoringDraftsMock.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(drafts())
    await renderPage()
    await screen.findByRole('alert')
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('link', { name: 'Open Draft: Integration foundations' })).toBeTruthy()
  })

  it('treats a malformed private Draft list as an operational failure', async () => {
    listAuthoringDraftsMock.mockRejectedValueOnce(new Error('Invalid Authoring draft list response'))
    await renderPage()
    expect(await screen.findByRole('heading', { level: 1, name: 'Authoring unavailable' })).toBeTruthy()
    expect(document.body.textContent).not.toContain('Invalid Authoring draft list response')
  })

  it('redirects unauthenticated visitors without loading Drafts and drops a late prior-session result', async () => {
    authMock.state.value = { status: 'unauthenticated' }
    const { router } = await renderPage()
    await waitFor(() => expect(router.currentRoute.value.path).toBe('/login'))
    expect(listAuthoringDraftsMock).not.toHaveBeenCalled()

    cleanup()
    authMock.state = shallowRef<AuthenticationState>({ status: 'authenticated', userId: '22222222-2222-4222-8222-222222222222', expiresAt: '2026-09-15T12:00:00Z' })
    let resolveFirst: (value: ReturnType<typeof drafts>) => void = () => undefined
    listAuthoringDraftsMock.mockImplementationOnce(() => new Promise<ReturnType<typeof drafts>>((resolve) => { resolveFirst = resolve }))
    await renderPage()
    authMock.state.value = { status: 'unauthenticated' }
    resolveFirst(drafts())
    await waitFor(() => expect(screen.queryByText('Integration foundations')).toBeNull())
  })

  it('localizes Academy copy without changing authored Draft titles', async () => {
    await renderPage('en-XA')
    expect(await screen.findByRole('heading', { level: 1, name: pseudoLocalize('Authoring') })).toBeTruthy()
    expect(screen.getByText('Integration foundations')).toBeTruthy()
    expect(screen.getByRole('button', { name: pseudoLocalize('Create Draft') })).toBeTruthy()
  })
})
