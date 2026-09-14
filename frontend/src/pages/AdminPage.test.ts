import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { shallowRef } from 'vue'

type AuthenticationState =
  | { status: 'bootstrapping' }
  | { status: 'unauthenticated' }
  | { status: 'authenticated'; userId: string; expiresAt: string }
  | { status: 'unavailable' }

const authMock = vi.hoisted(() => ({
  state: { value: { status: 'authenticated' } as AuthenticationState },
  request: vi.fn(),
  bootstrapSession: vi.fn(),
  logout: vi.fn(),
}))
const checkAdminAccessMock = vi.hoisted(() => vi.fn())

vi.mock('../auth/auth', () => ({ useAuth: () => authMock }))
vi.mock('../admin/adminAccess', () => ({ checkAdminAccess: checkAdminAccessMock }))

import AdminPage from './AdminPage.vue'

const authenticatedState = { status: 'authenticated' as const, userId: '0c640c8d-50e2-4098-8b2a-8094c544c9d7', expiresAt: '2026-09-15T12:00:00Z' }

async function renderAdmin(state: AuthenticationState = authenticatedState) {
  authMock.state.value = state
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<main>Home</main>' } },
      { path: '/login', component: { template: '<main>Login</main>' } },
      { path: '/admin', component: AdminPage },
    ],
  })
  await router.push('/admin')
  await router.isReady()
  return { ...render(AdminPage, { global: { plugins: [router] } }), router }
}

describe('AdminPage', () => {
  beforeEach(() => {
    authMock.state = shallowRef<AuthenticationState>(authenticatedState)
    authMock.request.mockReset()
    authMock.bootstrapSession.mockReset()
    authMock.logout.mockReset()
    checkAdminAccessMock.mockReset()
    checkAdminAccessMock.mockResolvedValue({ kind: 'authorized', status: 'ok' })
  })

  afterEach(cleanup)

  it('does not render Administration before the backend capability check succeeds', async () => {
    let resolveAccess: (outcome: { kind: 'authorized'; status: 'ok' }) => void = () => undefined
    checkAdminAccessMock.mockImplementation(() => new Promise((resolve) => { resolveAccess = resolve }))
    await renderAdmin()

    expect(screen.getByRole('heading', { level: 1, name: 'Checking access…' })).toBeTruthy()
    expect(screen.queryByText('System status: OK')).toBeNull()
    resolveAccess({ kind: 'authorized', status: 'ok' })
    await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: 'Administration' })).toBeTruthy())
  })

  it('uses the backend result for authorized and forbidden states without showing role internals', async () => {
    await renderAdmin()
    await waitFor(() => expect(screen.getByText('System status: OK')).toBeTruthy())
    expect(screen.getByRole('status').textContent).toBe('Administration access confirmed.')
    expect(checkAdminAccessMock).toHaveBeenCalledWith(authMock)
    expect(document.body.textContent).not.toContain('ADMINISTRATOR')
    expect(document.body.textContent).not.toContain('instance.manage')

    cleanup()
    checkAdminAccessMock.mockResolvedValue({ kind: 'forbidden' })
    await renderAdmin()
    await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: 'Access denied' })).toBeTruthy())
    expect(screen.getByRole('status').textContent).toBe('Access denied.')
    expect(authMock.state.value).toEqual(authenticatedState)
    expect(screen.queryByText('System status: OK')).toBeNull()
  })

  it('sends unauthenticated access checks to login instead of rendering denial', async () => {
    checkAdminAccessMock.mockResolvedValue({ kind: 'unauthenticated' })
    const { router } = await renderAdmin()

    await waitFor(() => expect(router.currentRoute.value.path).toBe('/login'))
    expect(screen.queryByRole('heading', { level: 1, name: 'Access denied' })).toBeNull()
  })

  it('redirects an unauthenticated route visit to login', async () => {
    const { router } = await renderAdmin({ status: 'unauthenticated' })

    await waitFor(() => expect(router.currentRoute.value.path).toBe('/login'))
    expect(checkAdminAccessMock).not.toHaveBeenCalled()
  })

  it('shows unavailable access separately and retries the backend check', async () => {
    checkAdminAccessMock.mockResolvedValue({ kind: 'unavailable' })
    await renderAdmin()
    await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: 'Administration unavailable' })).toBeTruthy())

    checkAdminAccessMock.mockResolvedValue({ kind: 'authorized', status: 'ok' })
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: 'Administration' })).toBeTruthy())
    expect(checkAdminAccessMock).toHaveBeenCalledTimes(2)
  })

  it('renders bootstrap and session-unavailable states without admin content', async () => {
    await renderAdmin({ status: 'bootstrapping' })
    expect(screen.getByRole('heading', { level: 1, name: 'Checking your session' })).toBeTruthy()
    expect(checkAdminAccessMock).not.toHaveBeenCalled()

    cleanup()
    authMock.bootstrapSession.mockResolvedValue({ status: 'unauthenticated' })
    await renderAdmin({ status: 'unavailable' })
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(authMock.bootstrapSession).toHaveBeenCalledOnce()
  })

  it('signs out through the existing auth service and prevents duplicate submissions', async () => {
    let completeLogout: (outcome: { kind: 'unauthenticated' }) => void = () => undefined
    authMock.logout.mockImplementation(() => new Promise((resolve) => { completeLogout = resolve }))
    await renderAdmin()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Sign out' })).toBeTruthy())

    await fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Signing out…' }))
    expect(authMock.logout).toHaveBeenCalledOnce()
    completeLogout({ kind: 'unauthenticated' })
  })

  it('keeps the authenticated page and announces operational logout failure', async () => {
    authMock.logout.mockResolvedValue({ kind: 'unavailable' })
    await renderAdmin()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Sign out' })).toBeTruthy())

    await fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))
    const error = await screen.findByText('We couldn’t sign you out right now. Please try again.')
    expect(error.textContent).toBe('We couldn’t sign you out right now. Please try again.')
    expect(document.activeElement).toBe(error)
    expect(authMock.state.value).toEqual(authenticatedState)
    expect(screen.getByRole('heading', { level: 1, name: 'Administration' })).toBeTruthy()
  })

  it('hides authorized content when a session becomes unauthenticated and ignores an old access result', async () => {
    let completeAccess: (value: { kind: 'authorized'; status: 'ok' }) => void = () => undefined
    checkAdminAccessMock.mockImplementation(() => new Promise((resolve) => { completeAccess = resolve }))
    const { router } = await renderAdmin()

    authMock.state.value = { status: 'unauthenticated' }
    completeAccess({ kind: 'authorized', status: 'ok' })
    await waitFor(() => expect(router.currentRoute.value.path).toBe('/login'))
    expect(screen.queryByText('System status: OK')).toBeNull()
  })

  it('rechecks access when a different authenticated session replaces the current one', async () => {
    let completeOldAccess: (value: { kind: 'forbidden' }) => void = () => undefined
    checkAdminAccessMock.mockImplementationOnce(() => new Promise((resolve) => { completeOldAccess = resolve }))
    checkAdminAccessMock.mockResolvedValue({ kind: 'authorized', status: 'ok' })
    await renderAdmin()

    authMock.state.value = { status: 'authenticated', userId: '9c3dc5dc-ac79-4bdd-8e1a-89a795186ddb', expiresAt: authenticatedState.expiresAt }
    await waitFor(() => expect(checkAdminAccessMock).toHaveBeenCalledTimes(2))
    completeOldAccess({ kind: 'forbidden' })
    await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: 'Administration' })).toBeTruthy())
    expect(screen.queryByRole('heading', { level: 1, name: 'Access denied' })).toBeNull()
  })

  it('handles an unexpected logout rejection without clearing the authenticated page', async () => {
    authMock.logout.mockRejectedValue(new Error('request failed'))
    await renderAdmin()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Sign out' })).toBeTruthy())

    await fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))
    expect(await screen.findByText('We couldn’t sign you out right now. Please try again.')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Sign out' }).hasAttribute('disabled')).toBe(false)
    expect(authMock.state.value).toEqual(authenticatedState)
  })

  it('rechecks backend authorization on a later page visit instead of storing an isAdmin flag', async () => {
    await renderAdmin()
    await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: 'Administration' })).toBeTruthy())

    cleanup()
    checkAdminAccessMock.mockResolvedValue({ kind: 'forbidden' })
    await renderAdmin()
    await waitFor(() => expect(screen.getByRole('heading', { level: 1, name: 'Access denied' })).toBeTruthy())
    expect(checkAdminAccessMock).toHaveBeenCalledTimes(2)
  })
})
