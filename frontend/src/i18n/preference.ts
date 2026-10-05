import { defaultApplicationLocale, isApplicationLocale, matchApplicationLocale, type ApplicationLocale } from './registry'

export const applicationLocaleStorageKey = 'btg.academy.application-locale'

export interface LocalePreferenceStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}

function browserStorage(): LocalePreferenceStorage | undefined {
  try {
    return typeof window === 'undefined' ? undefined : window.localStorage
  } catch {
    return undefined
  }
}

export function readLocalePreference(storage: LocalePreferenceStorage | undefined = browserStorage()): ApplicationLocale | undefined {
  if (!storage) return undefined
  try {
    const value = storage.getItem(applicationLocaleStorageKey)
    return isApplicationLocale(value) ? value : undefined
  } catch {
    return undefined
  }
}

export function saveLocalePreference(locale: ApplicationLocale, storage: LocalePreferenceStorage | undefined = browserStorage()): void {
  if (!storage) return
  try {
    storage.setItem(applicationLocaleStorageKey, locale)
  } catch {
    // Storage can be blocked or full. The active in-memory locale remains valid.
  }
}

export function clearLocalePreference(storage: LocalePreferenceStorage | undefined = browserStorage()): void {
  if (!storage) return
  try {
    storage.removeItem(applicationLocaleStorageKey)
  } catch {
    // A blocked storage implementation is equivalent to no persisted preference.
  }
}

export function resolveApplicationLocale(options: {
  storage?: LocalePreferenceStorage
  browserLanguages?: readonly string[]
} = {}): ApplicationLocale {
  const saved = readLocalePreference(options.storage)
  if (saved) return saved
  const browserLanguages = options.browserLanguages ?? (typeof navigator === 'undefined' ? [] : navigator.languages)
  for (const language of browserLanguages) {
    const matched = matchApplicationLocale(language)
    if (matched) return matched
  }
  return defaultApplicationLocale
}
