import { cleanup, fireEvent, render, screen } from '@testing-library/vue'
import { afterEach, describe, expect, it } from 'vitest'
import LanguagePicker from './LanguagePicker.vue'

describe('LanguagePicker', () => {
  afterEach(cleanup)
  it('uses one searchable combobox and emits the canonical selected tag', async () => {
    const { emitted } = render(LanguagePicker, { props: { id: 'language', modelValue: '' } })
    const picker = screen.getByRole('combobox')
    expect((picker as HTMLInputElement).readOnly).toBe(true)
    await fireEvent.focus(picker)
    await fireEvent.update(picker, 'spanish')
    expect(screen.getByRole('option', { name: /Spanish \(es\)/ })).toBeTruthy()
    await fireEvent.click(screen.getByRole('option', { name: /Spanish \(es\)/ }))
    expect(emitted()['update:modelValue']).toEqual([['es']])
    expect((picker as HTMLInputElement).readOnly).toBe(true)
  })

  it('supports keyboard selection and an in-flow valid language tag option', async () => {
    const { emitted } = render(LanguagePicker, { props: { id: 'language', modelValue: '' } })
    const picker = screen.getByRole('combobox')
    await fireEvent.focus(picker)
    await fireEvent.update(picker, 'sw-ke')
    await fireEvent.keyDown(picker, { key: 'ArrowDown' })
    await fireEvent.keyDown(picker, { key: 'Enter' })
    expect(emitted()['update:modelValue']).toEqual([['sw-KE']])
    expect(screen.queryByText('Use another language tag')).toBeNull()
  })

  it('closes when focus or a pointer moves outside the combobox', async () => {
    render(LanguagePicker, { props: { id: 'language', modelValue: '' } })
    const picker = screen.getByRole('combobox')
    await fireEvent.focus(picker)
    expect(picker.getAttribute('aria-expanded')).toBe('true')

    await fireEvent.focusOut(picker, { relatedTarget: document.body })
    expect(picker.getAttribute('aria-expanded')).toBe('false')

    await fireEvent.focus(picker)
    await fireEvent.pointerDown(document.body)
    expect(picker.getAttribute('aria-expanded')).toBe('false')
  })
})
