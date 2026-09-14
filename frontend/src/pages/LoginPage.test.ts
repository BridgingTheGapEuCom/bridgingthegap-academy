import { cleanup, fireEvent, render, screen } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

type AuthenticationState =
  | { status: 'bootstrapping' }
  | { status: 'unauthenticated' }
  | { status: 'authenticated'; userId: string; expiresAt: string }
  | { status: 'unavailable' }

const authMock = vi.hoisted(() => ({
  state: { value: { status: 'unauthenticated' } as AuthenticationState },
  login: vi.fn(),
  bootstrapSession: vi.fn(),
}))

vi.mock('../auth/auth', () => ({ useAuth: () => authMock }))

import LoginPage from './LoginPage.vue'

const authenticatedState = { status: 'authenticated' as const, userId: '0c640c8d-50e2-4098-8b2a-8094c544c9d7', expiresAt: '2026-09-15T12:00:00Z' }

async function renderLogin(state: AuthenticationState = { status: 'unauthenticated' }) {
  authMock.state.value = state
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<main>Home</main>' } },
      { path: '/login', component: LoginPage },
    ],
  })
  await router.push('/login')
  await router.isReady()
  return { ...render(LoginPage, { global: { plugins: [router] } }), router }
}

async function submitCredentials(email = 'learner@example.com', password = 'correct horse battery staple') {
  await fireEvent.update(emailInput(), email)
  await fireEvent.update(passwordInput(), password)
  await fireEvent.submit(screen.getByRole('form', { name: 'Sign in' }))
}

function emailInput() { return screen.getByLabelText(/^Email/) }
function passwordInput() { return screen.getByLabelText(/^Password/) }

describe('LoginPage', () => {
  beforeEach(() => {
    authMock.state.value = { status: 'unauthenticated' }
    authMock.login.mockReset()
    authMock.bootstrapSession.mockReset()
  })

  afterEach(cleanup)

  it('renders a labelled native sign-in form', async () => {
    await renderLogin()
    const email = screen.getByRole('textbox', { name: 'Email required' })
    const password = passwordInput()

    expect(screen.getByRole('heading', { level: 1, name: 'Sign in' })).toBeTruthy()
    expect(email.getAttribute('type')).toBe('email')
    expect(email.getAttribute('autocomplete')).toBe('username')
    expect(password.getAttribute('type')).toBe('password')
    expect(password.getAttribute('autocomplete')).toBe('current-password')
  })

  it('shows accessible required errors and focuses the first invalid field', async () => {
    await renderLogin()
    await fireEvent.submit(screen.getByRole('form', { name: 'Sign in' }))

    expect(screen.getByText('Enter your email address.')).toBeTruthy()
    expect(screen.getByText('Enter your password.')).toBeTruthy()
    expect(screen.queryByRole('alert')).toBeNull()
    expect(document.activeElement).toBe(emailInput())
  })

  it('delegates valid submission to the existing auth service and prevents duplicate pending requests', async () => {
    let completeLogin: (value: { kind: 'invalid-credentials' }) => void = () => undefined
    authMock.login.mockImplementation(() => new Promise((resolve) => { completeLogin = resolve }))
    await renderLogin()

    await submitCredentials()
    await fireEvent.submit(screen.getByRole('form', { name: 'Sign in' }))

    expect(authMock.login).toHaveBeenCalledOnce()
    expect(authMock.login).toHaveBeenCalledWith('learner@example.com', 'correct horse battery staple')
    expect(screen.getByRole('button', { name: 'Signing in…' }).getAttribute('disabled')).not.toBeNull()
    completeLogin({ kind: 'invalid-credentials' })
  })

  it('keeps the email, clears the password, and focuses a generic error after invalid credentials', async () => {
    authMock.login.mockResolvedValue({ kind: 'invalid-credentials' })
    await renderLogin()

    await submitCredentials('learner@example.com', 'private password')

    const error = screen.getByText('The email or password is incorrect.')
    expect(error.textContent).toBe('The email or password is incorrect.')
    expect(document.activeElement).toBe(error)
    expect((emailInput() as HTMLInputElement).value).toBe('learner@example.com')
    expect((passwordInput() as HTMLInputElement).value).toBe('')
    expect(error.textContent).not.toContain('private password')
  })

  it('distinguishes rate-limited and unavailable login outcomes without exposing credential details', async () => {
    authMock.login.mockResolvedValueOnce({ kind: 'rate-limited', retryAfterSeconds: 20 })
    await renderLogin()
    await submitCredentials()
    expect(screen.getByText('Too many sign-in attempts. Please try again in about 20 seconds.')).toBeTruthy()

    cleanup()
    authMock.login.mockResolvedValueOnce({ kind: 'unavailable' })
    await renderLogin()
    await submitCredentials()
    expect(screen.getByText('We couldn’t sign you in right now. Please try again.')).toBeTruthy()
  })

  it('clears the password and releases submission state after an unexpected service rejection', async () => {
    authMock.login.mockRejectedValue(new Error('request failed'))
    await renderLogin()
    await submitCredentials('learner@example.com', 'private password')

    expect(screen.getByText('We couldn’t sign you in right now. Please try again.')).toBeTruthy()
    expect((passwordInput() as HTMLInputElement).value).toBe('')
    expect((emailInput() as HTMLInputElement).value).toBe('learner@example.com')
    expect(screen.getByRole('button', { name: 'Sign in' }).hasAttribute('disabled')).toBe(false)
  })

  it('does not navigate after a pending login resolves on an unmounted page', async () => {
    let completeLogin: (value: { kind: 'authenticated'; userId: string; expiresAt: string }) => void = () => undefined
    authMock.login.mockImplementation(() => new Promise((resolve) => { completeLogin = resolve }))
    const { router, unmount } = await renderLogin()
    const push = vi.spyOn(router, 'push')
    await submitCredentials('learner@example.com', 'private password')

    unmount()
    completeLogin({ kind: 'authenticated', userId: authenticatedState.userId, expiresAt: authenticatedState.expiresAt })
    await Promise.resolve()
    await Promise.resolve()
    expect(push).not.toHaveBeenCalledWith('/')
  })

  it('clears the password and navigates only to home after successful login', async () => {
    authMock.login.mockResolvedValue({ kind: 'authenticated', userId: authenticatedState.userId, expiresAt: authenticatedState.expiresAt })
    const { router } = await renderLogin()
    const push = vi.spyOn(router, 'push')

    await submitCredentials()

    expect((passwordInput() as HTMLInputElement).value).toBe('')
    expect(push).toHaveBeenCalledWith('/')
  })

  it('does not render the form while bootstrapping or already authenticated', async () => {
    await renderLogin({ status: 'bootstrapping' })
    expect(screen.getByRole('heading', { level: 1, name: 'Checking your session' })).toBeTruthy()
    expect(screen.queryByRole('form')).toBeNull()

    cleanup()
    await renderLogin(authenticatedState)
    expect(screen.getByRole('heading', { level: 1, name: 'You are already signed in.' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Continue to home' })).toBeTruthy()
    expect(screen.queryByText(/administrator/i)).toBeNull()
  })

  it('provides a retry action while bootstrap is unavailable', async () => {
    authMock.bootstrapSession.mockResolvedValue({ status: 'unauthenticated' })
    await renderLogin({ status: 'unavailable' })

    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))

    expect(authMock.bootstrapSession).toHaveBeenCalledOnce()
  })
})
