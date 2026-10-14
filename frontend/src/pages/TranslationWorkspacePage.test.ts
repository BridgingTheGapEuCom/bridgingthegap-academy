import { cleanup, fireEvent, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { APIProblemError } from '../api/client'
import { pseudoLocalize } from '../i18n/pseudo'
import { renderWithI18n } from '../test/i18n'
import type { TranslationWorkspace } from '../translations/translations'

const getMock = vi.hoisted(() => vi.fn())
const patchMock = vi.hoisted(() => vi.fn())
const publishMock = vi.hoisted(() => vi.fn())
vi.mock('../translations/translations', async (original) => ({ ...(await original<typeof import('../translations/translations')>()), getTranslation: getMock, patchTranslation: patchMock, publishTranslation: publishMock }))

import TranslationWorkspacePage from './TranslationWorkspacePage.vue'

const translationId = '20000000-0000-4000-8000-000000000001'
const field = (source: string, translated: string | null = null) => ({ source, translated, state: translated === null ? 'UNTRANSLATED' as const : 'TRANSLATED' as const })
const fixture = (): TranslationWorkspace => ({
  translationId, revision: 3, lifecycle: 'DRAFT', targetLanguage: 'es',
  source: { courseId: '10000000-0000-4000-8000-000000000001', courseVersionId: '30000000-0000-4000-8000-000000000001', version: '1.0.0', language: 'en' },
  course: { title: field('Immutable source title'), description: field('Source description'), learningObjectives: [field('Source objective')] },
  modules: [{ sourceStableKey: 'module-one', position: 0, title: field('Long source module'), description: field('Module description'), completeness: { totalTranslatableFields: 5, translatedFields: 0, untranslatedFields: 5, complete: false }, lessons: [{ sourceStableKey: 'lesson-one', position: 0, prerequisiteStableKeys: [], title: field('Long source lesson'), description: field('Lesson description'), learningObjectives: [field('Lesson objective')], completeness: { totalTranslatableFields: 3, translatedFields: 0, untranslatedFields: 3, complete: false }, blocks: [{ sourceBlockKey: 'text-one', type: 'TEXT', position: 0, fields: { text: field('Authored source prose') }, assetKeys: [], code: '', tableHeaders: [], tableRows: [], assessmentKey: '' }] }] }],
  assessments: [], completeness: { totalTranslatableFields: 9, translatedFields: 0, untranslatedFields: 9, complete: false },
})

async function renderPage(locale: 'en' | 'en-XA' = 'en') {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/authoring/translations/:translationId', component: TranslationWorkspacePage },
    { path: '/authoring/courses/:courseId/versions/:version/translations', component: { template: '<p>List</p>' } },
  ] })
  await router.push(`/authoring/translations/${translationId}`)
  await router.isReady()
  return renderWithI18n({ template: '<RouterView />' }, { global: { plugins: [router] } }, locale)
}

describe('TranslationWorkspacePage', () => {
  beforeEach(() => {
    getMock.mockReset().mockImplementation(async () => fixture())
    patchMock.mockReset().mockResolvedValue({ translationId, targetLanguage: 'es', lifecycle: 'DRAFT', revision: 4, completeness: fixture().completeness })
    publishMock.mockReset()
  })
  afterEach(cleanup)

  it('distinguishes immutable source content from labelled editable target fields', async () => {
    await renderPage()
    expect(await screen.findByRole('heading', { level: 1, name: /Immutable source title.*Spanish/ })).toBeTruthy()
    expect(screen.getAllByText('Read only').length).toBeGreaterThan(0)
    expect(screen.getByText('Authored source prose')).toBeTruthy()
    expect(screen.getByRole('textbox', { name: 'Course title translation' })).toBeTruthy()
    expect(screen.getByText(/Academy interface language does not change/)).toBeTruthy()
  })

  it('keeps edits local, discards them, and saves all changes with the current revision', async () => {
    await renderPage()
    const input = await screen.findByRole('textbox', { name: 'Course title translation' })
    await fireEvent.update(input, 'Título traducido')
    expect(screen.getByLabelText('Unsaved translation changes')).toBeTruthy()
    expect(patchMock).not.toHaveBeenCalled()
    await fireEvent.click(screen.getByRole('button', { name: 'Discard changes' }))
    expect((input as HTMLInputElement).value).toBe('')

    await fireEvent.update(input, 'Título traducido')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(patchMock).toHaveBeenCalledWith(translationId, 3, [{ target: 'COURSE', field: 'title', translated: 'Título traducido' }]))
    expect(await screen.findByText('Translation saved.')).toBeTruthy()
  })

  it('preserves local edits and requires explicit reload after a CAS conflict', async () => {
    patchMock.mockRejectedValueOnce(new APIProblemError(409, undefined, undefined))
    await renderPage()
    const input = await screen.findByRole('textbox', { name: 'Course title translation' })
    await fireEvent.update(input, 'Local edit')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    expect((await screen.findByRole('alert')).textContent).toContain('changed elsewhere')
    expect((input as HTMLInputElement).value).toBe('Local edit')
    expect(screen.getByRole('button', { name: 'Reload latest translation' })).toBeTruthy()
  })

  it('publishes a complete workspace through the shared confirmation dialog', async () => {
    const complete = fixture()
    complete.completeness = { totalTranslatableFields: 9, translatedFields: 9, untranslatedFields: 0, complete: true }
    getMock.mockResolvedValue(complete)
    publishMock.mockResolvedValue({ publicationId: '40000000-0000-4000-8000-000000000001', translationId, revision: 3, targetLanguage: 'es', sourceCourseId: complete.source.courseId, sourceCourseVersionId: complete.source.courseVersionId, sourceVersion: complete.source.version, publishedAt: '2026-10-14T08:00:00Z' })
    await renderPage()
    await fireEvent.click(await screen.findByRole('button', { name: 'Publish translation' }))
    expect(screen.getByRole('dialog', { name: 'Publish translation?' })).toBeTruthy()
    await fireEvent.click(screen.getByRole('button', { name: 'Publish translation' }))
    await waitFor(() => expect(publishMock).toHaveBeenCalledWith(translationId, 3))
  })

  it('pseudo-localizes workspace chrome without changing authored source content', async () => {
    await renderPage('en-XA')
    expect(await screen.findByText(pseudoLocalize('Course metadata'))).toBeTruthy()
    expect(screen.getByText('Immutable source title')).toBeTruthy()
    expect(screen.getByText('Authored source prose')).toBeTruthy()
  })
})
