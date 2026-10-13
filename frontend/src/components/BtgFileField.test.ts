import { cleanup, fireEvent, screen } from '@testing-library/vue'
import { afterEach, describe, expect, it } from 'vitest'
import BtgFileField from './BtgFileField.vue'
import { renderWithI18n } from '../test/i18n'

describe('BtgFileField', () => {
  afterEach(cleanup)

  it('associates visible copy, exposes the filename, and can clear selection', async () => {
    const result = renderWithI18n(BtgFileField, { props: { label: 'Plugin package', description: 'ZIP files only.', emptyText: 'No file selected.', removeLabel: 'Remove package', required: true } })
    const input = screen.getByLabelText(/Plugin package/) as HTMLInputElement
    const selected = new File(['zip'], 'plugin.zip', { type: 'application/zip' })
    Object.defineProperty(input, 'files', { configurable: true, value: [selected] })
    await fireEvent.change(input)
    expect(screen.getByText('plugin.zip')).toBeTruthy()
    expect(result.emitted()['update:modelValue']?.[0]).toEqual([selected])
    await fireEvent.click(screen.getByRole('button', { name: 'Remove package' }))
    expect(screen.getByText('No file selected.')).toBeTruthy()
    expect(document.activeElement).toBe(input)
  })
})
