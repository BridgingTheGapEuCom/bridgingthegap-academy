import { describe, expect, it } from 'vitest'
import { configurationErrors, configurationInitial } from './configuration'

const schema = { fields: [
  { key: 'title', type: 'TEXT' as const, label: 'Title', required: true, default: 'Overview', minLength: 2, maxLength: 10 },
  { key: 'count', type: 'INTEGER' as const, label: 'Count', required: true, min: 1, max: 5 },
  { key: 'ratio', type: 'NUMBER' as const, label: 'Ratio', required: false, min: 0 },
  { key: 'visible', type: 'BOOLEAN' as const, label: 'Visible', required: false },
  { key: 'scope', type: 'SINGLE_SELECT' as const, label: 'Scope', required: true, options: [{ value: 'all', label: 'All' }] },
] }

describe('widget configuration helpers', () => {
  it('uses declared defaults without inventing required values', () => {
    expect(configurationInitial(schema)).toEqual({ title: 'Overview', visible: false })
    expect(configurationInitial(schema, { title: 'Current' })).toEqual({ title: 'Current', visible: false })
  })

  it('validates types, required values, bounds, options, and unknown keys', () => {
    expect(configurationErrors(schema, { title: 2, count: 1.5, ratio: -1, visible: 'yes', scope: 'active', extra: true })).toEqual({
      extra: 'This setting is not supported.',
      title: 'Enter text.',
      count: 'Enter a whole number.',
      ratio: 'Enter 0 or more.',
      visible: 'Choose whether this setting is enabled.',
      scope: 'Choose an available option.',
    })
    expect(configurationErrors(schema, { title: 'Ready', count: 3, visible: true, scope: 'all' })).toEqual({})
    expect(configurationErrors(schema, {})).toEqual({ title: 'This field is required.', count: 'This field is required.', scope: 'This field is required.' })
  })
})
