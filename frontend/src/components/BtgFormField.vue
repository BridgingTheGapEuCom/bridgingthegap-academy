<template>
  <div class="btg-form-field">
    <label :for="controlId" class="btg-form-field__label">
      {{ label }}
      <span v-if="required" class="btg-form-field__required" aria-hidden="true">*</span>
      <span v-if="required" class="sr-only"> required</span>
    </label>
    <slot :control-id="controlId" :described-by="describedBy" :invalid="Boolean(error)" />
    <p v-if="supportingText" :id="descriptionId" class="btg-form-field__description">
      {{ supportingText }}
    </p>
    <p v-if="error" :id="errorId" class="btg-form-field__error">
      {{ error }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue'

const props = withDefaults(
  defineProps<{
    label: string
    controlId?: string
    description?: string
    helper?: string
    error?: string
    required?: boolean
  }>(),
  { controlId: undefined, description: undefined, helper: undefined, error: undefined, required: false },
)

const generatedID = `btg-field-${useId()}`
const controlId = computed(() => props.controlId || generatedID)
const descriptionId = computed(() => `${controlId.value}-description`)
const errorId = computed(() => `${controlId.value}-error`)
const supportingText = computed(() => props.description ?? props.helper)
const describedBy = computed(() => [supportingText.value ? descriptionId.value : '', props.error ? errorId.value : ''].filter(Boolean).join(' ') || undefined)
</script>
