import { afterEach, describe, expect, it } from 'vitest'
import { createApplicationI18n, setApplicationLocale } from './application'
import { applicationErrorKey, validationMessageKey } from './errors'
import { formatApplicationDate, formatApplicationDuration, formatApplicationNumber } from './formatters'
import { applicationLocaleStorageKey, readLocalePreference, resolveApplicationLocale, saveLocalePreference, type LocalePreferenceStorage } from './preference'
import { pseudoLocalize } from './pseudo'
import { matchApplicationLocale } from './registry'

class MemoryStorage implements LocalePreferenceStorage {
  values = new Map<string, string>()
  getItem(key: string) { return this.values.get(key) ?? null }
  setItem(key: string, value: string) { this.values.set(key, value) }
  removeItem(key: string) { this.values.delete(key) }
}

afterEach(() => {
  document.documentElement.lang = 'en'
  document.documentElement.dir = ''
})

describe('Academy application internationalization', () => {
  it('resolves saved preference before browser language and validates unsupported values', () => {
    const storage = new MemoryStorage()
    expect(resolveApplicationLocale({ storage, browserLanguages: ['es-ES', 'en-GB'] })).toBe('en')
    storage.setItem(applicationLocaleStorageKey, 'en-XA')
    expect(resolveApplicationLocale({ storage, browserLanguages: ['en'] })).toBe('en-XA')
    storage.setItem(applicationLocaleStorageKey, 'invalid')
    expect(readLocalePreference(storage)).toBeUndefined()
  })

  it('persists only canonical registered locales through the preference boundary', () => {
    const storage = new MemoryStorage()
    saveLocalePreference('en-XA', storage)
    expect(readLocalePreference(storage)).toBe('en-XA')
    expect(matchApplicationLocale('EN-gb')).toBe('en')
    expect(matchApplicationLocale('es-ES')).toBeUndefined()
  })

  it('loads pseudo messages, preserves interpolation, and synchronizes document language', async () => {
    const i18n = await createApplicationI18n('en')
    await setApplicationLocale(i18n, 'en-XA', false)
    expect(document.documentElement.lang).toBe('en-XA')
    const position = i18n.global.t('authoring.structure.lessonPosition', { current: 2, total: 5 })
    expect(position).toContain('2')
    expect(position).toContain('5')
    expect(i18n.global.t('navigation.home')).toBe(pseudoLocalize('Home'))
  })

  it('formats dates, numbers, and durations through locale-aware boundaries', () => {
    expect(formatApplicationDate('2026-10-05T12:00:00Z', 'en-GB', 'date')).toContain('2026')
    expect(formatApplicationNumber(1234.5, 'en-GB', 'decimal')).toContain('1,234.5')
    expect(formatApplicationDuration(75, 'en-GB')).toBe('1 hr 15 mins')
  })

  it('maps known validation and API states without exposing arbitrary backend prose', () => {
    expect(validationMessageKey('required')).toBe('validation.required')
    expect(applicationErrorKey({ status: 409 })).toBe('errors.conflict')
    expect(applicationErrorKey({ problem: { code: 'known' } }, { known: 'errors.unavailable' })).toBe('errors.unavailable')
    expect(applicationErrorKey(new Error('sensitive backend detail'))).toBe('errors.generic')
  })

  it('uses framework pluralization instead of English-only string assembly', async () => {
    const i18n = await createApplicationI18n('en')
    expect(i18n.global.t('common.counts.lesson', 1)).toBe('1 lesson')
    expect(i18n.global.t('common.counts.lesson', 3)).toBe('3 lessons')
  })
})
