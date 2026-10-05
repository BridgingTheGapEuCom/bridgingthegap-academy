import { cleanup, fireEvent, screen } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { shallowRef } from 'vue'
import type { ApplicationLocale } from './i18n/registry'
import { pseudoLocalize } from './i18n/pseudo'
import { renderWithI18n } from './test/i18n'

type AuthenticationState =
  | { status: 'bootstrapping' }
  | { status: 'unauthenticated' }
  | { status: 'authenticated'; userId: string; expiresAt: string }
  | { status: 'unavailable' }

const authMock = vi.hoisted(() => ({ state: { value: { status: 'unauthenticated' } as AuthenticationState }, logout: vi.fn() }))
vi.mock('./auth/auth', () => ({ useAuth: () => authMock }))

import App from './App.vue'

async function renderShell(path = '/', locale: ApplicationLocale = 'en') {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/', component: { template: '<h1>Home page</h1>' } },
    { path: '/dashboard', component: { template: '<h1>Dashboard page</h1>' } },
    { path: '/courses', component: { template: '<h1>Courses page</h1>' } },
    { path: '/authoring', component: { template: '<h1>Authoring page</h1>' } },
    { path: '/login', component: { template: '<h1>Sign in</h1>' } },
  ] })
  await router.push(path)
  await router.isReady()
  return { ...renderWithI18n(App, { global: { plugins: [router] } }, locale), router }
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

  it('keeps the shared shell mounted while RouterLink replaces routed page content', async () => {
    const { router } = await renderShell('/')
    const header = document.querySelector('.site-header')
    await fireEvent.click(screen.getByRole('link', { name: 'Courses' }))
    await screen.findByRole('heading', { name: 'Courses page' })
    expect(router.currentRoute.value.path).toBe('/courses')
    expect(document.querySelector('.site-header')).toBe(header)
    expect(screen.queryByRole('heading', { name: 'Home page' })).toBeNull()
  })

  it('renders the migrated shell from the pseudo-locale without hardcoded English navigation', async () => {
    await renderShell('/', 'en-XA')
    expect(screen.getByRole('link', { name: pseudoLocalize('Home') })).toBeTruthy()
    expect(screen.queryByRole('link', { name: 'Home' })).toBeNull()
    expect(screen.getByRole('navigation', { name: pseudoLocalize('Main navigation') })).toBeTruthy()
  })
})
