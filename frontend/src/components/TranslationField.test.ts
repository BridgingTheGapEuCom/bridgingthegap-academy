import { cleanup, fireEvent, screen } from '@testing-library/vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { renderWithI18n } from '../test/i18n'
import TranslationField, { type TranslationFieldCopy } from './TranslationField.vue'

const copy: TranslationFieldCopy = { sourceRegion: 'Read-only source', targetRegion: 'Editable target', source: 'Original', translation: 'Translated value', readOnly: 'Locked', untranslated: 'Missing', translated: 'Complete', markUntranslated: 'Clear translation', inputLabel: 'Translated title' }

describe('TranslationField', () => {
  afterEach(cleanup)

  it('uses caller copy and associates a visible target label with the shared control', async () => {
    const change = vi.fn()
    renderWithI18n(TranslationField, { props: { label: 'Course title', field: { source: 'Source title', translated: null, state: 'UNTRANSLATED' }, copy, 'onChange': change } })
    expect(screen.getByText('Source title')).toBeTruthy()
    expect(screen.getByText('Locked')).toBeTruthy()
    const input = screen.getByRole('textbox', { name: 'Translated title' })
    await fireEvent.update(input, 'Target title')
    expect(change).toHaveBeenCalledWith('Target title')
  })

  it('emits null only through the explicit untranslated action', async () => {
    const change = vi.fn()
    renderWithI18n(TranslationField, { props: { label: 'Course title', field: { source: 'Source title', translated: '', state: 'TRANSLATED' }, copy, 'onChange': change } })
    await fireEvent.click(screen.getByRole('button', { name: 'Clear translation' }))
    expect(change).toHaveBeenLastCalledWith(null)
  })
})
