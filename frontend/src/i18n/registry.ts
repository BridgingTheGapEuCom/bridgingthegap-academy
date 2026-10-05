import englishMessages from './locales/en'

export const defaultApplicationLocale = 'en' as const
export const fallbackApplicationLocale = 'en' as const

export const applicationLocales = {
  en: {
    id: 'en',
    nativeName: 'English',
    direction: 'ltr',
    production: true,
    load: async () => englishMessages,
  },
  'en-XA': {
    id: 'en-XA',
    nativeName: 'Pseudo-English',
    direction: 'ltr',
    production: false,
    load: async () => (await import('./locales/en-XA')).default,
  },
} as const

export type ApplicationLocale = keyof typeof applicationLocales

export function isApplicationLocale(value: unknown): value is ApplicationLocale {
  return typeof value === 'string' && Object.hasOwn(applicationLocales, value)
}

export function matchApplicationLocale(value: string | null | undefined, includeTestLocales = false): ApplicationLocale | undefined {
  if (!value) return undefined
  let canonical: string
  try {
    canonical = Intl.getCanonicalLocales(value)[0] ?? ''
  } catch {
    return undefined
  }
  if (isApplicationLocale(canonical) && (includeTestLocales || applicationLocales[canonical].production)) return canonical
  const base = canonical.split('-')[0]
  return Object.values(applicationLocales).find((locale) => locale.production && locale.id === base)?.id
}
