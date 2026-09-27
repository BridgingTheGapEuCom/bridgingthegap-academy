import type { components } from '../api/generated'

export type WidgetConfigurationSchema = components['schemas']['WidgetConfigurationSchema']
export type WidgetConfigurationField = components['schemas']['WidgetConfigurationField']
export type WidgetConfiguration = Record<string, unknown>

export function configurationInitial(schema: WidgetConfigurationSchema | null, current: WidgetConfiguration = {}): WidgetConfiguration {
  const result: WidgetConfiguration = { ...current }
  for (const field of schema?.fields ?? []) {
    if (Object.hasOwn(result, field.key)) continue
    if (field.default !== undefined) result[field.key] = field.default
    else if (field.type === 'BOOLEAN') result[field.key] = false
  }
  return result
}

export function configurationErrors(schema: WidgetConfigurationSchema | null, configuration: WidgetConfiguration): Record<string, string> {
  const errors: Record<string, string> = {}
  const fields = new Map((schema?.fields ?? []).map((field) => [field.key, field]))
  for (const key of Object.keys(configuration)) if (!fields.has(key)) errors[key] = 'This setting is not supported.'
  for (const field of fields.values()) {
    const value = configuration[field.key]
    const missing = value === undefined || (typeof value === 'string' && value.length === 0)
    if (missing) {
      if (field.required) errors[field.key] = 'This field is required.'
      continue
    }
    if (field.type === 'TEXT' || field.type === 'TEXTAREA') {
      if (typeof value !== 'string') errors[field.key] = 'Enter text.'
      else if (field.minLength !== undefined && [...value].length < field.minLength) errors[field.key] = `Enter at least ${field.minLength} characters.`
      else if (field.maxLength !== undefined && [...value].length > field.maxLength) errors[field.key] = `Enter no more than ${field.maxLength} characters.`
    } else if (field.type === 'INTEGER' || field.type === 'NUMBER') {
      if (typeof value !== 'number' || !Number.isFinite(value) || (field.type === 'INTEGER' && !Number.isInteger(value))) errors[field.key] = field.type === 'INTEGER' ? 'Enter a whole number.' : 'Enter a number.'
      else if (field.min !== undefined && value < field.min) errors[field.key] = `Enter ${field.min} or more.`
      else if (field.max !== undefined && value > field.max) errors[field.key] = `Enter ${field.max} or less.`
    } else if (field.type === 'BOOLEAN' && typeof value !== 'boolean') errors[field.key] = 'Choose whether this setting is enabled.'
    else if (field.type === 'SINGLE_SELECT' && (typeof value !== 'string' || !field.options?.some((option) => option.value === value))) errors[field.key] = 'Choose an available option.'
    else if (!['TEXT', 'TEXTAREA', 'INTEGER', 'NUMBER', 'BOOLEAN', 'SINGLE_SELECT'].includes(field.type)) errors[field.key] = 'This field type is not supported.'
  }
  return errors
}
