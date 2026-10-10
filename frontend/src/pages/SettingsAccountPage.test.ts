import { cleanup, screen } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { pseudoLocalize } from '../i18n/pseudo'
import type { ApplicationLocale } from '../i18n/registry'
import { renderWithI18n } from '../test/i18n'

const authMock = vi.hoisted(() => ({
  state: { value: { status: 'authenticated' as const, userId: '22222222-2222-4222-8222-222222222222', expiresAt: '2026-10-10T12:00:00Z' } },
}))

vi.mock('../auth/auth', () => ({ useAuth: () => authMock }))

import SettingsAccountPage from './SettingsAccountPage.vue'

async function renderPage(locale: ApplicationLocale = 'en') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/settings/preferences', component: SettingsAccountPage },
      { path: '/settings/account', component: SettingsAccountPage },
    ],
  })
  await router.push('/settings/account')
  await router.isReady()
  return renderWithI18n(SettingsAccountPage, { global: { plugins: [router] } }, locale)
}

describe('Settings account', () => {
  beforeEach(() => { authMock.state.value = { status: 'authenticated', userId: '22222222-2222-4222-8222-222222222222', expiresAt: '2026-10-10T12:00:00Z' } })
  afterEach(cleanup)

  it('shows only the supported current-session expiry and both settings destinations', async () => {
    await renderPage()
    expect(screen.getByRole('heading', { level: 1, name: 'Account' })).toBeTruthy()
    expect(screen.getByRole('heading', { level: 2, name: 'Current session' })).toBeTruthy()
    expect(screen.getByText('Session expires')).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Preferences' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Account' }).getAttribute('aria-current')).toBe('page')
    expect(screen.queryByText('Profile')).toBeNull()
    expect(screen.queryByText('Authentication')).toBeNull()
    expect(screen.queryByRole('button', { name: /save|change password/i })).toBeNull()
  })

  it('localizes the account page in the pseudo-locale', async () => {
    await renderPage('en-XA')
    expect(screen.getByRole('heading', { level: 1, name: pseudoLocalize('Account') })).toBeTruthy()
  })
})
