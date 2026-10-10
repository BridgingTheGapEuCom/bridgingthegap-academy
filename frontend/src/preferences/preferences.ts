import { readonly, ref, type Ref, watch } from 'vue'
import type { components } from '../api/generated'
import type { AuthenticationState } from '../auth/auth'
import { setApplicationLocale, type AcademyI18n } from '../i18n/application'
import { resolveApplicationLocale } from '../i18n/preference'
import { isProductionApplicationLocale, type ApplicationLocale } from '../i18n/registry'
import { applyAppearance, colorSchemeMediaQuery, resolveAppearance, type ColorSchemeMediaQuery, type ResolvedAppearance, type StoredAppearance } from './appearance'

export type UserPreferencesResponse = components['schemas']['UserPreferences']

export interface PreferencesAuth {
  readonly state: Readonly<Ref<AuthenticationState>>
  bootstrapSession(): Promise<AuthenticationState>
  request<T>(path: string, options?: { invalidateOnUnauthorized?: boolean }): Promise<T>
}

export interface UserPreferencesState {
  loading: boolean
  error?: 'unavailable' | 'invalid-preference'
  preferences?: UserPreferencesResponse
  locale: ApplicationLocale
  storedAppearance: StoredAppearance
  appearance: ResolvedAppearance
  revision?: number
}

export interface UserPreferencesStore {
  readonly state: Readonly<Ref<UserPreferencesState>>
  bootstrap(): Promise<void>
  refresh(): Promise<void>
  dispose(): void
}

export interface UserPreferencesStoreOptions {
  auth: PreferencesAuth
  i18n: AcademyI18n
  mediaQuery?: ColorSchemeMediaQuery
}

function fallbackLocale(): ApplicationLocale {
  return resolveApplicationLocale()
}

function isAppearance(value: unknown): value is StoredAppearance {
  return value === 'SYSTEM' || value === 'LIGHT' || value === 'DARK'
}

function validPreferences(value: unknown): value is UserPreferencesResponse {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return false
  const candidate = value as Record<string, unknown>
  return (candidate.applicationLocale === null || typeof candidate.applicationLocale === 'string')
    && isAppearance(candidate.appearance)
    && typeof candidate.revision === 'number' && Number.isSafeInteger(candidate.revision) && candidate.revision >= 0
    && (candidate.updatedAt === null || typeof candidate.updatedAt === 'string')
}

export function createUserPreferencesStore(options: UserPreferencesStoreOptions): UserPreferencesStore {
  const mediaQuery = options.mediaQuery ?? colorSchemeMediaQuery()
  const initialLocale = fallbackLocale()
  const initialAppearance = resolveAppearance('SYSTEM', mediaQuery?.matches ?? false)
  const state = ref<UserPreferencesState>({
    loading: false,
    locale: initialLocale,
    storedAppearance: 'SYSTEM',
    appearance: initialAppearance,
  })
  applyAppearance(initialAppearance)

  let generation = 0
  let currentUserID: string | undefined
  let inFlight: Promise<void> | undefined

  const apply = async (next: Omit<UserPreferencesState, 'loading'> & { loading?: boolean }, token: number) => {
    await setApplicationLocale(options.i18n, next.locale, false)
    if (token !== generation) return
    applyAppearance(next.appearance)
    state.value = { ...next, loading: next.loading ?? false }
  }

  const applyFallback = async () => {
    const token = ++generation
    currentUserID = undefined
    const locale = fallbackLocale()
    const appearance = resolveAppearance('SYSTEM', mediaQuery?.matches ?? false)
    await apply({ locale, storedAppearance: 'SYSTEM', appearance }, token)
  }

  const load = async (userID: string) => {
    if (currentUserID === userID && inFlight) return inFlight
    const token = ++generation
    currentUserID = userID
    state.value = { ...state.value, loading: true, error: undefined }
    const task = (async () => {
      try {
        const preferences = await options.auth.request<unknown>('/api/me/preferences', { invalidateOnUnauthorized: false })
        if (token !== generation || currentUserID !== userID) return
        if (!validPreferences(preferences)) {
          await apply({ locale: fallbackLocale(), storedAppearance: 'SYSTEM', appearance: resolveAppearance('SYSTEM', mediaQuery?.matches ?? false), error: 'invalid-preference' }, token)
          return
        }
        const locale = resolveApplicationLocale({ persistedLocale: preferences.applicationLocale })
        const invalidLocale = preferences.applicationLocale !== null && !isProductionApplicationLocale(preferences.applicationLocale)
        await apply({
          locale,
          storedAppearance: preferences.appearance,
          appearance: resolveAppearance(preferences.appearance, mediaQuery?.matches ?? false),
          preferences,
          revision: preferences.revision,
          error: invalidLocale ? 'invalid-preference' : undefined,
        }, token)
      } catch {
        if (token !== generation || currentUserID !== userID) return
        await apply({ locale: fallbackLocale(), storedAppearance: 'SYSTEM', appearance: resolveAppearance('SYSTEM', mediaQuery?.matches ?? false), error: 'unavailable' }, token)
      }
    })()
    inFlight = task
    try {
      await task
    } finally {
      if (inFlight === task) inFlight = undefined
    }
  }

  const reconcile = async () => {
    const session = options.auth.state.value
    if (session.status === 'authenticated') return load(session.userId)
    await applyFallback()
  }

  const onColorSchemeChange = (event: MediaQueryListEvent) => {
    if (state.value.storedAppearance !== 'SYSTEM') return
    const appearance = resolveAppearance('SYSTEM', event.matches)
    applyAppearance(appearance)
    state.value = { ...state.value, appearance }
  }
  mediaQuery?.addEventListener?.('change', onColorSchemeChange)
  const stop = watch(() => options.auth.state.value, () => { void reconcile() })

  return {
    state: readonly(state),
    async bootstrap() {
      if (options.auth.state.value.status === 'bootstrapping') await options.auth.bootstrapSession()
      await reconcile()
    },
    async refresh() { await reconcile() },
    dispose() {
      stop()
      mediaQuery?.removeEventListener?.('change', onColorSchemeChange)
    },
  }
}

let applicationPreferences: UserPreferencesStore | undefined

export function initializeUserPreferences(options: UserPreferencesStoreOptions): UserPreferencesStore {
  applicationPreferences?.dispose()
  applicationPreferences = createUserPreferencesStore(options)
  return applicationPreferences
}

export function useUserPreferences(): UserPreferencesStore {
  if (!applicationPreferences) throw new Error('User preferences have not been initialized')
  return applicationPreferences
}
