import { render } from '@testing-library/vue'
import type { Component } from 'vue'
import { createI18n } from 'vue-i18n'
import englishMessages from '../i18n/locales/en'
import pseudoMessages from '../i18n/locales/en-XA'
import type { ApplicationLocale } from '../i18n/registry'

export function createTestI18n(locale: ApplicationLocale = 'en') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en',
    messages: { en: englishMessages, 'en-XA': pseudoMessages },
    missingWarn: false,
    fallbackWarn: false,
  })
}

export function renderWithI18n(component: Component, options: Parameters<typeof render>[1] = {}, locale: ApplicationLocale = 'en') {
  const existingPlugins = options.global?.plugins ?? []
  return render(component, {
    ...options,
    global: {
      ...options.global,
      plugins: [...existingPlugins, createTestI18n(locale)],
    },
  })
}
