import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { shallowRef } from 'vue'

type AuthenticationState =
  | { status: 'bootstrapping' }
  | { status: 'unauthenticated' }
  | { status: 'authenticated'; userId: string; expiresAt: string }
  | { status: 'unavailable' }

const authMock = vi.hoisted(() => ({ state: { value: { status: 'unauthenticated' } as AuthenticationState }, logout: vi.fn() }))
vi.mock('../auth/auth', () => ({ useAuth: () => authMock }))

import AppSessionControls from './AppSessionControls.vue'

function routerFor() {
  return createRouter({ history: createMemoryHistory(), routes: [
    { path: '/', component: { template: '<p>Home</p>' } },
    { path: '/login', component: { template: '<p>Login</p>' } },
  ] })
}

async function renderControls() {
  const router = routerFor()
  await router.push('/')
  await router.isReady()
  return { ...render(AppSessionControls, { global: { plugins: [router] } }), router }
}

describe('AppSessionControls', () => {
  beforeEach(() => {
    authMock.state = shallowRef<AuthenticationState>({ status: 'unauthenticated' })
    authMock.logout.mockReset()
  })
  afterEach(cleanup)

  it('does not flash sign-in or protected session controls while resolving', async () => {
    authMock.state.value = { status: 'bootstrapping' }
    await renderControls()
    expect(screen.getByRole('status').textContent).toContain('Checking session')
    expect(screen.queryByRole('link', { name: 'Sign in' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Sign out' })).toBeNull()
  })

  it('makes unauthenticated and authenticated states visible without exposing the opaque user ID', async () => {
    await renderControls()
    expect(screen.getByRole('link', { name: 'Sign in' }).getAttribute('href')).toBe('/login')

    cleanup()
    authMock.state = shallowRef<AuthenticationState>({ status: 'authenticated', userId: '33333333-3333-4333-8333-333333333333', expiresAt: '2026-09-17T12:00:00Z' })
    await renderControls()
    expect(screen.getByLabelText('Authentication status').textContent).toBe('Signed in')
    expect(document.body.textContent).not.toContain('33333333-3333-4333-8333-333333333333')
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeTruthy()
  })

  it('signs out through the existing service and keeps a retryable sanitised failure', async () => {
    authMock.state = shallowRef<AuthenticationState>({ status: 'authenticated', userId: '33333333-3333-4333-8333-333333333333', expiresAt: '2026-09-17T12:00:00Z' })
    authMock.logout.mockResolvedValueOnce({ kind: 'unavailable' }).mockResolvedValueOnce({ kind: 'unauthenticated' })
    const { router } = await renderControls()
    await fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))
    const error = await screen.findByRole('alert')
    expect(error.textContent).toContain('We couldn’t sign you out')
    expect(document.activeElement).toBe(error)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Sign out' })).toBeTruthy())
    await fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))
    await waitFor(() => expect(router.currentRoute.value.path).toBe('/login'))
  })
})
