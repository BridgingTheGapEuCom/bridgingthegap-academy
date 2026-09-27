<template>
  <fieldset class="objectives-editor" :aria-describedby="describedBy">
    <legend>{{ label }} <span aria-hidden="true">*</span><span class="sr-only"> required</span></legend>
    <p v-if="description" :id="descriptionID" class="objectives-editor__description">{{ description }}</p>
    <ol>
      <li v-for="(_, index) in modelValue" :key="index">
        <span class="objectives-editor__number" aria-hidden="true">{{ index + 1 }}</span>
        <BtgTextInput
          :model-value="modelValue[index]"
          :disabled="disabled"
          :aria-label="`${label.slice(0, -1)} ${index + 1}`"
          :aria-invalid="Boolean(error) || undefined"
          placeholder="Enter a learning objective"
          @update:model-value="update(index, $event)"
        />
        <BtgButton type="button" variant="secondary" class="objectives-editor__remove" :disabled="disabled" :aria-label="`Remove objective ${index + 1}`" @click="remove(index)">Remove</BtgButton>
      </li>
    </ol>
    <p v-if="error" :id="errorID" class="objectives-editor__error">{{ error }}</p>
    <BtgButton type="button" variant="secondary" class="objectives-editor__add" :disabled="disabled" @click="add">+ Add objective</BtgButton>
    <p class="sr-only" aria-live="polite">{{ announcement }}</p>
  </fieldset>
</template>

<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import BtgButton from './BtgButton.vue'
import BtgTextInput from './BtgTextInput.vue'

const props = withDefaults(defineProps<{
  modelValue: string[]
  label?: string
  description?: string
  error?: string
  disabled?: boolean
}>(), { label: 'Learning objectives', description: undefined, error: undefined, disabled: false })
const emit = defineEmits<{ 'update:modelValue': [value: string[]] }>()
const announcement = ref('')
const fieldID = `objectives-${useId()}`
const descriptionID = `${fieldID}-description`
const errorID = `${fieldID}-error`
const describedBy = computed(() => [props.description ? descriptionID : '', props.error ? errorID : ''].filter(Boolean).join(' ') || undefined)

function update(index: number, value: string) {
  const next = [...props.modelValue]
  next[index] = value
  emit('update:modelValue', next)
}
function add() {
  emit('update:modelValue', [...props.modelValue, ''])
  announcement.value = `Objective ${props.modelValue.length + 1} added.`
}
function remove(index: number) {
  emit('update:modelValue', props.modelValue.filter((_, currentIndex) => currentIndex !== index))
  announcement.value = 'Objective removed.'
}
</script>

<style scoped>
.objectives-editor { display: grid; min-width: 0; gap: var(--btg-space-2); margin: 0; border: 0; padding: 0; }
.objectives-editor legend { padding: 0; font-weight: var(--btg-font-weight-strong); }
.objectives-editor__description { margin: 0; color: var(--btg-color-text-secondary); font-size: var(--btg-font-size-small); }
.objectives-editor ol { display: grid; gap: var(--btg-space-2); margin: 0; padding: 0; list-style: none; }
.objectives-editor li { display: grid; grid-template-columns: 2rem minmax(0, 1fr) auto; align-items: center; gap: var(--btg-space-2); max-width: none; }
.objectives-editor__number { display: grid; width: 2rem; min-height: 2rem; place-items: center; border-radius: var(--btg-radius-control); background: var(--btg-color-surface-muted); color: var(--btg-color-text-secondary); font-weight: var(--btg-font-weight-strong); }
.objectives-editor__remove { min-height: 2.75rem; padding-inline: var(--btg-space-3); }
.objectives-editor__add { width: fit-content; margin-top: var(--btg-space-1); }
.objectives-editor__error { margin: 0; color: var(--btg-color-danger); font-weight: var(--btg-font-weight-medium); }
@media (max-width: 30rem) {
  .objectives-editor li { grid-template-columns: 2rem minmax(0, 1fr); }
  .objectives-editor__remove { grid-column: 2; width: fit-content; }
}
</style>
