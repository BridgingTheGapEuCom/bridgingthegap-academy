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
      <BtgTextarea v-if="field.type === 'TEXTAREA'" :id="controlId" :model-value="textValue(field)" :aria-describedby="describedBy" :invalid="invalid" :required="field.required" :minlength="field.minLength" :maxlength="field.maxLength" @update:model-value="update(field.key, $event)" />
      <BtgSelect v-else-if="field.type === 'SINGLE_SELECT'" :id="controlId" :model-value="textValue(field)" :aria-describedby="describedBy" :invalid="invalid" :required="field.required" @update:model-value="update(field.key, $event)">
        <option value="" :disabled="field.required">{{ copy.selectOption }}</option>
        <option v-for="option in field.options" :key="option.value" :value="option.value">{{ option.label }}</option>
      </BtgSelect>
      <BtgCheckbox v-else-if="field.type === 'BOOLEAN'" :id="controlId" :model-value="booleanValue(field)" :aria-describedby="describedBy" :label="booleanValue(field) ? copy.enabled : copy.disabled" @update:model-value="update(field.key, $event)" />
      <BtgTextInput v-else-if="field.type === 'INTEGER' || field.type === 'NUMBER'" :id="controlId" type="number" :model-value="String(numberValue(field))" :step="field.type === 'INTEGER' ? 1 : 'any'" :min="field.min" :max="field.max" :aria-describedby="describedBy" :invalid="invalid" :required="field.required" @update:model-value="numberValueUpdate(field, $event)" />
      <BtgTextInput v-else :id="controlId" :model-value="textValue(field)" :aria-describedby="describedBy" :invalid="invalid" :required="field.required" :minlength="field.minLength" :maxlength="field.maxLength" @update:model-value="update(field.key, $event)" />
    </BtgFormField>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { WidgetConfiguration, WidgetConfigurationField, WidgetConfigurationSchema } from '../plugins/configuration'
import BtgCheckbox from './BtgCheckbox.vue'
import BtgFormField from './BtgFormField.vue'
import BtgSelect from './BtgSelect.vue'
import BtgTextInput from './BtgTextInput.vue'
import BtgTextarea from './BtgTextarea.vue'

export interface WidgetConfigurationFormCopy {
  selectOption: string
  enabled: string
  disabled: string
}

const props = withDefaults(defineProps<{ schema: WidgetConfigurationSchema; modelValue: WidgetConfiguration; copy: WidgetConfigurationFormCopy; errors?: Record<string, string> }>(), { errors: () => ({}) })
const emit = defineEmits<{ 'update:modelValue': [value: WidgetConfiguration] }>()
const copy = computed(() => props.copy)
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
function numberValueUpdate(field: WidgetConfigurationField, value: string) {
  const next = { ...props.modelValue }
  if (value === '') delete next[field.key]
  else next[field.key] = Number(value)
  emit('update:modelValue', next)
}
</script>

<style scoped>
.widget-configuration-form { display: grid; gap: var(--btg-space-4); }
</style>
