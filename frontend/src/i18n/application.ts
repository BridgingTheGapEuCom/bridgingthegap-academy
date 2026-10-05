import { computed } from 'vue'
import { createI18n, useI18n } from 'vue-i18n'
import englishMessages from './locales/en'
import { resolveApplicationLocale, saveLocalePreference } from './preference'
import { applicationLocales, fallbackApplicationLocale, type ApplicationLocale } from './registry'

function createBaseI18n(locale: ApplicationLocale) {
  const messages: Record<string, typeof englishMessages> = { en: englishMessages }
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: fallbackApplicationLocale,
    messages,
    missingWarn: import.meta.env.DEV,
    fallbackWarn: import.meta.env.DEV,
  })
}

export type AcademyI18n = ReturnType<typeof createBaseI18n>

export async function createApplicationI18n(locale = resolveApplicationLocale()): Promise<AcademyI18n> {
  const i18n = createBaseI18n(locale)
  if (locale !== fallbackApplicationLocale) i18n.global.setLocaleMessage(locale, await applicationLocales[locale].load())
  updateDocumentLocale(locale)
  return i18n
}

export async function setApplicationLocale(i18n: AcademyI18n, locale: ApplicationLocale, persist = true): Promise<void> {
  if (!i18n.global.availableLocales.includes(locale)) i18n.global.setLocaleMessage(locale, await applicationLocales[locale].load())
  i18n.global.locale.value = locale
  if (persist) saveLocalePreference(locale)
  updateDocumentLocale(locale)
}

export function updateDocumentLocale(locale: ApplicationLocale): void {
  if (typeof document === 'undefined') return
  document.documentElement.lang = locale
  document.documentElement.dir = applicationLocales[locale].direction
}

export function useApplicationLocale() {
  const composer = useI18n()
  const locale = computed(() => composer.locale.value as ApplicationLocale)
  async function setLocale(next: ApplicationLocale) {
    if (!composer.availableLocales.includes(next)) composer.setLocaleMessage(next, await applicationLocales[next].load())
    composer.locale.value = next
    saveLocalePreference(next)
    updateDocumentLocale(next)
  }
  return { locale, locales: applicationLocales, setLocale }
}
