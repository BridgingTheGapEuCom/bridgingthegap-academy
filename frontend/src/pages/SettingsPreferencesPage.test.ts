import { cleanup, fireEvent, screen } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { APIProblemError } from '../api/client'
import type { ApplicationLocale } from '../i18n/registry'
import { pseudoLocalize } from '../i18n/pseudo'
import { renderWithI18n } from '../test/i18n'

const authMock = vi.hoisted(() => ({ request: vi.fn() }))
const preferenceMock = vi.hoisted(() => ({
  state: { value: {
    loading: false,
    preferences: { applicationLocale: null, appearance: 'SYSTEM', revision: 0, updatedAt: null },
    locale: 'en', storedAppearance: 'SYSTEM', appearance: 'light', revision: 0,
  } },
  refresh: vi.fn(),
  applySaved: vi.fn(),
}))

vi.mock('../auth/auth', () => ({ useAuth: () => authMock }))
vi.mock('../preferences/preferences', () => ({ useUserPreferences: () => preferenceMock }))

import SettingsPreferencesPage from './SettingsPreferencesPage.vue'

async function renderPage(locale: ApplicationLocale = 'en') {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/settings/preferences', component: SettingsPreferencesPage }, { path: '/settings/account', component: SettingsPreferencesPage }] })
  await router.push('/settings/preferences')
  await router.isReady()
  return renderWithI18n(SettingsPreferencesPage, { global: { plugins: [router] } }, locale)
}

describe('Settings preferences', () => {
  beforeEach(() => {
    preferenceMock.state.value = {
      loading: false,
      preferences: { applicationLocale: null, appearance: 'SYSTEM', revision: 0, updatedAt: null },
      locale: 'en', storedAppearance: 'SYSTEM', appearance: 'light', revision: 0,
    }
    preferenceMock.refresh.mockReset()
    preferenceMock.applySaved.mockReset()
    authMock.request.mockReset()
  })
  afterEach(cleanup)

  it('represents the null locale as the automatic option and excludes pseudo locales', async () => {
    await renderPage()
    const select = screen.getByRole('combobox') as HTMLSelectElement
    expect(select.value).toBe('')
    expect(screen.getByRole('option', { name: 'Use browser/default language' })).toBeTruthy()
    expect(screen.queryByRole('option', { name: 'Pseudo-English' })).toBeNull()
    expect((screen.getByRole('radio', { name: 'System' }) as HTMLInputElement).checked).toBe(true)
  })

  it('localizes the Settings shell in the pseudo-locale', async () => {
    await renderPage('en-XA')
    expect(screen.getByRole('heading', { level: 1, name: pseudoLocalize('Preferences') })).toBeTruthy()
    expect(screen.queryByRole('heading', { level: 1, name: 'Preferences' })).toBeNull()
  })

  it('keeps changes local until Save and restores them with Discard', async () => {
    await renderPage()
    await fireEvent.update(screen.getByRole('combobox'), 'en')
    await fireEvent.click(screen.getByRole('radio', { name: 'Dark' }))
    expect(screen.getByLabelText('Unsaved changes')).toBeTruthy()
    expect(authMock.request).not.toHaveBeenCalled()
    await fireEvent.click(screen.getByRole('button', { name: 'Discard' }))

    expect((screen.getByRole('combobox') as HTMLSelectElement).value).toBe('')
    expect((screen.getByRole('radio', { name: 'System' }) as HTMLInputElement).checked).toBe(true)
  })

  it('saves the complete preference replacement with its revision and applies the accepted response', async () => {
    preferenceMock.applySaved.mockImplementation(async (value: unknown) => {
      preferenceMock.state.value = {
        ...preferenceMock.state.value,
        preferences: value as typeof preferenceMock.state.value.preferences,
        revision: 1,
      }
      return true
    })
    authMock.request.mockResolvedValue({ applicationLocale: 'en', appearance: 'DARK', revision: 1, updatedAt: '2026-10-10T00:00:00Z' })
    await renderPage()
    await fireEvent.update(screen.getByRole('combobox'), 'en')
    await fireEvent.click(screen.getByRole('radio', { name: 'Dark' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(JSON.parse(authMock.request.mock.calls[0][1].body)).toEqual({ applicationLocale: 'en', appearance: 'DARK', revision: 0 })
    expect(preferenceMock.applySaved).toHaveBeenCalled()
    expect(await screen.findByText('Preferences saved.')).toBeTruthy()
    expect(screen.queryByLabelText('Unsaved changes')).toBeNull()
  })

  it('preserves the draft and offers reload after a revision conflict', async () => {
    authMock.request.mockRejectedValue(new APIProblemError(409, undefined, undefined))
    await renderPage()
    await fireEvent.click(screen.getByRole('radio', { name: 'Light' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect((await screen.findByRole('alert')).textContent).toContain('changed elsewhere')
    expect(screen.getByRole('button', { name: 'Reload settings' })).toBeTruthy()
    expect((screen.getByRole('radio', { name: 'Light' }) as HTMLInputElement).checked).toBe(true)
  })
})
