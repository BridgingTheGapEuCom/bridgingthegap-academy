import { afterEach, describe, expect, it, vi } from 'vitest'
import { ref, type Ref } from 'vue'
import type { AuthenticationState } from '../auth/auth'
import { createApplicationI18n } from '../i18n/application'
import { applicationLocaleStorageKey } from '../i18n/preference'
import type { ColorSchemeMediaQuery } from './appearance'
import { createUserPreferencesStore, type PreferencesAuth, type UserPreferencesResponse } from './preferences'

class MockColorScheme implements ColorSchemeMediaQuery {
  matches: boolean
  private readonly listeners = new Set<(event: MediaQueryListEvent) => void>()

  constructor(matches: boolean) { this.matches = matches }
  addEventListener(_type: 'change', listener: (event: MediaQueryListEvent) => void) { this.listeners.add(listener) }
  removeEventListener(_type: 'change', listener: (event: MediaQueryListEvent) => void) { this.listeners.delete(listener) }
  change(matches: boolean) {
    this.matches = matches
    for (const listener of this.listeners) listener({ matches } as MediaQueryListEvent)
  }
}

function authenticatedAuth(response: UserPreferencesResponse | Error) {
  const state = ref<AuthenticationState>({ status: 'authenticated', userId: 'user-a', expiresAt: '2027-01-01T00:00:00Z' })
  const auth: PreferencesAuth = {
    state,
    async bootstrapSession() { return state.value },
    async request<T>() {
      if (response instanceof Error) throw response
      return response as T
    },
  }
  return { auth, state }
}

afterEach(() => {
  window.localStorage.clear()
  document.documentElement.lang = 'en'
  document.documentElement.dir = ''
  document.documentElement.removeAttribute('data-theme')
})

describe('user preferences bootstrap', () => {
  it('uses an explicit persisted locale ahead of local preference and synchronizes the document', async () => {
    window.localStorage.setItem(applicationLocaleStorageKey, 'en-XA')
    const { auth } = authenticatedAuth({ applicationLocale: 'en', appearance: 'DARK', revision: 3, updatedAt: '2026-10-10T00:00:00Z' })
    const store = createUserPreferencesStore({ auth, i18n: await createApplicationI18n(), mediaQuery: new MockColorScheme(false) })
    await store.bootstrap()

    expect(store.state.value.locale).toBe('en')
    expect(store.state.value.revision).toBe(3)
    expect(store.state.value.appearance).toBe('dark')
    expect(document.documentElement.lang).toBe('en')
    expect(document.documentElement.dataset.theme).toBe('dark')
    store.dispose()
  })

  it('uses the existing local/browser fallback when the persisted locale is null', async () => {
    window.localStorage.setItem(applicationLocaleStorageKey, 'en-XA')
    const { auth } = authenticatedAuth({ applicationLocale: null, appearance: 'LIGHT', revision: 1, updatedAt: null })
    const store = createUserPreferencesStore({ auth, i18n: await createApplicationI18n(), mediaQuery: new MockColorScheme(true) })
    await store.bootstrap()

    expect(store.state.value.locale).toBe('en-XA')
    expect(store.state.value.appearance).toBe('light')
    expect(document.documentElement.lang).toBe('en-XA')
    store.dispose()
  })

  it('safely falls back from an unsupported persisted locale without discarding the valid appearance', async () => {
    const { auth } = authenticatedAuth({ applicationLocale: 'es', appearance: 'DARK', revision: 1, updatedAt: null })
    const store = createUserPreferencesStore({ auth, i18n: await createApplicationI18n('en'), mediaQuery: new MockColorScheme(false) })
    await store.bootstrap()

    expect(store.state.value.locale).toBe('en')
    expect(store.state.value.appearance).toBe('dark')
    expect(store.state.value.error).toBe('invalid-preference')
    store.dispose()
  })

  it('keeps bootstrap usable after a preference request failure', async () => {
    const { auth } = authenticatedAuth(new Error('offline'))
    const store = createUserPreferencesStore({ auth, i18n: await createApplicationI18n('en'), mediaQuery: new MockColorScheme(true) })
    await store.bootstrap()

    expect(store.state.value.loading).toBe(false)
    expect(store.state.value.error).toBe('unavailable')
    expect(store.state.value.locale).toBe('en')
    expect(store.state.value.appearance).toBe('dark')
    store.dispose()
  })

  it('does not request preferences for anonymous visitors', async () => {
    const state: Ref<AuthenticationState> = ref({ status: 'unauthenticated' })
    const request = vi.fn()
    const auth: PreferencesAuth = {
      state,
      async bootstrapSession() { return state.value },
      async request<T>() { return request() as T },
    }
    const store = createUserPreferencesStore({ auth, i18n: await createApplicationI18n('en'), mediaQuery: new MockColorScheme(false) })
    await store.bootstrap()

    expect(request).not.toHaveBeenCalled()
    expect(store.state.value.appearance).toBe('light')
    store.dispose()
  })

  it('follows browser color changes only while SYSTEM is stored', async () => {
    const media = new MockColorScheme(false)
    const { auth } = authenticatedAuth({ applicationLocale: null, appearance: 'SYSTEM', revision: 1, updatedAt: null })
    const store = createUserPreferencesStore({ auth, i18n: await createApplicationI18n('en'), mediaQuery: media })
    await store.bootstrap()
    media.change(true)

    expect(store.state.value.appearance).toBe('dark')
    expect(document.documentElement.dataset.theme).toBe('dark')
    store.dispose()
    media.change(false)
    expect(document.documentElement.dataset.theme).toBe('dark')
  })
})
