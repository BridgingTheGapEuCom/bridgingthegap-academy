import { cleanup, render, screen } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createI18n } from 'vue-i18n'
import { shallowRef } from 'vue'

type AuthenticationState =
  | { status: 'bootstrapping' }
  | { status: 'unauthenticated' }
  | { status: 'authenticated'; userId: string; expiresAt: string }
  | { status: 'unavailable' }

const authMock = vi.hoisted(() => ({ state: { value: { status: 'unauthenticated' } as AuthenticationState }, logout: vi.fn() }))
vi.mock('./auth/auth', () => ({ useAuth: () => authMock }))

import App from './App.vue'

async function renderShell(path = '/') {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/', component: { template: '<h1>Home page</h1>' } },
    { path: '/dashboard', component: { template: '<h1>Dashboard page</h1>' } },
    { path: '/courses', component: { template: '<h1>Courses page</h1>' } },
    { path: '/authoring', component: { template: '<h1>Authoring page</h1>' } },
    { path: '/login', component: { template: '<h1>Sign in</h1>' } },
  ] })
  await router.push(path)
  await router.isReady()
  const i18n = createI18n({ legacy: false, locale: 'en', messages: { en: { appName: 'Bridging the Gap Academy', appContext: 'Structured learning', home: 'Home', dashboard: 'Dashboard', courses: 'Courses', authoring: 'Authoring' } } })
  return render(App, { global: { plugins: [router, i18n] } })
}

describe('application header', () => {
  beforeEach(() => {
    authMock.state = shallowRef<AuthenticationState>({ status: 'unauthenticated' })
    authMock.logout.mockReset()
  })
  afterEach(cleanup)

  it('renders the Academy brand and marks only the current navigation link as current', async () => {
    await renderShell('/')
    expect(screen.getByRole('link', { name: /Bridging the Gap Academy/i })).toBeTruthy()
    expect(screen.getByText('Structured learning')).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Home' }).getAttribute('aria-current')).toBe('page')
    expect(screen.getByRole('link', { name: 'Courses' }).getAttribute('aria-current')).toBeNull()
  })

  it('keeps authenticated navigation and a single sign-out action without redundant status text', async () => {
    authMock.state = shallowRef<AuthenticationState>({ status: 'authenticated', userId: '33333333-3333-4333-8333-333333333333', expiresAt: '2027-01-01T00:00:00Z' })
    await renderShell('/dashboard')
    expect(screen.getByRole('link', { name: 'Dashboard' }).getAttribute('aria-current')).toBe('page')
    expect(screen.getByRole('link', { name: 'Authoring' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeTruthy()
    expect(screen.queryByText('Signed in')).toBeNull()
  })
})
