import { cleanup, fireEvent, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { pseudoLocalize } from '../i18n/pseudo'
import { renderWithI18n } from '../test/i18n'

const listMock = vi.hoisted(() => vi.fn())
const createMock = vi.hoisted(() => vi.fn())
const sourceMock = vi.hoisted(() => vi.fn())
vi.mock('../translations/translations', async (original) => ({ ...(await original<typeof import('../translations/translations')>()), listTranslations: listMock, createTranslation: createMock }))
vi.mock('../courses/courses', async (original) => ({ ...(await original<typeof import('../courses/courses')>()), getPublishedCourseVersionByID: sourceMock }))

import TranslationListPage from './TranslationListPage.vue'

const courseId = '10000000-0000-4000-8000-000000000001'
const source = { courseId, version: '1.0.0', title: 'Source course title', description: 'Source', objectives: [], sourceLanguage: 'en', changelog: '', license: { kind: 'STANDARD', displayName: 'License' }, contributors: [], publishedAt: '2026-10-01T00:00:00Z', modules: [], assessments: [] }
const summary = { translationId: '20000000-0000-4000-8000-000000000001', targetLanguage: 'es', lifecycle: 'DRAFT' as const, revision: 2, completeness: { totalTranslatableFields: 4, translatedFields: 1, untranslatedFields: 3, complete: false } }

async function renderPage(locale: 'en' | 'en-XA' = 'en') {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/authoring/courses/:courseId/versions/:version/translations', component: TranslationListPage },
    { path: '/authoring/translations/:translationId', component: { template: '<p>Workspace</p>' } },
    { path: '/authoring', component: { template: '<p>Authoring</p>' } },
  ] })
  await router.push(`/authoring/courses/${courseId}/versions/1.0.0/translations`)
  await router.isReady()
  return { router, ...renderWithI18n({ template: '<RouterView />' }, { global: { plugins: [router] } }, locale) }
}

describe('TranslationListPage', () => {
  beforeEach(() => {
    listMock.mockReset().mockResolvedValue({ translations: [summary] })
    createMock.mockReset().mockResolvedValue(summary)
    sourceMock.mockReset().mockResolvedValue(source)
  })
  afterEach(cleanup)

  it('renders exact source context and localized translation status', async () => {
    await renderPage()
    expect(await screen.findByText('Source course title')).toBeTruthy()
    expect(screen.getByRole('heading', { level: 1, name: 'Translations' })).toBeTruthy()
    expect(screen.getByText(/English \(en\)/)).toBeTruthy()
    expect(screen.getByText('Draft')).toBeTruthy()
    expect(screen.getByText('1 of 4 field translated')).toBeTruthy()
    expect(screen.getByRole('link', { name: /Open Spanish.*translation/ })).toBeTruthy()
  })

  it('prevents source and duplicate target choices before creation', async () => {
    await renderPage()
    const picker = await screen.findByRole('combobox', { name: 'Target language required' })
    await fireEvent.focus(picker)
    await fireEvent.update(picker, 'en')
    await fireEvent.keyDown(picker, { key: 'Enter' })
    await fireEvent.click(screen.getByRole('button', { name: 'Create translation' }))
    expect(await screen.findByText('The target language must be different from the source language.')).toBeTruthy()
    expect(createMock).not.toHaveBeenCalled()
  })

  it('creates a target translation and opens its reload-safe workspace route', async () => {
    const { router } = await renderPage()
    const picker = await screen.findByRole('combobox', { name: 'Target language required' })
    await fireEvent.focus(picker)
    await fireEvent.update(picker, 'fr')
    await fireEvent.keyDown(picker, { key: 'Enter' })
    await fireEvent.click(screen.getByRole('button', { name: 'Create translation' }))
    await waitFor(() => expect(createMock).toHaveBeenCalledWith(courseId, '1.0.0', 'fr'))
    await waitFor(() => expect(router.currentRoute.value.path).toBe(`/authoring/translations/${summary.translationId}`))
  })

  it('pseudo-localizes Academy copy without translating source content', async () => {
    await renderPage('en-XA')
    expect(await screen.findByText('Source course title')).toBeTruthy()
    expect(screen.getByRole('heading', { level: 1, name: pseudoLocalize('Translations') })).toBeTruthy()
  })
})
