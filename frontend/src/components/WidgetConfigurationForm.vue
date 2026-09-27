<template>
  <div class="widget-configuration-form">
    <BtgFormField
      v-for="field in schema.fields"
      :key="field.key"
      :label="field.label"
      :description="field.description"
      :required="field.required"
      :error="errors[field.key]"
      v-slot="{ controlId, describedBy, invalid }"
    >
      <textarea v-if="field.type === 'TEXTAREA'" :id="controlId" :value="textValue(field)" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" :required="field.required" :minlength="field.minLength" :maxlength="field.maxLength" @input="text(field, $event)" />
      <select v-else-if="field.type === 'SINGLE_SELECT'" :id="controlId" :value="textValue(field)" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" :required="field.required" @change="text(field, $event)">
        <option value="" :disabled="field.required">Choose an option</option>
        <option v-for="option in field.options" :key="option.value" :value="option.value">{{ option.label }}</option>
      </select>
      <div v-else-if="field.type === 'BOOLEAN'" class="widget-configuration-form__checkbox">
        <input :id="controlId" type="checkbox" :checked="booleanValue(field)" :aria-describedby="describedBy" @change="boolean(field, $event)" />
        <span>{{ booleanValue(field) ? 'Enabled' : 'Disabled' }}</span>
      </div>
      <input v-else-if="field.type === 'INTEGER' || field.type === 'NUMBER'" :id="controlId" type="number" :value="numberValue(field)" :step="field.type === 'INTEGER' ? 1 : 'any'" :min="field.min" :max="field.max" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" :required="field.required" @input="number(field, $event)" />
      <input v-else :id="controlId" type="text" :value="textValue(field)" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" :required="field.required" :minlength="field.minLength" :maxlength="field.maxLength" @input="text(field, $event)" />
    </BtgFormField>
  </div>
</template>

<script setup lang="ts">
import type { WidgetConfiguration, WidgetConfigurationField, WidgetConfigurationSchema } from '../plugins/configuration'
import BtgFormField from './BtgFormField.vue'

const props = withDefaults(defineProps<{ schema: WidgetConfigurationSchema; modelValue: WidgetConfiguration; errors?: Record<string, string> }>(), { errors: () => ({}) })
const emit = defineEmits<{ 'update:modelValue': [value: WidgetConfiguration] }>()
function textValue(field: WidgetConfigurationField): string { const value = props.modelValue[field.key]; return typeof value === 'string' ? value : '' }
function numberValue(field: WidgetConfigurationField): number | string { const value = props.modelValue[field.key]; return typeof value === 'number' ? value : '' }
function booleanValue(field: WidgetConfigurationField): boolean { return props.modelValue[field.key] === true }
function update(key: string, value: unknown) { emit('update:modelValue', { ...props.modelValue, [key]: value }) }
function text(field: WidgetConfigurationField, event: Event) {
  const value = (event.target as HTMLInputElement).value
  if (value === '' && !field.required) {
    const next = { ...props.modelValue }
    delete next[field.key]
    emit('update:modelValue', next)
    return
  }
  update(field.key, value)
}
function boolean(field: WidgetConfigurationField, event: Event) { update(field.key, (event.target as HTMLInputElement).checked) }
function number(field: WidgetConfigurationField, event: Event) {
  const value = (event.target as HTMLInputElement).value
  const next = { ...props.modelValue }
  if (value === '') delete next[field.key]
  else next[field.key] = Number(value)
  emit('update:modelValue', next)
}
</script>

<style scoped>
.widget-configuration-form { display: grid; gap: var(--btg-space-4); }
.widget-configuration-form textarea { min-height: 7rem; resize: vertical; }
.widget-configuration-form__checkbox { display: flex; align-items: center; gap: var(--btg-space-3); font-weight: var(--btg-font-weight-medium); }
.widget-configuration-form__checkbox input { width: 1.25rem; min-height: 1.25rem; }
</style>
