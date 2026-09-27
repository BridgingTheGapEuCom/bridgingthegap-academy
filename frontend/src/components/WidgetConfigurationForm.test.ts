import { cleanup, fireEvent, render, screen } from '@testing-library/vue'
import { afterEach, describe, expect, it } from 'vitest'
import WidgetConfigurationForm from './WidgetConfigurationForm.vue'

const schema = { fields: [
  { key: 'title', type: 'TEXT' as const, label: 'Title', description: 'A short title.', required: true, minLength: 2, maxLength: 20 },
  { key: 'notes', type: 'TEXTAREA' as const, label: 'Notes', required: false },
  { key: 'count', type: 'INTEGER' as const, label: 'Number of items', required: false, min: 1, max: 10 },
  { key: 'ratio', type: 'NUMBER' as const, label: 'Ratio', required: false },
  { key: 'completed', type: 'BOOLEAN' as const, label: 'Show completed', required: false },
  { key: 'scope', type: 'SINGLE_SELECT' as const, label: 'Scope', required: true, options: [{ value: 'active', label: 'Active courses' }, { value: 'all', label: 'All courses' }] },
] }

describe('WidgetConfigurationForm', () => {
  afterEach(cleanup)

  it('renders the bounded declarative field vocabulary with labels and help', () => {
    render(WidgetConfigurationForm, { props: { schema, modelValue: { title: 'Overview', notes: 'Hello', count: 5, ratio: 1.5, completed: true, scope: 'active' }, errors: { title: 'Review this value.' } } })
    expect((screen.getByRole('textbox', { name: /Title/ }) as HTMLInputElement).value).toBe('Overview')
    expect((screen.getByRole('textbox', { name: 'Notes' }) as HTMLTextAreaElement).value).toBe('Hello')
    expect((screen.getByRole('spinbutton', { name: 'Number of items' }) as HTMLInputElement).value).toBe('5')
    expect((screen.getByRole('spinbutton', { name: 'Ratio' }) as HTMLInputElement).value).toBe('1.5')
    expect((screen.getByRole('checkbox', { name: 'Show completed' }) as HTMLInputElement).checked).toBe(true)
    expect((screen.getByRole('combobox', { name: /Scope/ }) as HTMLSelectElement).value).toBe('active')
    expect(screen.getByText('A short title.')).toBeTruthy()
    expect(screen.getByText('Review this value.')).toBeTruthy()
  })

  it('emits structured data from native controls', async () => {
    const view = render(WidgetConfigurationForm, { props: { schema, modelValue: {} } })
    await fireEvent.update(screen.getByRole('textbox', { name: /Title/ }), 'Recent activity')
    expect((view.emitted('update:modelValue') as unknown[][]).at(-1)?.[0]).toEqual({ title: 'Recent activity' })
    await view.rerender({ schema, modelValue: { completed: false } })
    await fireEvent.click(screen.getByRole('checkbox', { name: 'Show completed' }))
    expect((view.emitted('update:modelValue') as unknown[][]).at(-1)?.[0]).toEqual({ completed: true })
  })
})
